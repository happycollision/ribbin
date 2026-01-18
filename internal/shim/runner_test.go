package shim

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/happycollision/ribbin/internal/config"
)

func TestExtractCommandName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/usr/bin/cat", "cat"},
		{"/usr/local/bin/node", "node"},
		{"cat", "cat"},
		{"/a/b/c/d/program", "program"},
		{"./relative/path/cmd", "cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractCommandName(tt.path)
			if result != tt.expected {
				t.Errorf("extractCommandName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	t.Run("returns true when GlobalOn is true", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    true,
		}

		if !isActive(registry) {
			t.Error("should be active when GlobalOn is true")
		}
	})

	t.Run("returns false when GlobalOn is false and no activations", func(t *testing.T) {
		registry := &config.Registry{
			Shims:       make(map[string]config.ShimEntry),
			Activations: make(map[int]config.ActivationEntry),
			GlobalOn:    false,
		}

		if isActive(registry) {
			t.Error("should not be active when GlobalOn is false and no activations")
		}
	})

	t.Run("returns true when ancestor PID is in activations", func(t *testing.T) {
		// PID 1 is always an ancestor (init/launchd)
		registry := &config.Registry{
			Shims: make(map[string]config.ShimEntry),
			Activations: map[int]config.ActivationEntry{
				1: {PID: 1, ActivatedAt: time.Now()},
			},
			GlobalOn: false,
		}

		if !isActive(registry) {
			t.Error("should be active when PID 1 is in activations")
		}
	})

	t.Run("returns false when non-ancestor PID is in activations", func(t *testing.T) {
		// Use a high PID that's unlikely to be an ancestor
		registry := &config.Registry{
			Shims: make(map[string]config.ShimEntry),
			Activations: map[int]config.ActivationEntry{
				99999999: {PID: 99999999, ActivatedAt: time.Now()},
			},
			GlobalOn: false,
		}

		if isActive(registry) {
			t.Error("should not be active when only non-ancestor PIDs in activations")
		}
	})
}

// Note: Run() uses syscall.Exec which replaces the current process,
// making it difficult to test directly. We test the helper functions
// and integration tests cover the full flow.

func TestRunWithMissingOriginal(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Call Run with a path that has no sidecar
	binaryPath := filepath.Join(tmpDir, "missing")
	err = Run(binaryPath, []string{})
	if err == nil {
		t.Error("expected error when original binary is missing")
	}
}

func TestRunWithBypassEnv(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create sidecar (we can't fully test this without exec, but we can check setup)
	binaryPath := filepath.Join(tmpDir, "test-cmd")
	sidecarPath := binaryPath + ".ribbin-original"

	// Create a valid sidecar binary
	if err := os.WriteFile(sidecarPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create sidecar: %v", err)
	}

	// Set bypass env
	os.Setenv("RIBBIN_BYPASS", "1")
	defer os.Unsetenv("RIBBIN_BYPASS")

	// Run would exec the original - we can't test this fully without forking
	// but we verify the sidecar exists which is the prerequisite
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		t.Error("sidecar should exist for bypass test setup")
	}
}

func TestPrintBlockMessage(t *testing.T) {
	// Capture stderr output is tricky, just verify it doesn't panic
	t.Run("prints with custom message", func(t *testing.T) {
		// Redirect stderr temporarily
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printBlockMessage("cat", "Use bat instead for syntax highlighting")

		w.Close()
		os.Stderr = oldStderr

		// Read output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if len(output) == 0 {
			t.Error("expected output to stderr")
		}
	})

	t.Run("prints with default message", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		printBlockMessage("npm", "")

		w.Close()
		os.Stderr = oldStderr

		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if len(output) == 0 {
			t.Error("expected output to stderr")
		}
	})
}
