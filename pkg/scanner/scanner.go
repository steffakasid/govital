package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/steffakasid/eslog"
)

type Dependency struct {
	Path    string
	Version string
	Update  string
	Latest  string
	Error   string
}

type ScanResult struct {
	ProjectPath  string
	Dependencies []Dependency
	Summary      struct {
		Total      int
		Updated    int
		Outdated   int
		Errors     int
	}
}

type Scanner struct {
	projectPath string
	result      *ScanResult
}

func NewScanner(projectPath string) *Scanner {
	return &Scanner{
		projectPath: projectPath,
		result: &ScanResult{
			ProjectPath:  projectPath,
			Dependencies: make([]Dependency, 0),
		},
	}
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

	// Parse dependencies
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

		s.result.Dependencies = append(s.result.Dependencies, Dependency{
			Path:    dep.Path,
			Version: dep.Version,
		})
		s.result.Summary.Total++
	}

	eslog.Infof("Dependencies found: %d", s.result.Summary.Total)
	return nil
}

func (s *Scanner) PrintResults() {
	fmt.Printf("\n=== Govital Dependency Scan Results ===\n")
	fmt.Printf("Project: %s\n\n", s.projectPath)
	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Dependencies: %d\n", s.result.Summary.Total)
	fmt.Printf("  Updated:           %d\n", s.result.Summary.Updated)
	fmt.Printf("  Outdated:          %d\n", s.result.Summary.Outdated)
	fmt.Printf("  Errors:            %d\n", s.result.Summary.Errors)
	fmt.Printf("\nDependencies:\n")

	for _, dep := range s.result.Dependencies {
		if dep.Error != "" {
			fmt.Printf("  - %s@%s [ERROR: %s]\n", dep.Path, dep.Version, dep.Error)
		} else if dep.Update != "" {
			fmt.Printf("  - %s@%s (update available: %s)\n", dep.Path, dep.Version, dep.Update)
		} else {
			fmt.Printf("  - %s@%s\n", dep.Path, dep.Version)
		}
	}
	fmt.Printf("\n")
}
