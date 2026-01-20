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

// HasSidecar checks if a binary has a sidecar file (was shimmed)
func HasSidecar(binaryPath string) bool {
	sidecarPath := binaryPath + ".ribbin-original"
	_, err := os.Stat(sidecarPath)
	return err == nil
}

// Install creates a shim for a binary:
// 1. Acquire lock to prevent TOCTOU races
// 2. Validate paths and check file state (including symlink validation)
// 3. Rename original to {path}.ribbin-original
// 4. Create symlink {path} -> ribbinPath
// 5. Update registry
func Install(binaryPath, ribbinPath string, registry *config.Registry, configPath string) error {
	// Log privileged operations
	if os.Getuid() == 0 {
		security.LogPrivilegedOperation("shim_install", binaryPath, true, nil)
	}

	// Deferred audit logging for install result
	var installErr error
	defer func() {
		security.LogShimInstall(binaryPath, installErr == nil, installErr)
	}()

	// 1. ACQUIRE LOCK FIRST (prevents concurrent modifications)
	lock, err := security.AcquireLock(binaryPath, 10*time.Second)
	if err != nil {
		installErr = fmt.Errorf("cannot acquire lock: %w", err)
		return installErr
	}
	defer lock.Release()

	// 2. VALIDATE PATHS (within lock)
	if err := security.ValidateBinaryPath(binaryPath); err != nil {
		installErr = fmt.Errorf("invalid binary path: %w", err)
		return installErr
	}
	if err := security.ValidateBinaryPath(ribbinPath); err != nil {
		installErr = fmt.Errorf("invalid ribbin path: %w", err)
		return installErr
	}

	// 2a. VALIDATE SYMLINKS (if binary is a symlink)
	info, err := os.Lstat(binaryPath)
	if err != nil && !os.IsNotExist(err) {
		installErr = fmt.Errorf("cannot stat binary: %w", err)
		return installErr
	}
	if info != nil && info.Mode()&os.ModeSymlink != 0 {
		// Binary is a symlink - validate it's safe
		finalTarget, err := security.ValidateSymlinkForShimming(binaryPath)
		if err != nil {
			installErr = err
			return installErr
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
		installErr = err
		return installErr
	}

	// 2b. ENSURE NO SYMLINKS IN SIDECAR PATH (prevent TOCTOU attacks)
	if err := security.NoSymlinksInPath(filepath.Dir(sidecarPath)); err != nil {
		installErr = fmt.Errorf("unsafe parent directory (contains symlinks): %w", err)
		return installErr
	}

	// 3. GET FILE INFO (for later verification)
	binaryInfo, err := security.GetFileInfo(binaryPath)
	if err != nil {
		installErr = fmt.Errorf("cannot stat binary: %w", err)
		return installErr
	}

	// 4. CHECK IF ALREADY SHIMMED (within lock)
	if _, err := os.Lstat(sidecarPath); err == nil {
		installErr = fmt.Errorf("binary %s is already shimmed (sidecar exists at %s)", binaryPath, sidecarPath)
		return installErr
	} else if !os.IsNotExist(err) {
		installErr = fmt.Errorf("failed to check sidecar path %s: %w", sidecarPath, err)
		return installErr
	}

	// 5. VERIFY BINARY UNCHANGED (prevent race)
	if err := security.VerifyFileUnchanged(binaryPath, binaryInfo); err != nil {
		installErr = fmt.Errorf("binary changed during operation: %w", err)
		return installErr
	}

	// 6. ATOMIC RENAME (using O_EXCL)
	if err := security.AtomicRename(binaryPath, sidecarPath); err != nil {
		if os.IsPermission(err) {
			// Provide context-aware error message based on directory category
			if security.IsCriticalSystemBinary(binaryPath) {
				installErr = fmt.Errorf("permission denied: %s\n\nCANNOT shim critical system binary %s for security reasons",
					binaryPath, filepath.Base(binaryPath))
				return installErr
			}

			category, _ := security.GetDirectoryCategory(binaryPath)
			if category == security.CategoryForbidden {
				installErr = fmt.Errorf("permission denied: %s\n\nDirectory %s is protected and cannot be shimmed",
					binaryPath, filepath.Dir(binaryPath))
				return installErr
			}

			cmdName := filepath.Base(binaryPath)
			installErr = fmt.Errorf("permission denied: %s\n\nIf you understand the security implications:\n  sudo ribbin shim %s --confirm-system-dir",
				binaryPath, cmdName)
			return installErr
		}
		installErr = fmt.Errorf("cannot rename binary to sidecar: %w", err)
		return installErr
	}

	// 6a. VERIFY SIDECAR IS NOT A SYMLINK (security check)
	if err := verifySidecarNotSymlink(sidecarPath); err != nil {
		installErr = err
		return installErr
	}

	// 7. CREATE SYMLINK (rollback on failure)
	if err := os.Symlink(ribbinPath, binaryPath); err != nil {
		// ROLLBACK: restore original
		rollbackErr := os.Rename(sidecarPath, binaryPath)
		if rollbackErr != nil {
			installErr = fmt.Errorf("cannot create symlink (and rollback failed: %v): %w", rollbackErr, err)
			return installErr
		}
		if os.IsPermission(err) {
			installErr = fmt.Errorf("permission denied: cannot create symlink at %s (try with sudo)", binaryPath)
			return installErr
		}
		installErr = fmt.Errorf("failed to create symlink at %s: %w", binaryPath, err)
		return installErr
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
	// Log privileged operations
	if os.Getuid() == 0 {
		security.LogPrivilegedOperation("shim_uninstall", binaryPath, true, nil)
	}

	// Deferred audit logging for uninstall result
	var uninstallErr error
	defer func() {
		security.LogShimUninstall(binaryPath, uninstallErr == nil, uninstallErr)
	}()

	// ACQUIRE LOCK
	lock, err := security.AcquireLock(binaryPath, 10*time.Second)
	if err != nil {
		uninstallErr = fmt.Errorf("cannot acquire lock: %w", err)
		return uninstallErr
	}
	defer lock.Release()

	// Validate binary path
	if err := security.ValidateBinaryPath(binaryPath); err != nil {
		uninstallErr = fmt.Errorf("invalid binary path: %w", err)
		return uninstallErr
	}

	sidecarPath, err := SidecarPath(binaryPath)
	if err != nil {
		uninstallErr = err
		return uninstallErr
	}

	// Verify it's a shim (check symlink)
	info, err := os.Lstat(binaryPath)
	if err != nil {
		uninstallErr = fmt.Errorf("cannot stat binary: %w", err)
		return uninstallErr
	}
	if info.Mode()&os.ModeSymlink == 0 {
		uninstallErr = fmt.Errorf("%s is not a shim (not a symlink)", binaryPath)
		return uninstallErr
	}

	// Verify sidecar exists
	if _, err := os.Stat(sidecarPath); err != nil {
		uninstallErr = fmt.Errorf("sidecar not found: %s", sidecarPath)
		return uninstallErr
	}

	// Remove symlink
	if err := os.Remove(binaryPath); err != nil {
		if os.IsPermission(err) {
			uninstallErr = fmt.Errorf("permission denied: cannot remove symlink at %s (try with sudo)", binaryPath)
			return uninstallErr
		}
		uninstallErr = fmt.Errorf("cannot remove symlink: %w", err)
		return uninstallErr
	}

	// ATOMIC RENAME sidecar back to original
	if err := security.AtomicRename(sidecarPath, binaryPath); err != nil {
		if os.IsPermission(err) {
			uninstallErr = fmt.Errorf("permission denied: cannot restore original at %s (try with sudo)", binaryPath)
			return uninstallErr
		}
		uninstallErr = fmt.Errorf("cannot restore original binary: %w", err)
		return uninstallErr
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
