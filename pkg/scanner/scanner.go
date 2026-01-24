package scanner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/steffakasid/eslog"
)

type Dependency struct {
	Path              string
	Version           string
	Update            string
	Latest            string
	Error             string
	LastCommitTime    time.Time
	IsActive          bool
	DaysSinceLastCommit int
}

type ScanResult struct {
	ProjectPath  string
	Dependencies []Dependency
	Summary      struct {
		Total           int
		Updated         int
		Outdated        int
		Errors          int
		Inactive        int
		StaleThresholdDays int
	}
}

type Scanner struct {
	projectPath              string
	result                   *ScanResult
	staleThresholdDays       int
	includeIndirectDependencies bool
	workers                  int
	resultMutex              *sync.Mutex
}

func NewScanner(projectPath string) *Scanner {
	result := &ScanResult{
		ProjectPath:  projectPath,
		Dependencies: make([]Dependency, 0),
	}
	result.Summary.StaleThresholdDays = 30 // Set default threshold in result

	return &Scanner{
		projectPath:              projectPath,
		staleThresholdDays:       30,
		includeIndirectDependencies: false,
		workers:                  4,
		resultMutex:              &sync.Mutex{},
		result:                   result,
	}
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
		if !s.includeIndirectDependencies && len(directDeps) > 0 && !directDeps[dep.Path] {
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
	// Try to get the repository metadata using go mod download
	cmd := exec.Command("go", "mod", "download", "-json", dep.Path+"@"+dep.Version)
	cmd.Dir = s.projectPath

	output, err := cmd.Output()
	if err != nil {
		// Fallback: mark as potentially stale if we can't check
		return fmt.Errorf("failed to check %s: %w", dep.Path, err)
	}

	// Parse the go mod download output to get Info file path
	var modDownloadInfo struct {
		Info string `json:"Info"`
	}

	if err := json.Unmarshal(output, &modDownloadInfo); err != nil {
		return fmt.Errorf("failed to unmarshal module download info: %w", err)
	}

	// The Info field contains the path to the .info file with the actual metadata
	if modDownloadInfo.Info == "" {
		return fmt.Errorf("no info file path provided for %s", dep.Path)
	}

	// Read the .info file
	infoFileData, err := os.ReadFile(modDownloadInfo.Info)
	if err != nil {
		return fmt.Errorf("failed to read info file for %s: %w", dep.Path, err)
	}

	// Parse the .info file JSON
	var moduleInfo struct {
		Version string    `json:"Version"`
		Time    time.Time `json:"Time"`
	}

	if err := json.Unmarshal(infoFileData, &moduleInfo); err != nil {
		return fmt.Errorf("failed to unmarshal module info from %s: %w", modDownloadInfo.Info, err)
	}

	dep.LastCommitTime = moduleInfo.Time
	daysSinceCommit := int(time.Since(dep.LastCommitTime).Hours() / 24)
	dep.DaysSinceLastCommit = daysSinceCommit

	if daysSinceCommit > s.staleThresholdDays {
		dep.IsActive = false
	}

	return nil
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
