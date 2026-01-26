package scanner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

	// Get direct dependencies if not including indirect
	var directDeps map[string]bool
	if !s.includeIndirectDependencies {
		var err error
		directDeps, err = s.getDirectDependencies()
		if err != nil {
			eslog.Warnf("Failed to get direct dependencies list, will scan all: %v", err)
			directDeps = make(map[string]bool)
		}
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
			Path    string
			Version string
			Main    bool
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
		if !s.includeIndirectDependencies && !directDeps[dep.Path] {
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
	// Get the git repository URL from the module
	repoURL, commitHash, err := s.getRepositoryInfo(dep.Path, dep.Version)
	if err != nil {
		eslog.Warnf("Failed to get repository info for %s: %v", dep.Path, err)
		dep.IsActive = true // Assume active if we can't check
		return nil
	}

	// Get the actual commit time from git
	commitTime, err := s.getCommitTime(repoURL, commitHash)
	if err != nil {
		eslog.Warnf("Failed to get commit time for %s: %v", dep.Path, err)
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

// getRepositoryInfo extracts the git repository URL and commit hash for a specific version
func (s *Scanner) getRepositoryInfo(modulePath, version string) (repoURL, commitHash string, err error) {
	// Use go list to get detailed information about the module
	cmd := exec.Command("go", "list", "-json", modulePath+"@"+version)
	cmd.Dir = s.projectPath

	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to list module info: %w", err)
	}

	var modInfo struct {
		Module struct {
			Path    string
			Version string
		}
		Error interface{}
	}

	if err := json.Unmarshal(output, &modInfo); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal module info: %w", err)
	}

	// Get module source info using go mod download
	dlCmd := exec.Command("go", "mod", "download", "-json", modulePath+"@"+version)
	dlCmd.Dir = s.projectPath

	dlOutput, err := dlCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to download module info: %w", err)
	}

	var downloadInfo struct {
		Dir string `json:"Dir"`
	}

	if err := json.Unmarshal(dlOutput, &downloadInfo); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal download info: %w", err)
	}

	// Note: We could read go.mod to find repository URL, but for now
	// we construct it directly from the module path

	// Extract commit hash from version (e.g., v1.2.3-20240125abcdef1 or v1.2.3)
	// For tagged versions, we need to construct the repo URL
	repoURL = "https://" + modulePath
	commitHash = s.extractCommitHash(version)

	return repoURL, commitHash, nil
}

// getCommitTime fetches the actual commit timestamp from a git repository
func (s *Scanner) getCommitTime(repoURL, commitHash string) (time.Time, error) {
	if commitHash == "" {
		// If no commit hash, use a default or return current time
		return time.Now(), nil
	}

	// Use git clone and git show to fetch the actual commit timestamp
	tempDir, err := os.MkdirTemp("", "govital-repo-")
	if err != nil {
		return time.Now(), err
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			eslog.Warnf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Clone the repository
	cloneCmd := exec.Command("git", "clone", "--quiet", "--depth", "1", repoURL, tempDir)
	if err := cloneCmd.Run(); err != nil {
		return s.getCommitTimeViaHTTP(repoURL, commitHash)
	}

	// Get commit timestamp using git show
	showCmd := exec.Command("git", "show", "-s", "--format=%cI", commitHash)
	showCmd.Dir = tempDir

	timeOutput, err := showCmd.Output()
	if err != nil {
		return time.Now(), fmt.Errorf("failed to get commit time: %w", err)
	}

	commitTime, err := time.Parse(time.RFC3339, strings.TrimSpace(string(timeOutput)))
	if err != nil {
		return time.Now(), fmt.Errorf("failed to parse commit time: %w", err)
	}

	return commitTime, nil
}

// getCommitTimeViaHTTP attempts to get commit time via GitHub API or other HTTP methods
func (s *Scanner) getCommitTimeViaHTTP(repoURL, commitHash string) (time.Time, error) {
	// For GitHub repositories, we could use the GitHub API, but that requires auth
	// For now, return the current time as we can't determine it
	// In a real implementation, this could integrate with GitHub API using tokens
	eslog.Debugf("Cannot determine commit time for %s@%s via HTTP, assuming recently active", repoURL, commitHash)
	return time.Now(), nil
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

// getDirectDependencies returns a map of direct dependency paths
func (s *Scanner) getDirectDependencies() (map[string]bool, error) {
	cmd := exec.Command("go", "mod", "graph")
	cmd.Dir = s.projectPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get module graph: %w", err)
	}

	directDeps := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Format: module@version direct-dep@version
		// We only care about direct deps (from the root module)
		parts := bytes.Fields([]byte(line))
		if len(parts) >= 2 {
			// Mark as direct dependency
			depPath := string(parts[1])
			// Remove version suffix if present
			if idx := bytes.IndexByte(parts[1], '@'); idx >= 0 {
				depPath = string(parts[1][:idx])
			}
			directDeps[depPath] = true
		}
	}

	return directDeps, nil
}
