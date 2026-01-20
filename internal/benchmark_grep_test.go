//go:build integration

package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/wrap"
)

// BenchmarkShimOverheadGrep measures the overhead of having a ribbin shim in place
// on a longer-running command (grep searching through multiple files).
func BenchmarkShimOverheadGrep(b *testing.B) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "ribbin-bench-grep-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	binDir := filepath.Join(tmpDir, "bin")
	projectDir := filepath.Join(tmpDir, "project")
	searchDir := filepath.Join(tmpDir, "search")

	for _, dir := range []string{homeDir, binDir, projectDir, searchDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create 100 files with 100 lines each (10,000 lines total to search)
	for i := 0; i < 100; i++ {
		filename := filepath.Join(searchDir, fmt.Sprintf("file_%03d.txt", i))
		var content string
		for j := 0; j < 100; j++ {
			if j%10 == 0 {
				content += "MATCH_PATTERN line number\n"
			} else {
				content += "some random text on this line\n"
			}
		}
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create file %s: %v", filename, err)
		}
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

	// Find real grep binary
	realGrepPath, err := exec.LookPath("grep")
	if err != nil {
		b.Fatalf("failed to find grep: %v", err)
	}

	// Create a wrapper script for grep (Alpine's grep is busybox, so we need a wrapper)
	grepPath := filepath.Join(binDir, "grep")
	grepWrapper := `#!/bin/sh
exec ` + realGrepPath + ` "$@"
`
	if err := os.WriteFile(grepPath, []byte(grepWrapper), 0755); err != nil {
		b.Fatalf("failed to create grep wrapper: %v", err)
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

	// Create ribbin.toml (grep is NOT blocked - it's passthrough)
	// This tests the overhead of the shim decision logic
	configContent := `[wrappers.grep]
action = "passthrough"
message = ""
`
	configPath := filepath.Join(projectDir, "ribbin.toml")
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
	if err := wrap.Install(grepPath, ribbinPath, registry, configPath); err != nil {
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
	warmupCmd := exec.Command(grepPath, "-r", "MATCH_PATTERN", searchDir)
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
			cmd := exec.Command(grepPath, "-r", "MATCH_PATTERN", searchDir)
			cmd.Env = append(os.Environ(),
				"HOME="+homeDir,
				"PATH="+binDir+":"+origPath,
			)
			if _, err := cmd.CombinedOutput(); err != nil {
				b.Fatalf("grep command failed: %v", err)
			}
		}
		elapsed := time.Since(start)
		b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "ns/op")
	})

	// Uninstall shim for comparison
	if err := wrap.Uninstall(grepPath, registry); err != nil {
		b.Fatalf("failed to uninstall shim: %v", err)
	}

	// Benchmark without shim
	b.Run("WithoutShim", func(b *testing.B) {
		start := time.Now()
		for i := 0; i < b.N; i++ {
			cmd := exec.Command(grepPath, "-r", "MATCH_PATTERN", searchDir)
			cmd.Env = append(os.Environ(),
				"HOME="+homeDir,
				"PATH="+binDir+":"+origPath,
			)
			if _, err := cmd.CombinedOutput(); err != nil {
				b.Fatalf("grep command failed: %v", err)
			}
		}
		elapsed := time.Since(start)
		b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "ns/op")
	})
}
