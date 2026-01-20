//go:build !production

package testsafety

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAllTestFilesImportTestsafety ensures every _test.go file in the project
// imports the testsafety package. This prevents accidentally creating test files
// that could run on the host machine without safety checks.
func TestAllTestFilesImportTestsafety(t *testing.T) {
	// Find the project root (walk up looking for go.mod)
	projectRoot := findProjectRoot(t)

	var missingImport []string

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor directories
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}

		// Only check _test.go files
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip this test file itself
		if strings.HasSuffix(path, "import_check_test.go") {
			return nil
		}

		// Check if the file imports testsafety
		if !fileImportsTestsafety(t, path) {
			relPath, _ := filepath.Rel(projectRoot, path)
			missingImport = append(missingImport, relPath)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk project: %v", err)
	}

	if len(missingImport) > 0 {
		t.Errorf("The following test files do not import testsafety:\n")
		for _, path := range missingImport {
			t.Errorf("  - %s", path)
		}
		t.Errorf("\nAll test files must include:\n  import _ \"github.com/happycollision/ribbin/internal/testsafety\"")
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from the directory containing this test file
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find project root (go.mod)")
		}
		dir = parent
	}
}

func fileImportsTestsafety(t *testing.T, path string) bool {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Check for the testsafety import (with or without alias)
		if strings.Contains(line, `"github.com/happycollision/ribbin/internal/testsafety"`) {
			return true
		}
	}

	return false
}
