package shim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/security"
)

// SidecarPath returns the sidecar path for a binary
func SidecarPath(binaryPath string) (string, error) {
	// Validate binary path first
	if err := security.ValidateBinaryPath(binaryPath); err != nil {
		return "", fmt.Errorf("invalid binary path: %w", err)
	}
	return binaryPath + ".ribbin-original", nil
}

// Install creates a shim for a binary:
// 1. Acquire lock to prevent TOCTOU races
// 2. Validate paths and check file state (including symlink validation)
// 3. Rename original to {path}.ribbin-original
// 4. Create symlink {path} -> ribbinPath
// 5. Update registry
func Install(binaryPath, ribbinPath string, registry *config.Registry, configPath string) error {
	// 1. ACQUIRE LOCK FIRST (prevents concurrent modifications)
	lock, err := security.AcquireLock(binaryPath, 10*time.Second)
	if err != nil {
		return fmt.Errorf("cannot acquire lock: %w", err)
	}
	defer lock.Release()

	// 2. VALIDATE PATHS (within lock)
	if err := security.ValidateBinaryPath(binaryPath); err != nil {
		return fmt.Errorf("invalid binary path: %w", err)
	}
	if err := security.ValidateBinaryPath(ribbinPath); err != nil {
		return fmt.Errorf("invalid ribbin path: %w", err)
	}

	// 2a. VALIDATE SYMLINKS (if binary is a symlink)
	info, err := os.Lstat(binaryPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot stat binary: %w", err)
	}
	if info != nil && info.Mode()&os.ModeSymlink != 0 {
		// Binary is a symlink - validate it's safe
		finalTarget, err := security.ValidateSymlinkForShimming(binaryPath)
		if err != nil {
			return err
		}

		// Get symlink info for user warning
		symlinkInfo, infoErr := security.GetSymlinkInfo(binaryPath)
		if infoErr == nil && symlinkInfo.ChainDepth > 0 {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: %s is a symlink ", filepath.Base(binaryPath))
			if symlinkInfo.ChainDepth > 1 {
				fmt.Fprintf(os.Stderr, "(chain depth %d) ", symlinkInfo.ChainDepth)
			}
			fmt.Fprintf(os.Stderr, "-> %s\n", finalTarget)
			fmt.Fprintf(os.Stderr, "   The shim will redirect to the symlink, not the final target\n")
		}
	}

	sidecarPath, err := SidecarPath(binaryPath)
	if err != nil {
		return err
	}

	// 2b. ENSURE NO SYMLINKS IN SIDECAR PATH (prevent TOCTOU attacks)
	if err := security.NoSymlinksInPath(filepath.Dir(sidecarPath)); err != nil {
		return fmt.Errorf("unsafe parent directory (contains symlinks): %w", err)
	}

	// 3. GET FILE INFO (for later verification)
	binaryInfo, err := security.GetFileInfo(binaryPath)
	if err != nil {
		return fmt.Errorf("cannot stat binary: %w", err)
	}

	// 4. CHECK IF ALREADY SHIMMED (within lock)
	if _, err := os.Lstat(sidecarPath); err == nil {
		return fmt.Errorf("binary %s is already shimmed (sidecar exists at %s)", binaryPath, sidecarPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check sidecar path %s: %w", sidecarPath, err)
	}

	// 5. VERIFY BINARY UNCHANGED (prevent race)
	if err := security.VerifyFileUnchanged(binaryPath, binaryInfo); err != nil {
		return fmt.Errorf("binary changed during operation: %w", err)
	}

	// 6. ATOMIC RENAME (using O_EXCL)
	if err := security.AtomicRename(binaryPath, sidecarPath); err != nil {
		if os.IsPermission(err) {
			// Provide context-aware error message based on directory category
			if security.IsCriticalSystemBinary(binaryPath) {
				return fmt.Errorf("permission denied: %s\n\nCANNOT shim critical system binary %s for security reasons",
					binaryPath, filepath.Base(binaryPath))
			}

			category, _ := security.GetDirectoryCategory(binaryPath)
			if category == security.CategoryForbidden {
				return fmt.Errorf("permission denied: %s\n\nDirectory %s is protected and cannot be shimmed",
					binaryPath, filepath.Dir(binaryPath))
			}

			cmdName := filepath.Base(binaryPath)
			return fmt.Errorf("permission denied: %s\n\nIf you understand the security implications:\n  sudo ribbin shim %s --confirm-system-dir",
				binaryPath, cmdName)
		}
		return fmt.Errorf("cannot rename binary to sidecar: %w", err)
	}

	// 6a. VERIFY SIDECAR IS NOT A SYMLINK (security check)
	if err := verifySidecarNotSymlink(sidecarPath); err != nil {
		return err
	}

	// 7. CREATE SYMLINK (rollback on failure)
	if err := os.Symlink(ribbinPath, binaryPath); err != nil {
		// ROLLBACK: restore original
		rollbackErr := os.Rename(sidecarPath, binaryPath)
		if rollbackErr != nil {
			return fmt.Errorf("cannot create symlink (and rollback failed: %v): %w", rollbackErr, err)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot create symlink at %s (try with sudo)", binaryPath)
		}
		return fmt.Errorf("failed to create symlink at %s: %w", binaryPath, err)
	}

	// 8. UPDATE REGISTRY (within lock)
	commandName := filepath.Base(binaryPath)
	registry.Shims[commandName] = config.ShimEntry{
		Original: binaryPath,
		Config:   configPath,
	}

	// Lock automatically released by defer
	return nil
}

// verifySidecarNotSymlink ensures the sidecar file is not a symlink.
// This prevents attacks where an attacker creates a malicious symlink
// in place of the expected sidecar file.
func verifySidecarNotSymlink(sidecarPath string) error {
	info, err := os.Lstat(sidecarPath)
	if err != nil {
		return fmt.Errorf("cannot verify sidecar: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("security violation: sidecar is a symlink: %s", sidecarPath)
	}
	return nil
}

// Uninstall removes a shim:
// 1. Acquire lock to prevent concurrent operations
// 2. Remove symlink at {path}
// 3. Rename {path}.ribbin-original back to {path}
// 4. Remove from registry
func Uninstall(binaryPath string, registry *config.Registry) error {
	// ACQUIRE LOCK
	lock, err := security.AcquireLock(binaryPath, 10*time.Second)
	if err != nil {
		return fmt.Errorf("cannot acquire lock: %w", err)
	}
	defer lock.Release()

	// Validate binary path
	if err := security.ValidateBinaryPath(binaryPath); err != nil {
		return fmt.Errorf("invalid binary path: %w", err)
	}

	sidecarPath, err := SidecarPath(binaryPath)
	if err != nil {
		return err
	}

	// Verify it's a shim (check symlink)
	info, err := os.Lstat(binaryPath)
	if err != nil {
		return fmt.Errorf("cannot stat binary: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s is not a shim (not a symlink)", binaryPath)
	}

	// Verify sidecar exists
	if _, err := os.Stat(sidecarPath); err != nil {
		return fmt.Errorf("sidecar not found: %s", sidecarPath)
	}

	// Remove symlink
	if err := os.Remove(binaryPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot remove symlink at %s (try with sudo)", binaryPath)
		}
		return fmt.Errorf("cannot remove symlink: %w", err)
	}

	// ATOMIC RENAME sidecar back to original
	if err := security.AtomicRename(sidecarPath, binaryPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot restore original at %s (try with sudo)", binaryPath)
		}
		return fmt.Errorf("cannot restore original binary: %w", err)
	}

	// Update registry
	commandName := filepath.Base(binaryPath)
	delete(registry.Shims, commandName)

	return nil
}

// FindSidecars searches directories for .ribbin-original files
func FindSidecars(searchPaths []string) ([]string, error) {
	var sidecars []string
	var errs []error

	for _, searchPath := range searchPaths {
		// Check if the search path exists and is accessible
		info, err := os.Stat(searchPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Skip non-existent paths
				continue
			}
			if os.IsPermission(err) {
				errs = append(errs, fmt.Errorf("permission denied: cannot access %s", searchPath))
				continue
			}
			errs = append(errs, fmt.Errorf("failed to access %s: %w", searchPath, err))
			continue
		}

		if !info.IsDir() {
			// Skip non-directories
			continue
		}

		pattern := filepath.Join(searchPath, "*.ribbin-original")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to glob pattern %s: %w", pattern, err))
			continue
		}

		sidecars = append(sidecars, matches...)
	}

	if len(errs) > 0 && len(sidecars) == 0 {
		return nil, errors.Join(errs...)
	}

	return sidecars, nil
}
