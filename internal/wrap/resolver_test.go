package wrap

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestResolveCommand(t *testing.T) {
	t.Run("finds common commands", func(t *testing.T) {
		// sh should exist on all Unix systems
		path, err := ResolveCommand("sh")
		if err != nil {
			t.Fatalf("ResolveCommand('sh') error: %v", err)
		}
		if path == "" {
			t.Error("expected non-empty path for sh")
		}
	})

	t.Run("returns error for non-existent command", func(t *testing.T) {
		_, err := ResolveCommand("nonexistent-command-xyz123")
		if err == nil {
			t.Error("expected error for non-existent command")
		}
	})
}

func TestResolveCommands(t *testing.T) {
	t.Run("resolves multiple commands", func(t *testing.T) {
		commands := []string{"sh", "ls", "nonexistent-xyz"}
		result := ResolveCommands(commands)

		// sh and ls should be resolved
		if _, exists := result["sh"]; !exists {
			t.Error("sh should be resolved")
		}
		if _, exists := result["ls"]; !exists {
			t.Error("ls should be resolved")
		}

		// nonexistent should be omitted
		if _, exists := result["nonexistent-xyz"]; exists {
			t.Error("nonexistent command should be omitted")
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := ResolveCommands([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}

func TestIsAlreadyShimmed(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("returns false for regular file", func(t *testing.T) {
		regularFile := filepath.Join(tmpDir, "regular")
		if err := os.WriteFile(regularFile, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		isShimmed, err := IsAlreadyShimmed(regularFile)
		if err != nil {
			t.Fatalf("IsAlreadyShimmed error: %v", err)
		}
		if isShimmed {
			t.Error("regular file should not be shimmed")
		}
	})

	t.Run("returns true for symlink to ribbin", func(t *testing.T) {
		// Create a fake ribbin binary
		ribbinPath := filepath.Join(tmpDir, "ribbin")
		if err := os.WriteFile(ribbinPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create ribbin: %v", err)
		}

		// Create symlink pointing to ribbin
		symlinkPath := filepath.Join(tmpDir, "shimmed-cmd")
		if err := os.Symlink(ribbinPath, symlinkPath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		isShimmed, err := IsAlreadyShimmed(symlinkPath)
		if err != nil {
			t.Fatalf("IsAlreadyShimmed error: %v", err)
		}
		if !isShimmed {
			t.Error("symlink to ribbin should be shimmed")
		}
	})

	t.Run("returns false for symlink to other binary", func(t *testing.T) {
		// Create a non-ribbin binary
		otherPath := filepath.Join(tmpDir, "other-binary")
		if err := os.WriteFile(otherPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create other binary: %v", err)
		}

		// Create symlink pointing to other binary
		symlinkPath := filepath.Join(tmpDir, "other-link")
		if err := os.Symlink(otherPath, symlinkPath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		isShimmed, err := IsAlreadyShimmed(symlinkPath)
		if err != nil {
			t.Fatalf("IsAlreadyShimmed error: %v", err)
		}
		if isShimmed {
			t.Error("symlink to other binary should not be shimmed")
		}
	})

	t.Run("returns error for non-existent path", func(t *testing.T) {
		_, err := IsAlreadyShimmed(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})
}
