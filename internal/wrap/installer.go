package wrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/happycollision/ribbin/internal/config"
	"github.com/happycollision/ribbin/internal/security"
)

// Version can be set by the CLI package at startup to include in metadata
var Version = "dev"

// WrapperMetadata tracks information about a wrapped binary for stale detection
type WrapperMetadata struct {
	WrappedAt     time.Time `json:"wrapped_at"`
	OriginalHash  string    `json:"original_hash"` // sha256:abc123...
	OriginalSize  int64     `json:"original_size"`
	RibbinPath    string    `json:"ribbin_path"`
	RibbinVersion string    `json:"ribbin_version"`
}

// MetadataPath returns the metadata file path for a binary
func MetadataPath(binaryPath string) string {
	return binaryPath + ".ribbin-meta"
}

// HasMetadata checks if a binary has a metadata file
func HasMetadata(binaryPath string) bool {
	_, err := os.Stat(MetadataPath(binaryPath))
	return err == nil
}

// hashFile calculates the SHA256 hash of a file
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile copies a file from src to dst, preserving permissions
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// LoadMetadata reads metadata from a .ribbin-meta file
func LoadMetadata(binaryPath string) (*WrapperMetadata, error) {
	metaPath := MetadataPath(binaryPath)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta WrapperMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// saveMetadata writes metadata to a .ribbin-meta file
func saveMetadata(binaryPath string, meta *WrapperMetadata) error {
	metaPath := MetadataPath(binaryPath)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// removeMetadata removes the metadata file for a binary
func removeMetadata(binaryPath string) error {
	metaPath := MetadataPath(binaryPath)
	err := os.Remove(metaPath)
	if os.IsNotExist(err) {
		return nil // No metadata file to remove is fine
	}
	return err
}

// ConflictResolution represents how a hash mismatch was resolved
type ConflictResolution int

const (
	ResolutionNone     ConflictResolution = iota // No conflict
	ResolutionSkipped                            // User chose to skip (do nothing)
	ResolutionCleanup                            // User chose to remove sidecar files, keep current binary
	ResolutionRestored                           // User chose to restore original from sidecar
)

// UnwrapResult tracks the result of unwrapping a single binary
type UnwrapResult struct {
	BinaryPath string
	Success    bool
	Error      error
	Conflict   bool
	Resolution ConflictResolution
}

// CheckHashConflict checks if the sidecar hash differs from what was recorded at wrap time.
// Returns true if there's a conflict, false if no conflict or no metadata.
func CheckHashConflict(binaryPath string) (hasConflict bool, currentHash string, originalHash string) {
	sidecarPath := binaryPath + ".ribbin-original"

	// Load metadata
	meta, err := LoadMetadata(binaryPath)
	if err != nil {
		// No metadata or can't read it - assume no conflict
		return false, "", ""
	}

	// Calculate current hash of sidecar
	currentHash, err = hashFile(sidecarPath)
	if err != nil {
		// Can't hash sidecar - assume no conflict
		return false, "", meta.OriginalHash
	}

	// Compare hashes
	if currentHash != meta.OriginalHash {
		return true, currentHash, meta.OriginalHash
	}

	return false, currentHash, meta.OriginalHash
}

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
	var finalTarget string // If binary is a symlink, track the final target for dual-sidecar creation
	info, err := os.Lstat(binaryPath)
	if err != nil && !os.IsNotExist(err) {
		installErr = fmt.Errorf("cannot stat binary: %w", err)
		return installErr
	}
	if info != nil && info.Mode()&os.ModeSymlink != 0 {
		// Binary is a symlink - validate it's safe
		var validateErr error
		finalTarget, validateErr = security.ValidateSymlinkForShimming(binaryPath)
		if validateErr != nil {
			installErr = validateErr
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
			fmt.Fprintf(os.Stderr, "   Creating sidecars at symlink and target for robustness\n")
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

	// 7a. CREATE METADATA FILE (best effort - don't fail if this fails)
	hash, hashErr := hashFile(sidecarPath)
	if hashErr == nil {
		sidecarInfo, statErr := os.Stat(sidecarPath)
		if statErr == nil {
			meta := &WrapperMetadata{
				WrappedAt:     time.Now(),
				OriginalHash:  hash,
				OriginalSize:  sidecarInfo.Size(),
				RibbinPath:    ribbinPath,
				RibbinVersion: Version,
			}
			// Best effort - don't fail installation if metadata write fails
			_ = saveMetadata(binaryPath, meta)
		}
	}

	// 7b. CREATE SECOND SIDECAR AT FINAL TARGET (if binary was a symlink)
	if finalTarget != "" {
		// Create a copy of the sidecar at the final target location
		targetSidecarPath := finalTarget + ".ribbin-original"

		// Only create if it doesn't already exist
		if _, err := os.Stat(targetSidecarPath); os.IsNotExist(err) {
			// Copy the sidecar content to the target location
			if copyErr := copyFile(sidecarPath, targetSidecarPath); copyErr == nil {
				fmt.Fprintf(os.Stderr, "   Created sidecar at target: %s\n", targetSidecarPath)
			}
			// Best effort - don't fail if this fails
		}
	}

	// 8. UPDATE REGISTRY (within lock)
	commandName := filepath.Base(binaryPath)
	registry.Wrappers[commandName] = config.WrapperEntry{
		Original: binaryPath,
		Config:   configPath,
	}

	// Lock automatically released by defer
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

	// Clean up metadata file (best effort)
	_ = removeMetadata(binaryPath)

	// Update registry
	commandName := filepath.Base(binaryPath)
	delete(registry.Wrappers, commandName)

	return nil
}

// CleanupSidecarFiles removes sidecar and metadata files without restoring the original.
// Used when the user chooses to keep the current binary during conflict resolution.
func CleanupSidecarFiles(binaryPath string, registry *config.Registry) error {
	sidecarPath := binaryPath + ".ribbin-original"

	// Log cleanup operation for audit trail
	security.LogPrivilegedOperation("cleanup_sidecar", binaryPath, true, nil)

	// Remove sidecar file
	if err := os.Remove(sidecarPath); err != nil && !os.IsNotExist(err) {
		security.LogPrivilegedOperation("cleanup_sidecar", binaryPath, false, err)
		return fmt.Errorf("cannot remove sidecar: %w", err)
	}

	// Remove metadata file
	_ = removeMetadata(binaryPath)

	// Update registry
	commandName := filepath.Base(binaryPath)
	delete(registry.Wrappers, commandName)

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
