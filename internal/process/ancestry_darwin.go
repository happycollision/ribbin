//go:build darwin

package process

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// IsDescendantOf checks if the current process is a descendant of targetPID.
// It walks up the process tree from the current PID to PID 1, checking if any
// ancestor matches targetPID.
func IsDescendantOf(targetPID int) (bool, error) {
	currentPID := os.Getpid()

	// Walk up the process tree
	for currentPID > 1 {
		if currentPID == targetPID {
			return true, nil
		}

		parentPID, err := getParentPID(currentPID)
		if err != nil {
			return false, err
		}

		// Check if we've reached the target
		if parentPID == targetPID {
			return true, nil
		}

		// Move up the tree
		currentPID = parentPID
	}

	// Check if target is PID 1 (init/launchd)
	if targetPID == 1 {
		return true, nil
	}

	return false, nil
}

// getParentPID retrieves the parent PID for a given process using ps command.
func getParentPID(pid int) (int, error) {
	cmd := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	ppidStr := strings.TrimSpace(string(output))
	ppid, err := strconv.Atoi(ppidStr)
	if err != nil {
		return 0, err
	}

	return ppid, nil
}

// ProcessExists checks if a process with the given PID exists.
func ProcessExists(pid int) bool {
	// On Darwin, sending signal 0 checks if process exists without affecting it
	err := syscall.Kill(pid, 0)
	return err == nil
}
