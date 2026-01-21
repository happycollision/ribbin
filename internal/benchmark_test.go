package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/wrap"
)

// BenchmarkShimOverhead measures the overhead of having a ribbin shim in place
// by running cat command one million times on a 10-line text file.
func BenchmarkShimOverhead(b *testing.B) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "ribbin-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	binDir := filepath.Join(tmpDir, "bin")
	projectDir := filepath.Join(tmpDir, "project")

	for _, dir := range []string{homeDir, binDir, projectDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create a 10-line test file
	testFilePath := filepath.Join(tmpDir, "test.txt")
	testContent := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}

	// Save original environment
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	origDir, _ := os.Getwd()

	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
		os.Chdir(origDir)
	}()

	os.Setenv("HOME", homeDir)

	// Find real cat binary
	realCatPath, err := exec.LookPath("cat")
	if err != nil {
		b.Fatalf("failed to find cat: %v", err)
	}

	// Create a wrapper script for cat (Alpine's cat is busybox, so we need a wrapper)
	catPath := filepath.Join(binDir, "cat")
	catWrapper := `#!/bin/sh
exec ` + realCatPath + ` "$@"
`
	if err := os.WriteFile(catPath, []byte(catWrapper), 0755); err != nil {
		b.Fatalf("failed to create cat wrapper: %v", err)
	}

	os.Setenv("PATH", binDir+":"+origPath)

	// Build ribbin
	ribbinPath := filepath.Join(binDir, "ribbin")
	buildCmd := exec.Command("go", "build", "-o", ribbinPath, "./cmd/ribbin")
	moduleRoot := findModuleRootBenchmark(b)
	buildCmd.Dir = moduleRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		b.Fatalf("failed to build ribbin: %v\n%s", err, output)
	}

	// Create ribbin.jsonc (cat is NOT blocked - it's passthrough)
	// This tests the overhead of the shim decision logic
	configContent := `{
  "wrappers": {
    "cat": {
      "action": "passthrough",
      "message": ""
    }
  }
}`
	configPath := filepath.Join(projectDir, "ribbin.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		b.Fatalf("failed to create config: %v", err)
	}

	// Create registry with GlobalOn = true
	registry := &config.Registry{
		Wrappers:       make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
		GlobalActive:    true,
	}

	// Install shim
	if err := wrap.Install(catPath, ribbinPath, registry, configPath); err != nil {
		b.Fatalf("failed to install shim: %v", err)
	}

	// Save registry
	registryDir := filepath.Join(homeDir, ".config", "ribbin")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		b.Fatalf("failed to create registry dir: %v", err)
	}
	registryPath := filepath.Join(registryDir, "registry.json")
	data, _ := json.MarshalIndent(registry, "", "  ")
	if err := os.WriteFile(registryPath, data, 0644); err != nil {
		b.Fatalf("failed to save registry: %v", err)
	}

	os.Chdir(projectDir)

	// Warm up - run once to ensure everything is loaded
	warmupCmd := exec.Command(catPath, testFilePath)
	warmupCmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+binDir+":"+origPath,
	)
	if output, err := warmupCmd.CombinedOutput(); err != nil {
		b.Fatalf("warmup failed: %v\nOutput: %s", err, output)
	}

	// Benchmark with shim
	b.Run("WithShim", func(b *testing.B) {
		start := time.Now()
		for i := 0; i < b.N; i++ {
			cmd := exec.Command(catPath, testFilePath)
			cmd.Env = append(os.Environ(),
				"HOME="+homeDir,
				"PATH="+binDir+":"+origPath,
			)
			if _, err := cmd.CombinedOutput(); err != nil {
				b.Fatalf("cat command failed: %v", err)
			}
		}
		elapsed := time.Since(start)
		b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "ns/op")
	})

	// Uninstall shim for comparison
	if err := wrap.Uninstall(catPath, registry); err != nil {
		b.Fatalf("failed to uninstall shim: %v", err)
	}

	// Benchmark without shim
	b.Run("WithoutShim", func(b *testing.B) {
		start := time.Now()
		for i := 0; i < b.N; i++ {
			cmd := exec.Command(catPath, testFilePath)
			cmd.Env = append(os.Environ(),
				"HOME="+homeDir,
				"PATH="+binDir+":"+origPath,
			)
			if _, err := cmd.CombinedOutput(); err != nil {
				b.Fatalf("cat command failed: %v", err)
			}
		}
		elapsed := time.Since(start)
		b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "ns/op")
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// findModuleRootBenchmark walks up the directory tree to find go.mod
func findModuleRootBenchmark(b *testing.B) string {
	dir, err := os.Getwd()
	if err != nil {
		b.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			b.Fatalf("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}
