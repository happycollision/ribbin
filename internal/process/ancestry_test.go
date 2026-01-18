//go:build linux || darwin

package process

import (
	"os"
	"testing"
)

func TestIsDescendantOf(t *testing.T) {
	t.Run("returns true for self PID", func(t *testing.T) {
		selfPID := os.Getpid()
		isDescendant, err := IsDescendantOf(selfPID)
		if err != nil {
			t.Fatalf("IsDescendantOf error: %v", err)
		}
		if !isDescendant {
			t.Error("process should be descendant of itself")
		}
	})

	t.Run("returns true for PID 1", func(t *testing.T) {
		// All processes are descendants of PID 1 (init/launchd)
		isDescendant, err := IsDescendantOf(1)
		if err != nil {
			t.Fatalf("IsDescendantOf error: %v", err)
		}
		if !isDescendant {
			t.Error("process should be descendant of PID 1")
		}
	})

	t.Run("returns true for parent PID", func(t *testing.T) {
		parentPID := os.Getppid()
		isDescendant, err := IsDescendantOf(parentPID)
		if err != nil {
			t.Fatalf("IsDescendantOf error: %v", err)
		}
		if !isDescendant {
			t.Error("process should be descendant of its parent")
		}
	})

	t.Run("returns false for unrelated PID", func(t *testing.T) {
		// Use a very high PID that's unlikely to be an ancestor
		// (but might not exist, which is also fine)
		isDescendant, err := IsDescendantOf(99999999)
		// We expect either false with no error, or an error
		if err == nil && isDescendant {
			t.Error("process should not be descendant of random high PID")
		}
	})
}

func TestProcessExists(t *testing.T) {
	t.Run("returns true for current process", func(t *testing.T) {
		if !ProcessExists(os.Getpid()) {
			t.Error("current process should exist")
		}
	})

	t.Run("returns true for PID 1", func(t *testing.T) {
		// PID 1 is always the init process (on Linux) or launchd (on macOS)
		if !ProcessExists(1) {
			t.Error("PID 1 should exist")
		}
	})

	t.Run("returns false for non-existent PID", func(t *testing.T) {
		// Use a very high PID that's extremely unlikely to exist
		if ProcessExists(99999999) {
			t.Error("PID 99999999 should not exist")
		}
	})

	t.Run("returns true for parent PID", func(t *testing.T) {
		ppid := os.Getppid()
		if !ProcessExists(ppid) {
			t.Error("parent process should exist")
		}
	})
}

func TestGetParentPID(t *testing.T) {
	t.Run("gets parent of current process", func(t *testing.T) {
		ppid, err := getParentPID(os.Getpid())
		if err != nil {
			t.Fatalf("getParentPID error: %v", err)
		}
		expected := os.Getppid()
		if ppid != expected {
			t.Errorf("expected parent PID %d, got %d", expected, ppid)
		}
	})

	t.Run("gets parent of PID 1", func(t *testing.T) {
		ppid, err := getParentPID(1)
		if err != nil {
			t.Fatalf("getParentPID error: %v", err)
		}
		// PID 1's parent is 0 (no parent)
		if ppid != 0 {
			t.Errorf("expected PID 1's parent to be 0, got %d", ppid)
		}
	})

	t.Run("returns error for non-existent PID", func(t *testing.T) {
		_, err := getParentPID(99999999)
		if err == nil {
			t.Error("expected error for non-existent PID")
		}
	})
}
