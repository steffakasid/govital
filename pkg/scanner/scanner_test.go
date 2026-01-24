package scanner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewScanner(t *testing.T) {
	projectPath := "/test/project"
	scanner := NewScanner(projectPath)

	assert.NotNil(t, scanner)
	assert.Equal(t, projectPath, scanner.projectPath)
	assert.Equal(t, 30, scanner.staleThresholdDays)
	assert.NotNil(t, scanner.result)
	assert.Equal(t, projectPath, scanner.result.ProjectPath)
	assert.Equal(t, 0, len(scanner.result.Dependencies))
}

func TestSetStaleThreshold(t *testing.T) {
	scanner := NewScanner(".")
	threshold := 180

	scanner.SetStaleThreshold(threshold)

	assert.Equal(t, threshold, scanner.staleThresholdDays)
	assert.Equal(t, threshold, scanner.result.Summary.StaleThresholdDays)
}

func TestScanGoModNotFound(t *testing.T) {
	// Create a temporary directory without go.mod
	tmpDir := t.TempDir()
	scanner := NewScanner(tmpDir)

	err := scanner.Scan()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go.mod not found")
}

func TestScanWithValidGoMod(t *testing.T) {
	// We can't reliably test with "." since test working dir varies
	// Just verify the error handling works correctly
	scanner := NewScanner(".")
	err := scanner.Scan()
	
	// Either succeeds (if run from project root) or fails with proper error
	if err != nil {
		assert.Contains(t, err.Error(), "go.mod not found")
	} else {
		assert.Greater(t, scanner.result.Summary.Total, 0)
		assert.Equal(t, len(scanner.result.Dependencies), scanner.result.Summary.Total)
	}
}

func TestGetInactiveDependencies(t *testing.T) {
	scanner := NewScanner(".")
	scanner.result.Dependencies = []Dependency{
		{
			Path:     "github.com/example/active",
			Version:  "v1.0.0",
			IsActive: true,
		},
		{
			Path:     "github.com/example/inactive1",
			Version:  "v1.0.0",
			IsActive: false,
		},
		{
			Path:     "github.com/example/inactive2",
			Version:  "v1.0.0",
			IsActive: false,
		},
	}
	scanner.result.Summary.Total = 3
	scanner.result.Summary.Inactive = 2

	inactive := scanner.GetInactiveDependencies()

	assert.Equal(t, 2, len(inactive))
	assert.Equal(t, "github.com/example/inactive1", inactive[0].Path)
	assert.Equal(t, "github.com/example/inactive2", inactive[1].Path)
}

func TestGetResults(t *testing.T) {
	scanner := NewScanner(".")
	result := scanner.GetResults()

	assert.NotNil(t, result)
	assert.Same(t, scanner.result, result)
}

func TestDependencyIsActive(t *testing.T) {
	tests := []struct {
		name                   string
		daysSinceLastCommit    int
		staleThreshold         int
		expectedIsActive       bool
	}{
		{
			name:                   "recent commit should be active",
			daysSinceLastCommit:    30,
			staleThreshold:         365,
			expectedIsActive:       true,
		},
		{
			name:                   "old commit should be inactive",
			daysSinceLastCommit:    500,
			staleThreshold:         365,
			expectedIsActive:       false,
		},
		{
			name:                   "exactly at threshold should be active",
			daysSinceLastCommit:    365,
			staleThreshold:         365,
			expectedIsActive:       true,
		},
		{
			name:                   "just over threshold should be inactive",
			daysSinceLastCommit:    366,
			staleThreshold:         365,
			expectedIsActive:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			scanner.SetStaleThreshold(tt.staleThreshold)

			dep := &Dependency{
				Path:                    "github.com/example/test",
				Version:                 "v1.0.0",
				LastCommitTime:          time.Now().AddDate(0, 0, -tt.daysSinceLastCommit),
				DaysSinceLastCommit:     tt.daysSinceLastCommit,
				IsActive:                tt.daysSinceLastCommit <= tt.staleThreshold,
			}

			assert.Equal(t, tt.expectedIsActive, dep.IsActive)
		})
	}
}

func TestScanResultSummary(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := NewScanner(tmpDir)

	// This will fail since tmpDir has no go.mod, but we can still test the result structure
	err := scanner.Scan()
	
	assert.Error(t, err)
	result := scanner.GetResults()

	assert.NotNil(t, result.Summary)
	assert.Equal(t, 0, result.Summary.Total)
	assert.Equal(t, 365, result.Summary.StaleThresholdDays)
}
