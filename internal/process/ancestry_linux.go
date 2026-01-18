//go:build linux

package process

import (
	"os"
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

	// Check if target is PID 1 (init/systemd)
	if targetPID == 1 {
		return true, nil
	}

	return false, nil
}

// getParentPID retrieves the parent PID for a given process using /proc filesystem.
func getParentPID(pid int) (int, error) {
	// Read /proc/<pid>/stat which contains process info
	// Format: pid (comm) state ppid ...
	statPath := "/proc/" + strconv.Itoa(pid) + "/stat"
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}

	// Find the closing parenthesis of comm field (process name can contain spaces/parens)
	statStr := string(data)
	lastParen := strings.LastIndex(statStr, ")")
	if lastParen == -1 {
		return 0, os.ErrInvalid
	}

	// Fields after comm: state ppid ...
	fields := strings.Fields(statStr[lastParen+1:])
	if len(fields) < 2 {
		return 0, os.ErrInvalid
	}

	// ppid is the second field after the closing paren (index 1, since state is index 0)
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, err
	}

	return ppid, nil
}

// ProcessExists checks if a process with the given PID exists.
func ProcessExists(pid int) bool {
	// On Linux, sending signal 0 checks if process exists without affecting it
	err := syscall.Kill(pid, 0)
	return err == nil
}
