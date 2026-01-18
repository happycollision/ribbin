package shim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dondenton/ribbin/internal/config"
)

// SidecarPath returns the sidecar path for a binary
func SidecarPath(binaryPath string) string {
	return binaryPath + ".ribbin-original"
}

// Install creates a shim for a binary:
// 1. Rename original to {path}.ribbin-original
// 2. Create symlink {path} -> ribbinPath
// 3. Update registry
func Install(binaryPath, ribbinPath string, registry *config.Registry, configPath string) error {
	sidecarPath := SidecarPath(binaryPath)

	// Check if already shimmed (sidecar exists)
	if _, err := os.Stat(sidecarPath); err == nil {
		return fmt.Errorf("binary %s is already shimmed (sidecar exists at %s)", binaryPath, sidecarPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check sidecar path %s: %w", sidecarPath, err)
	}

	// Step 1: Rename original to sidecar
	if err := os.Rename(binaryPath, sidecarPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot rename %s (try with sudo)", binaryPath)
		}
		return fmt.Errorf("failed to rename %s to %s: %w", binaryPath, sidecarPath, err)
	}

	// Step 2: Create symlink
	if err := os.Symlink(ribbinPath, binaryPath); err != nil {
		// Rollback: restore original
		rollbackErr := os.Rename(sidecarPath, binaryPath)
		if rollbackErr != nil {
			return fmt.Errorf("failed to create symlink and rollback failed: symlink error: %w, rollback error: %v", err, rollbackErr)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot create symlink at %s (try with sudo)", binaryPath)
		}
		return fmt.Errorf("failed to create symlink at %s: %w", binaryPath, err)
	}

	// Step 3: Update registry
	commandName := filepath.Base(binaryPath)
	registry.Shims[commandName] = config.ShimEntry{
		Original: binaryPath,
		Config:   configPath,
	}

	return nil
}

// Uninstall removes a shim:
// 1. Remove symlink at {path}
// 2. Rename {path}.ribbin-original back to {path}
// 3. Remove from registry
func Uninstall(binaryPath string, registry *config.Registry) error {
	sidecarPath := SidecarPath(binaryPath)

	// Verify sidecar exists before proceeding
	if _, err := os.Stat(sidecarPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cannot uninstall: sidecar not found at %s (binary may not be shimmed)", sidecarPath)
		}
		return fmt.Errorf("failed to check sidecar path %s: %w", sidecarPath, err)
	}

	// Step 1: Remove symlink at path
	if err := os.Remove(binaryPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot remove symlink at %s (try with sudo)", binaryPath)
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove symlink at %s: %w", binaryPath, err)
		}
		// If symlink doesn't exist, continue anyway to restore the sidecar
	}

	// Step 2: Rename sidecar back to original path
	if err := os.Rename(sidecarPath, binaryPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot restore original at %s (try with sudo)", binaryPath)
		}
		return fmt.Errorf("failed to restore original from %s to %s: %w", sidecarPath, binaryPath, err)
	}

	// Step 3: Remove from registry
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
