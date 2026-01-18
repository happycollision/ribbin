package shim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/happycollision/ribbin/internal/config"
)

func TestSidecarPath(t *testing.T) {
	path := SidecarPath("/usr/local/bin/cat")
	expected := "/usr/local/bin/cat.ribbin-original"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestInstall(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates symlink and renames original", func(t *testing.T) {
		// Create original binary
		binaryPath := filepath.Join(tmpDir, "test-binary")
		if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho original"), 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		// Create fake ribbin
		ribbinPath := filepath.Join(tmpDir, "ribbin")
		if err := os.WriteFile(ribbinPath, []byte("#!/bin/sh\necho ribbin"), 0755); err != nil {
			t.Fatalf("failed to create ribbin: %v", err)
		}

		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
		}

		err := Install(binaryPath, ribbinPath, registry, "/project/ribbin.toml")
		if err != nil {
			t.Fatalf("Install error: %v", err)
		}

		// Check sidecar exists
		sidecarPath := SidecarPath(binaryPath)
		if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
			t.Error("sidecar should exist after install")
		}

		// Check symlink points to ribbin
		target, err := os.Readlink(binaryPath)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if target != ribbinPath {
			t.Errorf("symlink should point to ribbin, got %s", target)
		}

		// Check registry updated
		entry, exists := registry.Shims["test-binary"]
		if !exists {
			t.Error("registry should have entry for test-binary")
		}
		if entry.Original != binaryPath {
			t.Errorf("registry Original should be %s, got %s", binaryPath, entry.Original)
		}
		if entry.Config != "/project/ribbin.toml" {
			t.Errorf("registry Config should be /project/ribbin.toml, got %s", entry.Config)
		}
	})

	t.Run("fails when already shimmed", func(t *testing.T) {
		// Create original binary
		binaryPath := filepath.Join(tmpDir, "already-shimmed")
		if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		// Create sidecar (simulating already shimmed)
		sidecarPath := SidecarPath(binaryPath)
		if err := os.WriteFile(sidecarPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		ribbinPath := filepath.Join(tmpDir, "ribbin")
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
		}

		err := Install(binaryPath, ribbinPath, registry, "/project/ribbin.toml")
		if err == nil {
			t.Error("expected error when binary is already shimmed")
		}
	})

	t.Run("rolls back on symlink failure", func(t *testing.T) {
		// Create original binary
		binaryPath := filepath.Join(tmpDir, "rollback-test")
		originalContent := []byte("#!/bin/sh\necho original")
		if err := os.WriteFile(binaryPath, originalContent, 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		// Create a directory at ribbinPath (will cause symlink to fail if we try to create it there)
		// Actually we need the symlink target to fail - use an empty path
		ribbinPath := "" // Empty path will cause symlink to fail

		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
		}

		err := Install(binaryPath, ribbinPath, registry, "/project/ribbin.toml")
		if err == nil {
			t.Error("expected error with empty ribbin path")
		}

		// Original should be restored (though it might be at sidecar location)
		// Check that we haven't left things in a broken state
		sidecarPath := SidecarPath(binaryPath)

		// Either original exists or sidecar exists
		origExists := false
		if _, err := os.Stat(binaryPath); err == nil {
			origExists = true
		}
		sidecarExists := false
		if _, err := os.Stat(sidecarPath); err == nil {
			sidecarExists = true
		}

		if !origExists && !sidecarExists {
			t.Error("neither original nor sidecar exists - rollback failed")
		}
	})
}

func TestUninstall(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("removes symlink and restores original", func(t *testing.T) {
		binaryPath := filepath.Join(tmpDir, "uninstall-test")
		ribbinPath := filepath.Join(tmpDir, "ribbin")
		sidecarPath := SidecarPath(binaryPath)

		// Create sidecar (original)
		originalContent := []byte("#!/bin/sh\necho original")
		if err := os.WriteFile(sidecarPath, originalContent, 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		// Create symlink (shim)
		if err := os.WriteFile(ribbinPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create ribbin: %v", err)
		}
		if err := os.Symlink(ribbinPath, binaryPath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		registry := &config.Registry{
			Shims: map[string]config.ShimEntry{
				"uninstall-test": {Original: binaryPath, Config: "/project/ribbin.toml"},
			},
			Activations: make(map[int]config.ActivationEntry),
		}

		err := Uninstall(binaryPath, registry)
		if err != nil {
			t.Fatalf("Uninstall error: %v", err)
		}

		// Sidecar should be gone
		if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
			t.Error("sidecar should not exist after uninstall")
		}

		// Original should be restored
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			t.Error("original should be restored after uninstall")
		}

		// Check it's no longer a symlink
		info, err := os.Lstat(binaryPath)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Error("binary should not be a symlink after uninstall")
		}

		// Registry should be updated
		if _, exists := registry.Shims["uninstall-test"]; exists {
			t.Error("registry entry should be removed after uninstall")
		}
	})

	t.Run("fails when sidecar doesn't exist", func(t *testing.T) {
		binaryPath := filepath.Join(tmpDir, "no-sidecar")
		if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
		}

		err := Uninstall(binaryPath, registry)
		if err == nil {
			t.Error("expected error when sidecar doesn't exist")
		}
	})
}

func TestFindSidecars(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("finds sidecars in directory", func(t *testing.T) {
		// Create some sidecar files
		sidecar1 := filepath.Join(tmpDir, "cat.ribbin-original")
		sidecar2 := filepath.Join(tmpDir, "ls.ribbin-original")
		regularFile := filepath.Join(tmpDir, "regular-file")

		for _, f := range []string{sidecar1, sidecar2, regularFile} {
			if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0755); err != nil {
				t.Fatalf("failed to create file: %v", err)
			}
		}

		sidecars, err := FindSidecars([]string{tmpDir})
		if err != nil {
			t.Fatalf("FindSidecars error: %v", err)
		}

		if len(sidecars) != 2 {
			t.Errorf("expected 2 sidecars, got %d", len(sidecars))
		}
	})

	t.Run("handles missing directory", func(t *testing.T) {
		sidecars, err := FindSidecars([]string{"/nonexistent/path"})
		if err != nil {
			t.Fatalf("FindSidecars error: %v", err)
		}
		if len(sidecars) != 0 {
			t.Errorf("expected 0 sidecars, got %d", len(sidecars))
		}
	})

	t.Run("handles empty search paths", func(t *testing.T) {
		sidecars, err := FindSidecars([]string{})
		if err != nil {
			t.Fatalf("FindSidecars error: %v", err)
		}
		if len(sidecars) != 0 {
			t.Errorf("expected 0 sidecars, got %d", len(sidecars))
		}
	})

	t.Run("searches multiple directories", func(t *testing.T) {
		dir1 := filepath.Join(tmpDir, "dir1")
		dir2 := filepath.Join(tmpDir, "dir2")
		os.MkdirAll(dir1, 0755)
		os.MkdirAll(dir2, 0755)

		os.WriteFile(filepath.Join(dir1, "cmd1.ribbin-original"), []byte(""), 0755)
		os.WriteFile(filepath.Join(dir2, "cmd2.ribbin-original"), []byte(""), 0755)

		sidecars, err := FindSidecars([]string{dir1, dir2})
		if err != nil {
			t.Fatalf("FindSidecars error: %v", err)
		}
		if len(sidecars) != 2 {
			t.Errorf("expected 2 sidecars, got %d", len(sidecars))
		}
	})
}
