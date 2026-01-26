//go:build integration
// +build integration

package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScanWithRealGoProject tests scanning against the govital project itself
func TestScanWithRealGoProject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get the path to govital's go.mod
	govitalPath := filepath.Join("..", "..")
	goModPath := filepath.Join(govitalPath, "go.mod")

	// Verify go.mod exists
	_, err := os.Stat(goModPath)
	require.NoError(t, err, "go.mod should exist in govital project")

	scanner := NewScanner(govitalPath)
	scanner.SetStaleThreshold(30)
	scanner.SetWorkers(2)
	scanner.SetIncludeIndirectDependencies(false)

	err = scanner.Scan()
	require.NoError(t, err, "Scan should succeed")

	result := scanner.GetResults()
	assert.NotNil(t, result)
	assert.Equal(t, govitalPath, result.ProjectPath)
	assert.Greater(t, result.Summary.Total, 0, "Should find direct dependencies")
	assert.NotEmpty(t, result.Dependencies, "Should have dependencies")
}

// TestScanDirectDependencies tests that direct dependencies are correctly identified
func TestScanDirectDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	govitalPath := filepath.Join("..", "..")

	scanner := NewScanner(govitalPath)
	scanner.SetStaleThreshold(30)
	scanner.SetIncludeIndirectDependencies(false)

	err := scanner.Scan()
	require.NoError(t, err)

	result := scanner.GetResults()

	// govital should have these direct dependencies in go.mod
	hasSpfCobra := false
	hasSpfViper := false
	hasEstlog := false

	for _, dep := range result.Dependencies {
		if dep.Path == "github.com/spf13/cobra" {
			hasSpfCobra = true
		}
		if dep.Path == "github.com/spf13/viper" {
			hasSpfViper = true
		}
		if dep.Path == "github.com/steffakasid/eslog" {
			hasEstlog = true
		}
	}

	assert.True(t, hasSpfCobra, "Should find github.com/spf13/cobra")
	assert.True(t, hasSpfViper, "Should find github.com/spf13/viper")
	assert.True(t, hasEstlog, "Should find github.com/steffakasid/eslog")
}

// TestScanWithIndirectDependencies tests that indirect dependencies can be included
func TestScanWithIndirectDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	govitalPath := filepath.Join("..", "..")

	scanner := NewScanner(govitalPath)
	scanner.SetIncludeIndirectDependencies(true)
	scanner.SetStaleThreshold(30)

	err := scanner.Scan()
	require.NoError(t, err)

	resultWithIndirect := scanner.GetResults()

	// Now scan without indirect
	scanner2 := NewScanner(govitalPath)
	scanner2.SetIncludeIndirectDependencies(false)
	scanner2.SetStaleThreshold(30)

	err = scanner2.Scan()
	require.NoError(t, err)

	resultWithoutIndirect := scanner2.GetResults()

	// Including indirect should result in more dependencies
	assert.GreaterOrEqual(t, resultWithIndirect.Summary.Total, resultWithoutIndirect.Summary.Total,
		"Should have >= dependencies when including indirect")
}

// TestScanResultConsistency tests that multiple scans return consistent results
func TestScanResultConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	govitalPath := filepath.Join("..", "..")

	scanner1 := NewScanner(govitalPath)
	scanner1.SetStaleThreshold(30)
	scanner1.SetIncludeIndirectDependencies(false)

	err := scanner1.Scan()
	require.NoError(t, err)
	result1 := scanner1.GetResults()

	scanner2 := NewScanner(govitalPath)
	scanner2.SetStaleThreshold(30)
	scanner2.SetIncludeIndirectDependencies(false)

	err = scanner2.Scan()
	require.NoError(t, err)
	result2 := scanner2.GetResults()

	// Results should be consistent
	assert.Equal(t, result1.Summary.Total, result2.Summary.Total)
	assert.Equal(t, len(result1.Dependencies), len(result2.Dependencies))

	// Dependency count by status should match
	inactivCount1 := scanner1.GetInactiveDependencies()
	inactivCount2 := scanner2.GetInactiveDependencies()
	assert.Equal(t, len(inactivCount1), len(inactivCount2))
}

// TestScanWithDifferentThresholds tests that threshold changes affect results
func TestScanWithDifferentThresholds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	govitalPath := filepath.Join("..", "..")

	// Scan with strict threshold
	scanner1 := NewScanner(govitalPath)
	scanner1.SetStaleThreshold(30) // Very strict - 30 days
	scanner1.SetIncludeIndirectDependencies(false)

	err := scanner1.Scan()
	require.NoError(t, err)
	inactiveStrict := scanner1.GetInactiveDependencies()

	// Scan with lenient threshold
	scanner2 := NewScanner(govitalPath)
	scanner2.SetStaleThreshold(730) // Very lenient - 2 years
	scanner2.SetIncludeIndirectDependencies(false)

	err = scanner2.Scan()
	require.NoError(t, err)
	inactiveLenient := scanner2.GetInactiveDependencies()

	// Stricter threshold should flag more as inactive
	assert.GreaterOrEqual(t, len(inactiveStrict), len(inactiveLenient),
		"Stricter threshold should have more inactive dependencies")
}

// TestParallelScanConsistency tests that parallel scanning produces consistent results
func TestParallelScanConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	govitalPath := filepath.Join("..", "..")

	results := make([]int, 0)

	// Run multiple scans with different worker counts
	for workers := 1; workers <= 4; workers++ {
		scanner := NewScanner(govitalPath)
		scanner.SetWorkers(workers)
		scanner.SetStaleThreshold(30)
		scanner.SetIncludeIndirectDependencies(false)

		err := scanner.Scan()
		require.NoError(t, err)

		result := scanner.GetResults()
		results = append(results, result.Summary.Total)
	}

	// All worker counts should find the same number of dependencies
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i],
			"Different worker counts should find same number of dependencies")
	}
}

// TestPrintResultsWithRealData tests printing results with actual scan data
func TestPrintResultsWithRealData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	govitalPath := filepath.Join("..", "..")
	scanner := NewScanner(govitalPath)
	scanner.SetStaleThreshold(30)
	scanner.SetIncludeIndirectDependencies(false)

	err := scanner.Scan()
	require.NoError(t, err)

	// Should not panic when printing
	assert.NotPanics(t, func() {
		scanner.PrintResults()
	})
}
