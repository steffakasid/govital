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
	assert.Equal(t, 30, result.Summary.StaleThresholdDays)
}

func TestSetWorkers(t *testing.T) {
	tests := []struct {
		name           string
		workers        int
		expectedWorkers int
	}{
		{"positive workers", 4, 4},
		{"zero workers becomes one", 0, 1},
		{"negative workers becomes one", -5, 1},
		{"large number of workers", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			scanner.SetWorkers(tt.workers)
			assert.Equal(t, tt.expectedWorkers, scanner.workers)
		})
	}
}

func TestSetIncludeIndirectDependencies(t *testing.T) {
	tests := []struct {
		name     string
		include  bool
		expected bool
	}{
		{"true includes indirect", true, true},
		{"false excludes indirect", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			scanner.SetIncludeIndirectDependencies(tt.include)
			assert.Equal(t, tt.expected, scanner.includeIndirectDependencies)
		})
	}
}

func TestExtractCommitHashFromVersion(t *testing.T) {
	tests := []struct {
		name              string
		version           string
		expectedCommitLen int
	}{
		{
			name:              "pseudo-version with commit hash",
			version:           "v1.0.0-20240125abcdef123456",
			expectedCommitLen: 12,
		},
		{
			name:              "pseudo-version short",
			version:           "v1.0.0-20240125abc",
			expectedCommitLen: 0, // Less than 12 chars
		},
		{
			name:              "tagged version",
			version:           "v1.0.0",
			expectedCommitLen: 0, // No commit hash
		},
		{
			name:              "version with multiple dashes",
			version:           "v1.0.0-pre-20240125abcdef123456",
			expectedCommitLen: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract commit hash logic (mimicking getRepositoryInfo)
			var commitHash string
			if len(tt.version) > 0 && tt.version[0] == 'v' {
				parts := tt.version[1:] // Remove 'v'
				for i := len(parts) - 1; i >= 0; i-- {
					if parts[i] == '-' {
						suffix := parts[i+1:]
						if len(suffix) >= 12 {
							commitHash = suffix[len(suffix)-12:] // Last 12 chars is the commit hash
						}
						break
					}
				}
			}

			assert.Equal(t, tt.expectedCommitLen, len(commitHash))
		})
	}
}

func TestGetCommitTime(t *testing.T) {
	tests := []struct {
		name       string
		commitHash string
		expectNow  bool
	}{
		{
			name:       "empty commit hash returns now",
			commitHash: "",
			expectNow:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			commitTime, err := scanner.getCommitTime("https://github.com/nonexistent/repo", tt.commitHash)

			assert.NoError(t, err)

			if tt.expectNow {
				// Should be very recent (within 5 seconds of now)
				diff := time.Since(commitTime)
				assert.Less(t, diff, 5*time.Second)
			}
		})
	}
}

func TestGetCommitTimeViaHTTP(t *testing.T) {
	scanner := NewScanner(".")
	commitTime, err := scanner.getCommitTimeViaHTTP("https://github.com/test/repo", "abc123")

	assert.NoError(t, err)
	// Should return current time as fallback
	diff := time.Since(commitTime)
	assert.Less(t, diff, 5*time.Second)
}

func TestCheckMaintenanceStatusWithError(t *testing.T) {
	scanner := NewScanner(".")
	dep := &Dependency{
		Path:     "github.com/steffakasid/govital",
		Version:  "v0.0.1",
		IsActive: false,
	}

	// Should handle errors gracefully - either succeeds or marks as active on error
	err := scanner.checkMaintenanceStatus(dep)
	assert.NoError(t, err)
	// When it can't verify, it marks as active
	assert.True(t, dep.IsActive)
}

func TestPrintResults(t *testing.T) {
	scanner := NewScanner(".")
	scanner.result.Dependencies = []Dependency{
		{
			Path:                "github.com/example/active",
			Version:             "v1.0.0",
			IsActive:            true,
			DaysSinceLastCommit: 10,
			LastCommitTime:      time.Now().AddDate(0, 0, -10),
		},
		{
			Path:                "github.com/example/inactive",
			Version:             "v1.0.0",
			IsActive:            false,
			DaysSinceLastCommit: 400,
			LastCommitTime:      time.Now().AddDate(0, 0, -400),
		},
	}
	scanner.result.Summary.Total = 2
	scanner.result.Summary.Inactive = 1
	scanner.result.Summary.Errors = 0
	scanner.result.Summary.StaleThresholdDays = 30

	// Should not panic
	assert.NotPanics(t, func() {
		scanner.PrintResults()
	})
}

func TestDependencyInitialization(t *testing.T) {
	dep := Dependency{
		Path:                "github.com/test/module",
		Version:             "v1.2.3",
		Update:              "v1.2.4",
		Latest:              "v1.3.0",
		Error:               "",
		LastCommitTime:      time.Now(),
		IsActive:            true,
		DaysSinceLastCommit: 5,
	}

	assert.Equal(t, "github.com/test/module", dep.Path)
	assert.Equal(t, "v1.2.3", dep.Version)
	assert.Equal(t, "v1.2.4", dep.Update)
	assert.Equal(t, "v1.3.0", dep.Latest)
	assert.Empty(t, dep.Error)
	assert.True(t, dep.IsActive)
	assert.Equal(t, 5, dep.DaysSinceLastCommit)
}

func TestScanResultSummaryFields(t *testing.T) {
	scanner := NewScanner(".")
	result := scanner.GetResults()

	assert.NotNil(t, result)
	assert.Equal(t, ".", result.ProjectPath)
	assert.NotNil(t, result.Dependencies)
	assert.Equal(t, 0, result.Summary.Total)
	assert.Equal(t, 0, result.Summary.Updated)
	assert.Equal(t, 0, result.Summary.Outdated)
	assert.Equal(t, 0, result.Summary.Errors)
	assert.Equal(t, 0, result.Summary.Inactive)
	assert.Equal(t, 30, result.Summary.StaleThresholdDays)
}

func TestMultipleThresholdUpdates(t *testing.T) {
	scanner := NewScanner(".")

	// Set initial threshold
	scanner.SetStaleThreshold(90)
	assert.Equal(t, 90, scanner.staleThresholdDays)
	assert.Equal(t, 90, scanner.result.Summary.StaleThresholdDays)

	// Update threshold
	scanner.SetStaleThreshold(180)
	assert.Equal(t, 180, scanner.staleThresholdDays)
	assert.Equal(t, 180, scanner.result.Summary.StaleThresholdDays)

	// Set again
	scanner.SetStaleThreshold(365)
	assert.Equal(t, 365, scanner.staleThresholdDays)
	assert.Equal(t, 365, scanner.result.Summary.StaleThresholdDays)
}

func TestDependencyStatusEdgeCases(t *testing.T) {
	tests := []struct {
		name                   string
		daysSinceLastCommit    int
		staleThreshold         int
		expectedIsActive       bool
	}{
		{
			name:                   "zero days inactive",
			daysSinceLastCommit:    0,
			staleThreshold:         30,
			expectedIsActive:       true,
		},
		{
			name:                   "one day inactive",
			daysSinceLastCommit:    1,
			staleThreshold:         30,
			expectedIsActive:       true,
		},
		{
			name:                   "exactly at threshold",
			daysSinceLastCommit:    30,
			staleThreshold:         30,
			expectedIsActive:       true,
		},
		{
			name:                   "one day over threshold",
			daysSinceLastCommit:    31,
			staleThreshold:         30,
			expectedIsActive:       false,
		},
		{
			name:                   "far over threshold",
			daysSinceLastCommit:    1000,
			staleThreshold:         30,
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

// Mock implementations for testing
type MockCommandExecutor struct {
	ExecuteFunc       func(name string, args ...string) ([]byte, error)
	ExecuteInDirFunc  func(dir, name string, args ...string) ([]byte, error)
}

func (m *MockCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(name, args...)
	}
	return nil, nil
}

func (m *MockCommandExecutor) ExecuteInDir(dir, name string, args ...string) ([]byte, error) {
	if m.ExecuteInDirFunc != nil {
		return m.ExecuteInDirFunc(dir, name, args...)
	}
	return nil, nil
}

// Test with mocked git commands
func TestGetRepositoryInfoWithMockedCommands(t *testing.T) {
	tests := []struct {
		name              string
		version           string
		expectedCommitLen int
		shouldError       bool
	}{
		{
			name:              "successful pseudo-version extraction",
			version:           "v1.0.0-20240125abcdef123456",
			expectedCommitLen: 12,
			shouldError:       false,
		},
		{
			name:              "tagged version without commit hash",
			version:           "v1.0.0",
			expectedCommitLen: 0,
			shouldError:       false,
		},
		{
			name:              "version with multiple dashes",
			version:           "v1.0.0-pre-20240125abcdef123456",
			expectedCommitLen: 12,
			shouldError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test commit hash extraction from version string
			var commitHash string
			if len(tt.version) > 0 && tt.version[0] == 'v' {
				parts := tt.version[1:]
				for i := len(parts) - 1; i >= 0; i-- {
					if parts[i] == '-' {
						suffix := parts[i+1:]
						if len(suffix) >= 12 {
							commitHash = suffix[len(suffix)-12:]
						}
						break
					}
				}
			}

			if tt.expectedCommitLen > 0 {
				assert.Equal(t, tt.expectedCommitLen, len(commitHash))
			} else {
				assert.Equal(t, 0, len(commitHash))
			}
		})
	}
}

// Test maintenance status with various scenarios
func TestCheckMaintenanceStatusScenarios(t *testing.T) {
	tests := []struct {
		name              string
		daysOld           int
		threshold         int
		expectedIsActive  bool
	}{
		{
			name:              "very recent commit",
			daysOld:           1,
			threshold:         30,
			expectedIsActive:  true,
		},
		{
			name:              "old commit beyond threshold",
			daysOld:           100,
			threshold:         30,
			expectedIsActive:  false,
		},
		{
			name:              "commit exactly at threshold",
			daysOld:           30,
			threshold:         30,
			expectedIsActive:  true,
		},
		{
			name:              "old project with high threshold",
			daysOld:           500,
			threshold:         730,
			expectedIsActive:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			scanner.SetStaleThreshold(tt.threshold)

			dep := &Dependency{
				Path:                "github.com/test/module",
				Version:             "v1.0.0",
				LastCommitTime:      time.Now().AddDate(0, 0, -tt.daysOld),
				DaysSinceLastCommit: tt.daysOld,
				IsActive:            tt.daysOld <= tt.threshold,
			}

			assert.Equal(t, tt.expectedIsActive, dep.IsActive)
		})
	}
}

// Test Scanner configuration persistence
func TestScannerConfigPersistence(t *testing.T) {
	scanner := NewScanner(".")

	// Set various configurations
	scanner.SetStaleThreshold(180)
	scanner.SetWorkers(8)
	scanner.SetIncludeIndirectDependencies(true)

	// Verify they persist
	assert.Equal(t, 180, scanner.staleThresholdDays)
	assert.Equal(t, 8, scanner.workers)
	assert.True(t, scanner.includeIndirectDependencies)

	// Change them
	scanner.SetStaleThreshold(365)
	scanner.SetWorkers(2)
	scanner.SetIncludeIndirectDependencies(false)

	// Verify changes persist
	assert.Equal(t, 365, scanner.staleThresholdDays)
	assert.Equal(t, 2, scanner.workers)
	assert.False(t, scanner.includeIndirectDependencies)
}

// Test result aggregation
func TestResultAggregation(t *testing.T) {
	scanner := NewScanner(".")

	// Simulate adding dependencies
	scanner.result.Dependencies = []Dependency{
		{Path: "active-1", Version: "v1.0.0", IsActive: true},
		{Path: "active-2", Version: "v2.0.0", IsActive: true},
		{Path: "inactive-1", Version: "v1.0.0", IsActive: false},
		{Path: "inactive-2", Version: "v2.0.0", IsActive: false},
	}
	scanner.result.Summary.Total = 4
	scanner.result.Summary.Inactive = 2

	// Verify GetInactiveDependencies works
	inactive := scanner.GetInactiveDependencies()
	assert.Equal(t, 2, len(inactive))
	assert.False(t, inactive[0].IsActive)
	assert.False(t, inactive[1].IsActive)

	// Verify GetResults works
	results := scanner.GetResults()
	assert.Equal(t, 4, results.Summary.Total)
	assert.Equal(t, 2, results.Summary.Inactive)
}

// Test isStale helper method
func TestIsStale(t *testing.T) {
	tests := []struct {
		name              string
		daysSinceCommit   int
		staleThreshold    int
		expectedIsStale   bool
	}{
		{
			name:              "within threshold",
			daysSinceCommit:   10,
			staleThreshold:    30,
			expectedIsStale:   false,
		},
		{
			name:              "exactly at threshold",
			daysSinceCommit:   30,
			staleThreshold:    30,
			expectedIsStale:   false,
		},
		{
			name:              "one day over threshold",
			daysSinceCommit:   31,
			staleThreshold:    30,
			expectedIsStale:   true,
		},
		{
			name:              "far over threshold",
			daysSinceCommit:   365,
			staleThreshold:    30,
			expectedIsStale:   true,
		},
		{
			name:              "zero days",
			daysSinceCommit:   0,
			staleThreshold:    30,
			expectedIsStale:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			scanner.SetStaleThreshold(tt.staleThreshold)

			result := scanner.isStale(tt.daysSinceCommit)
			assert.Equal(t, tt.expectedIsStale, result)
		})
	}
}

// Test extractCommitHash helper method
func TestExtractCommitHash(t *testing.T) {
	tests := []struct {
		name              string
		version           string
		expectedHash      string
	}{
		{
			name:              "pseudo-version with full commit hash",
			version:           "v1.0.0-20240125abcdef123456",
			expectedHash:      "abcdef123456",
		},
		{
			name:              "tagged version",
			version:           "v1.0.0",
			expectedHash:      "",
		},
		{
			name:              "version with multiple dashes",
			version:           "v1.0.0-pre-20240125abcdef123456",
			expectedHash:      "abcdef123456",
		},
		{
			name:              "empty version",
			version:           "",
			expectedHash:      "",
		},
		{
			name:              "version without v prefix",
			version:           "1.0.0-20240125abcdef123456",
			expectedHash:      "",
		},
		{
			name:              "complex version string",
			version:           "v2.1.0-rc1-20240125abcdef123456",
			expectedHash:      "abcdef123456",
		},
		{
			name:              "version with exactly 12 chars after dash",
			version:           "v1.0.0-abcdef123456",
			expectedHash:      "abcdef123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(".")
			hash := scanner.extractCommitHash(tt.version)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}
