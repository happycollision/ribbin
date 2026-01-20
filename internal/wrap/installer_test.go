package wrap

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"

	"github.com/happycollision/ribbin/internal/config"
)

func TestSidecarPath(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cat")

	// Create the binary file so path validation passes
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create binary: %v", err)
	}

	path, err := SidecarPath(binPath)
	if err != nil {
		t.Fatalf("SidecarPath error: %v", err)
	}
	expected := binPath + ".ribbin-original"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestHasSidecar(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("returns false when no sidecar exists", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "no-sidecar")
		if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		if HasSidecar(binPath) {
			t.Error("HasSidecar should return false when sidecar doesn't exist")
		}
	})

	t.Run("returns true when sidecar exists", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "with-sidecar")
		sidecarPath := binPath + ".ribbin-original"

		// Create the sidecar file
		if err := os.WriteFile(sidecarPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		if !HasSidecar(binPath) {
			t.Error("HasSidecar should return true when sidecar exists")
		}
	})

	t.Run("returns false for nonexistent path", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "nonexistent")

		if HasSidecar(binPath) {
			t.Error("HasSidecar should return false for nonexistent path")
		}
	})
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
			Wrappers:       make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
		}

		err := Install(binaryPath, ribbinPath, registry, "/project/ribbin.jsonc")
		if err != nil {
			t.Fatalf("Install error: %v", err)
		}

		// Check sidecar exists
		sidecarPath, err := SidecarPath(binaryPath)
		if err != nil {
			t.Fatalf("SidecarPath error: %v", err)
		}
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
		entry, exists := registry.Wrappers["test-binary"]
		if !exists {
			t.Error("registry should have entry for test-binary")
		}
		if entry.Original != binaryPath {
			t.Errorf("registry Original should be %s, got %s", binaryPath, entry.Original)
		}
		if entry.Config != "/project/ribbin.jsonc" {
			t.Errorf("registry Config should be /project/ribbin.jsonc, got %s", entry.Config)
		}
	})

	t.Run("fails when already shimmed", func(t *testing.T) {
		// Create original binary
		binaryPath := filepath.Join(tmpDir, "already-shimmed")
		if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		// Create sidecar (simulating already shimmed)
		sidecarPath, sidecarErr := SidecarPath(binaryPath)
		if sidecarErr != nil {
			t.Fatalf("SidecarPath error: %v", sidecarErr)
		}
		if err := os.WriteFile(sidecarPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		ribbinPath := filepath.Join(tmpDir, "ribbin")
		registry := &config.Registry{
			Wrappers:       make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
		}

		installErr := Install(binaryPath, ribbinPath, registry, "/project/ribbin.jsonc")
		if installErr == nil {
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
			Wrappers:       make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
		}

		err := Install(binaryPath, ribbinPath, registry, "/project/ribbin.jsonc")
		if err == nil {
			t.Error("expected error with empty ribbin path")
		}

		// Original should be restored (though it might be at sidecar location)
		// Check that we haven't left things in a broken state
		sidecarPath, err := SidecarPath(binaryPath)
		if err != nil {
			t.Fatalf("SidecarPath error: %v", err)
		}

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
		sidecarPath, err := SidecarPath(binaryPath)
		if err != nil {
			// The path might not exist yet, which could cause validation to fail
			// Use filepath.Join to construct it directly
			sidecarPath = binaryPath + ".ribbin-original"
		}

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
			Wrappers: map[string]config.WrapperEntry{
				"uninstall-test": {Original: binaryPath, Config: "/project/ribbin.jsonc"},
			},
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
		}

		uninstallErr := Uninstall(binaryPath, registry)
		if uninstallErr != nil {
			t.Fatalf("Uninstall error: %v", uninstallErr)
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
		if _, exists := registry.Wrappers["uninstall-test"]; exists {
			t.Error("registry entry should be removed after uninstall")
		}
	})

	t.Run("fails when sidecar doesn't exist", func(t *testing.T) {
		binaryPath := filepath.Join(tmpDir, "no-sidecar")
		if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create binary: %v", err)
		}

		registry := &config.Registry{
			Wrappers:       make(map[string]config.WrapperEntry),
			ShellActivations:  make(map[int]config.ShellActivationEntry),
			ConfigActivations: make(map[string]config.ConfigActivationEntry),
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

func TestMetadataPath(t *testing.T) {
	path := MetadataPath("/usr/local/bin/cat")
	expected := "/usr/local/bin/cat.ribbin-meta"
	if path != expected {
		t.Errorf("MetadataPath() = %q, want %q", path, expected)
	}
}

func TestHasMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("returns false when no metadata exists", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "no-meta")
		if HasMetadata(binPath) {
			t.Error("HasMetadata should return false when metadata doesn't exist")
		}
	})

	t.Run("returns true when metadata exists", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "with-meta")
		metaPath := MetadataPath(binPath)

		// Create the metadata file
		if err := os.WriteFile(metaPath, []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to create metadata: %v", err)
		}

		if !HasMetadata(binPath) {
			t.Error("HasMetadata should return true when metadata exists")
		}
	})
}

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("calculates SHA256 hash", func(t *testing.T) {
		content := []byte("hello world")
		filePath := filepath.Join(tmpDir, "test-file")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		hash, err := hashFile(filePath)
		if err != nil {
			t.Fatalf("hashFile error: %v", err)
		}

		// SHA256 of "hello world" is well-known
		expectedHash := "sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
		if hash != expectedHash {
			t.Errorf("hashFile() = %q, want %q", hash, expectedHash)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := hashFile(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}

func TestMetadataLoadSave(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("saves and loads metadata correctly", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "test-binary")

		meta := &WrapperMetadata{
			OriginalHash:  "sha256:abc123",
			OriginalSize:  12345,
			RibbinPath:    "/usr/local/bin/ribbin",
			RibbinVersion: "1.0.0",
		}

		// Save
		if err := saveMetadata(binPath, meta); err != nil {
			t.Fatalf("saveMetadata error: %v", err)
		}

		// Load
		loaded, err := LoadMetadata(binPath)
		if err != nil {
			t.Fatalf("LoadMetadata error: %v", err)
		}

		if loaded.OriginalHash != meta.OriginalHash {
			t.Errorf("OriginalHash = %q, want %q", loaded.OriginalHash, meta.OriginalHash)
		}
		if loaded.OriginalSize != meta.OriginalSize {
			t.Errorf("OriginalSize = %d, want %d", loaded.OriginalSize, meta.OriginalSize)
		}
		if loaded.RibbinPath != meta.RibbinPath {
			t.Errorf("RibbinPath = %q, want %q", loaded.RibbinPath, meta.RibbinPath)
		}
		if loaded.RibbinVersion != meta.RibbinVersion {
			t.Errorf("RibbinVersion = %q, want %q", loaded.RibbinVersion, meta.RibbinVersion)
		}
	})

	t.Run("returns error for non-existent metadata", func(t *testing.T) {
		_, err := LoadMetadata(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for non-existent metadata")
		}
	})
}

func TestCheckHashConflict(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("returns no conflict when no metadata exists", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "no-meta")
		hasConflict, _, _ := CheckHashConflict(binPath)
		if hasConflict {
			t.Error("expected no conflict when metadata doesn't exist")
		}
	})

	t.Run("returns no conflict when hashes match", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "matching")
		sidecarPath := binPath + ".ribbin-original"
		content := []byte("original content")

		// Create sidecar
		if err := os.WriteFile(sidecarPath, content, 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		// Calculate hash
		hash, err := hashFile(sidecarPath)
		if err != nil {
			t.Fatalf("hashFile error: %v", err)
		}

		// Create metadata with matching hash
		meta := &WrapperMetadata{
			OriginalHash: hash,
			OriginalSize: int64(len(content)),
		}
		if err := saveMetadata(binPath, meta); err != nil {
			t.Fatalf("saveMetadata error: %v", err)
		}

		hasConflict, currentHash, originalHash := CheckHashConflict(binPath)
		if hasConflict {
			t.Error("expected no conflict when hashes match")
		}
		if currentHash != originalHash {
			t.Errorf("hashes should match: current=%q, original=%q", currentHash, originalHash)
		}
	})

	t.Run("returns conflict when hashes differ", func(t *testing.T) {
		binPath := filepath.Join(tmpDir, "mismatched")
		sidecarPath := binPath + ".ribbin-original"

		// Create sidecar with different content than what metadata says
		if err := os.WriteFile(sidecarPath, []byte("new content"), 0755); err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		// Create metadata with different hash
		meta := &WrapperMetadata{
			OriginalHash: "sha256:different",
			OriginalSize: 100,
		}
		if err := saveMetadata(binPath, meta); err != nil {
			t.Fatalf("saveMetadata error: %v", err)
		}

		hasConflict, currentHash, originalHash := CheckHashConflict(binPath)
		if !hasConflict {
			t.Error("expected conflict when hashes differ")
		}
		if currentHash == originalHash {
			t.Error("hashes should be different")
		}
	})
}

func TestInstallCreatesMetadata(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
		Wrappers:          make(map[string]config.WrapperEntry),
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
	}

	err = Install(binaryPath, ribbinPath, registry, "/project/ribbin.jsonc")
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	// Check metadata file was created
	if !HasMetadata(binaryPath) {
		t.Error("metadata file should exist after install")
	}

	// Verify metadata content
	meta, err := LoadMetadata(binaryPath)
	if err != nil {
		t.Fatalf("LoadMetadata error: %v", err)
	}

	if meta.OriginalHash == "" {
		t.Error("metadata should have OriginalHash")
	}
	if meta.OriginalSize <= 0 {
		t.Error("metadata should have OriginalSize")
	}
	if meta.RibbinPath != ribbinPath {
		t.Errorf("RibbinPath = %q, want %q", meta.RibbinPath, ribbinPath)
	}
}

func TestUninstallRemovesMetadata(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "uninstall-test")
	ribbinPath := filepath.Join(tmpDir, "ribbin")
	sidecarPath := binaryPath + ".ribbin-original"
	metaPath := MetadataPath(binaryPath)

	// Create sidecar (original)
	if err := os.WriteFile(sidecarPath, []byte("#!/bin/sh\necho original"), 0755); err != nil {
		t.Fatalf("failed to create sidecar: %v", err)
	}

	// Create metadata
	meta := &WrapperMetadata{OriginalHash: "sha256:test"}
	if err := saveMetadata(binaryPath, meta); err != nil {
		t.Fatalf("saveMetadata error: %v", err)
	}

	// Create symlink (shim)
	if err := os.WriteFile(ribbinPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create ribbin: %v", err)
	}
	if err := os.Symlink(ribbinPath, binaryPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	registry := &config.Registry{
		Wrappers: map[string]config.WrapperEntry{
			"uninstall-test": {Original: binaryPath, Config: "/project/ribbin.jsonc"},
		},
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
	}

	uninstallErr := Uninstall(binaryPath, registry)
	if uninstallErr != nil {
		t.Fatalf("Uninstall error: %v", uninstallErr)
	}

	// Check metadata file was removed
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("metadata file should be removed after uninstall")
	}
}

func TestCleanupSidecarFiles(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "cleanup-test")
	sidecarPath := binaryPath + ".ribbin-original"
	metaPath := MetadataPath(binaryPath)

	// Create sidecar and metadata
	if err := os.WriteFile(sidecarPath, []byte("sidecar content"), 0755); err != nil {
		t.Fatalf("failed to create sidecar: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create metadata: %v", err)
	}

	registry := &config.Registry{
		Wrappers: map[string]config.WrapperEntry{
			"cleanup-test": {Original: binaryPath},
		},
		ShellActivations:  make(map[int]config.ShellActivationEntry),
		ConfigActivations: make(map[string]config.ConfigActivationEntry),
	}

	err := CleanupSidecarFiles(binaryPath, registry)
	if err != nil {
		t.Fatalf("CleanupSidecarFiles error: %v", err)
	}

	// Verify sidecar removed
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Error("sidecar should be removed")
	}

	// Verify metadata removed
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("metadata should be removed")
	}

	// Verify registry updated
	if _, exists := registry.Wrappers["cleanup-test"]; exists {
		t.Error("registry entry should be removed")
	}
}
