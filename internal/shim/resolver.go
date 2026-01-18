package shim

import (
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveCommand finds the path to a command using exec.LookPath.
// Returns the absolute path to the command or an error if not found.
func ResolveCommand(name string) (string, error) {
	return exec.LookPath(name)
}

// ResolveCommands resolves multiple command names to their paths.
// Returns a map of command name to path. Commands that cannot be
// resolved are omitted from the result.
func ResolveCommands(names []string) map[string]string {
	result := make(map[string]string)
	for _, name := range names {
		path, err := ResolveCommand(name)
		if err == nil {
			result[name] = path
		}
	}
	return result
}

// IsAlreadyShimmed checks if the binary at the given path is a symlink
// pointing to ribbin. Returns true if the binary is already shimmed.
func IsAlreadyShimmed(path string) (bool, error) {
	// Check if path is a symlink using os.Lstat
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}

	// Read the symlink target using os.Readlink (not SafeReadlink)
	// We use os.Readlink here because we just need the direct target,
	// not a validated chain. This is a simple check, not a security operation.
	target, err := os.Readlink(path)
	if err != nil {
		return false, err
	}

	// Check if the target basename is "ribbin"
	return filepath.Base(target) == "ribbin", nil
}
