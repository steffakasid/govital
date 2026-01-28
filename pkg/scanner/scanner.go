package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/steffakasid/eslog"
)

type Dependency struct {
	Path                string
	Version             string
	Update              string
	Latest              string
	Error               string
	LastCommitTime      time.Time
	IsActive            bool
	DaysSinceLastCommit int
}

type ScanResult struct {
	ProjectPath  string
	Dependencies []Dependency
	Summary      struct {
		Total              int
		Updated            int
		Outdated           int
		Errors             int
		Inactive           int
		StaleThresholdDays int
	}
}

type Scanner struct {
	projectPath                 string
	result                      *ScanResult
	staleThresholdDays          int
	includeIndirectDependencies bool
	workers                     int
	resultMutex                 *sync.Mutex
}

func NewScanner(projectPath string) *Scanner {
	result := &ScanResult{
		ProjectPath:  projectPath,
		Dependencies: make([]Dependency, 0),
	}
	result.Summary.StaleThresholdDays = 30 // Set default threshold in result

	return &Scanner{
		projectPath:                 projectPath,
		staleThresholdDays:          30,
		includeIndirectDependencies: false,
		workers:                     4,
		resultMutex:                 &sync.Mutex{},
		result:                      result,
	}
}

// isStale returns true if a dependency is stale based on days since last commit
func (s *Scanner) isStale(daysSinceCommit int) bool {
	return daysSinceCommit > s.staleThresholdDays
}

// extractCommitHash extracts the commit hash from a pseudo-version string
func (s *Scanner) extractCommitHash(version string) string {
	if len(version) == 0 || version[0] != 'v' {
		return ""
	}

	parts := version[1:] // Remove 'v'
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == '-' {
			suffix := parts[i+1:]
			if len(suffix) >= 12 {
				return suffix[len(suffix)-12:] // Last 12 chars is the commit hash
			}
			break
		}
	}
	return ""
}

func (s *Scanner) SetWorkers(count int) {
	if count < 1 {
		count = 1
	}
	s.workers = count
}

func (s *Scanner) SetStaleThreshold(days int) {
	s.staleThresholdDays = days
	s.result.Summary.StaleThresholdDays = days
}

func (s *Scanner) SetIncludeIndirectDependencies(include bool) {
	s.includeIndirectDependencies = include
}

func (s *Scanner) Scan() error {
	// Check if go.mod exists
	goModPath := filepath.Join(s.projectPath, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		eslog.Errorf("go.mod not found at %s", goModPath)
		return fmt.Errorf("go.mod not found at %s", goModPath)
	}

	// Get all dependencies with go list
	cmd := exec.Command("go", "list", "-json", "-m", "all")
	cmd.Dir = s.projectPath

	output, err := cmd.Output()
	if err != nil {
		eslog.Errorf("Failed to list dependencies: %v", err)
		return fmt.Errorf("failed to list dependencies: %w", err)
	}

	// Collect dependencies to scan
	var depsToScan []Dependency
	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var dep struct {
			Path     string
			Version  string
			Main     bool
			Indirect bool
		}

		if err := decoder.Decode(&dep); err != nil {
			eslog.Errorf("Failed to decode dependency: %v", err)
			s.result.Summary.Errors++
			continue
		}

		if dep.Main {
			continue // Skip main module
		}

		// Skip indirect dependencies if not including them
		if !s.includeIndirectDependencies && dep.Indirect {
			continue
		}

		depsToScan = append(depsToScan, Dependency{
			Path:     dep.Path,
			Version:  dep.Version,
			IsActive: true,
		})
	}

	// Scan dependencies in parallel
	s.scanParallel(depsToScan)

	s.result.Summary.StaleThresholdDays = s.staleThresholdDays
	eslog.Infof("Dependencies found: %d (scanned with %d workers)", s.result.Summary.Total, s.workers)
	return nil
}

// scanParallel scans dependencies in parallel using worker goroutines
func (s *Scanner) scanParallel(depsToScan []Dependency) {
	var wg sync.WaitGroup
	depChan := make(chan *Dependency, len(depsToScan))

	// Start worker goroutines
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dep := range depChan {
				// Check maintenance status
				if err := s.checkMaintenanceStatus(dep); err != nil {
					eslog.Debugf("Failed to check maintenance status for %s: %v", dep.Path, err)
				}

				// Append result safely
				s.resultMutex.Lock()
				s.result.Dependencies = append(s.result.Dependencies, *dep)
				s.result.Summary.Total++
				if !dep.IsActive {
					s.result.Summary.Inactive++
				}
				s.resultMutex.Unlock()
			}
		}()
	}

	// Send dependencies to be scanned
	for i := range depsToScan {
		depChan <- &depsToScan[i]
	}
	close(depChan)

	// Wait for all workers to finish
	wg.Wait()
}

func (s *Scanner) checkMaintenanceStatus(dep *Dependency) error {
	// Get version info from Go proxy
	commitTime, err := s.getVersionInfoFromProxy(dep.Path, dep.Version)
	if err != nil {
		eslog.Warnf("Failed to get version info for %s@%s from proxy: %v", dep.Path, dep.Version, err)
		dep.IsActive = true // Assume active if we can't check
		return nil
	}

	dep.LastCommitTime = commitTime
	daysSinceCommit := int(time.Since(dep.LastCommitTime).Hours() / 24)
	dep.DaysSinceLastCommit = daysSinceCommit

	if s.isStale(daysSinceCommit) {
		dep.IsActive = false
	}

	return nil
}

// getGoProxyURLs returns a list of Go proxy URLs from the GOPROXY environment variable
// Falls back to proxy.golang.org if GOPROXY is not set
// Handles multiple proxies separated by commas
func (s *Scanner) getGoProxyURLs() []string {
	goproxy := os.Getenv("GOPROXY")
	if goproxy == "" {
		return []string{"https://proxy.golang.org"}
	}

	var proxies []string
	for _, p := range strings.Split(goproxy, ",") {
		p = strings.TrimSpace(p)
		if p != "" && p != "direct" {
			// Remove trailing slash for consistency
			p = strings.TrimSuffix(p, "/")
			proxies = append(proxies, p)
		}
	}

	// If no valid proxies found (e.g., only "direct" was specified),
	// fall back to the default proxy
	if len(proxies) == 0 {
		proxies = append(proxies, "https://proxy.golang.org")
	}

	return proxies
}

// versionInfo represents the JSON response from the Go proxy
type versionInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

// getVersionInfoFromProxy fetches version information from the Go proxy
// Tries each proxy in order and returns the first successful result
func (s *Scanner) getVersionInfoFromProxy(modulePath, version string) (time.Time, error) {
	proxies := s.getGoProxyURLs()
	var lastErr error

	// Try each proxy in order
	for i, proxyURL := range proxies {
		// Construct the proxy URL for the version info endpoint
		// Format: {GOPROXY}/{modulePath}/@v/{version}.info
		escapedPath := url.PathEscape(modulePath)
		infoURL := fmt.Sprintf("%s/%s/@v/%s.info", proxyURL, escapedPath, url.PathEscape(version))

		response, err := http.Get(infoURL)
		if err != nil {
			lastErr = fmt.Errorf("proxy %s: %w", proxyURL, err)
			eslog.Debugf("Failed to fetch from proxy %d/%d (%s): %v", i+1, len(proxies), proxyURL, err)
			continue
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			lastErr = fmt.Errorf("proxy %s returned status %d: %s", proxyURL, response.StatusCode, string(body))
			eslog.Debugf("Proxy %d/%d (%s) failed: %v", i+1, len(proxies), proxyURL, lastErr)
			continue
		}

		// Successfully got response, decode it
		var info versionInfo
		if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
			lastErr = fmt.Errorf("failed to decode version info from proxy %s: %w", proxyURL, err)
			eslog.Debugf("Failed to decode response from proxy %d/%d (%s): %v", i+1, len(proxies), proxyURL, err)
			continue
		}

		// Success!
		eslog.Debugf("Successfully fetched version info for %s@%s from proxy %d/%d (%s)", modulePath, version, i+1, len(proxies), proxyURL)
		return info.Time, nil
	}

	// All proxies failed
	if lastErr != nil {
		return time.Time{}, fmt.Errorf("failed to fetch version info from all %d proxies: %w", len(proxies), lastErr)
	}
	return time.Time{}, fmt.Errorf("no proxies available")
}

func (s *Scanner) PrintResults() {
	fmt.Printf("\n=== Govital Dependency Scan Results ===\n")
	fmt.Printf("Project: %s\n", s.projectPath)
	fmt.Printf("Stale Threshold: %d days\n\n", s.staleThresholdDays)

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Dependencies:        %d\n", s.result.Summary.Total)
	fmt.Printf("  Inactive Dependencies:     %d\n", s.result.Summary.Inactive)
	fmt.Printf("  Errors:                    %d\n", s.result.Summary.Errors)
	fmt.Printf("\nDependencies:\n")

	for _, dep := range s.result.Dependencies {
		status := "✓ Active"
		if !dep.IsActive {
			status = "✗ Inactive"
		}

		if dep.Error != "" {
			fmt.Printf("  - %s@%s [ERROR: %s]\n", dep.Path, dep.Version, dep.Error)
		} else if !dep.LastCommitTime.IsZero() {
			fmt.Printf("  - %s@%s [%s] (last commit: %d days ago)\n",
				dep.Path, dep.Version, status, dep.DaysSinceLastCommit)
		} else {
			fmt.Printf("  - %s@%s [%s]\n", dep.Path, dep.Version, status)
		}
	}
	fmt.Printf("\n")
}

func (s *Scanner) GetInactiveDependencies() []Dependency {
	var inactive []Dependency
	for _, dep := range s.result.Dependencies {
		if !dep.IsActive {
			inactive = append(inactive, dep)
		}
	}
	return inactive
}

func (s *Scanner) GetResults() *ScanResult {
	return s.result
}
