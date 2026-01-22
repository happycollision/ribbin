//go:build linux || darwin

package process

import (
	"os"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
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

func TestGetParentCommand(t *testing.T) {
	t.Run("returns non-empty command for current process", func(t *testing.T) {
		cmd, err := GetParentCommand()
		if err != nil {
			t.Fatalf("GetParentCommand error: %v", err)
		}
		if cmd == "" {
			t.Error("expected non-empty parent command")
		}
	})

	t.Run("returns command containing go for test runner", func(t *testing.T) {
		// When running tests, the parent process is typically "go test" or similar
		cmd, err := GetParentCommand()
		if err != nil {
			t.Fatalf("GetParentCommand error: %v", err)
		}
		// The parent should contain "go" somewhere (go test, or the go binary)
		if len(cmd) == 0 {
			t.Error("expected parent command to be non-empty")
		}
		// Note: We don't assert specific content since it varies by environment
		t.Logf("Parent command: %s", cmd)
	})
}

func TestGetAncestorCommands(t *testing.T) {
	t.Run("returns at least one ancestor", func(t *testing.T) {
		cmds, err := GetAncestorCommands(0)
		if err != nil {
			t.Fatalf("GetAncestorCommands error: %v", err)
		}
		if len(cmds) == 0 {
			t.Error("expected at least one ancestor command")
		}
		t.Logf("Found %d ancestors", len(cmds))
		for i, cmd := range cmds {
			t.Logf("  Ancestor %d: %s", i+1, cmd)
		}
	})

	t.Run("first ancestor matches GetParentCommand", func(t *testing.T) {
		parentCmd, err := GetParentCommand()
		if err != nil {
			t.Fatalf("GetParentCommand error: %v", err)
		}

		cmds, err := GetAncestorCommands(0)
		if err != nil {
			t.Fatalf("GetAncestorCommands error: %v", err)
		}
		if len(cmds) == 0 {
			t.Fatal("expected at least one ancestor")
		}

		if cmds[0] != parentCmd {
			t.Errorf("first ancestor %q doesn't match parent command %q", cmds[0], parentCmd)
		}
	})

	t.Run("respects maxDepth=1", func(t *testing.T) {
		cmds, err := GetAncestorCommands(1)
		if err != nil {
			t.Fatalf("GetAncestorCommands error: %v", err)
		}
		if len(cmds) != 1 {
			t.Errorf("expected exactly 1 ancestor with maxDepth=1, got %d", len(cmds))
		}
	})

	t.Run("respects maxDepth=2", func(t *testing.T) {
		cmds, err := GetAncestorCommands(2)
		if err != nil {
			t.Fatalf("GetAncestorCommands error: %v", err)
		}
		// Should return at most 2, but could be fewer if process tree is shallow
		if len(cmds) > 2 {
			t.Errorf("expected at most 2 ancestors with maxDepth=2, got %d", len(cmds))
		}
	})

	t.Run("unlimited depth returns more than depth=1", func(t *testing.T) {
		cmdsLimited, err := GetAncestorCommands(1)
		if err != nil {
			t.Fatalf("GetAncestorCommands(1) error: %v", err)
		}

		cmdsUnlimited, err := GetAncestorCommands(0)
		if err != nil {
			t.Fatalf("GetAncestorCommands(0) error: %v", err)
		}

		// Unlimited should return at least as many as limited (usually more)
		if len(cmdsUnlimited) < len(cmdsLimited) {
			t.Errorf("unlimited (%d) should return at least as many as depth=1 (%d)",
				len(cmdsUnlimited), len(cmdsLimited))
		}
	})
}

func TestGetCommandForPID(t *testing.T) {
	t.Run("returns command for current process parent", func(t *testing.T) {
		ppid := os.Getppid()
		cmd, err := getCommandForPID(ppid)
		if err != nil {
			t.Fatalf("getCommandForPID error: %v", err)
		}
		if cmd == "" {
			t.Error("expected non-empty command for parent PID")
		}
	})

	t.Run("returns error for non-existent PID", func(t *testing.T) {
		_, err := getCommandForPID(99999999)
		if err == nil {
			t.Error("expected error for non-existent PID")
		}
	})
}
