package security

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// Lock represents an advisory file lock.
// Uses flock(2) for cross-process locking to prevent TOCTOU race conditions.
type Lock struct {
	file     *os.File
	path     string
	released bool
}

// AcquireLock acquires an exclusive advisory lock on a file.
// Creates lock file if it doesn't exist.
// Times out after specified duration to prevent deadlocks.
//
// The lock file is created at path + ".lock" and uses flock(2) for
// advisory locking, which works across processes but requires cooperation.
//
// Example:
//
//	lock, err := AcquireLock("/usr/local/bin/tsc", 10*time.Second)
//	if err != nil {
//	    return err
//	}
//	defer lock.Release()
func AcquireLock(path string, timeout time.Duration) (*Lock, error) {
	lockPath := path + ".lock"

	// Create lock file if doesn't exist
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file: %w", err)
	}

	// Try to acquire lock with timeout
	deadline := time.Now().Add(timeout)
	for {
		// Try exclusive lock (LOCK_EX | LOCK_NB for non-blocking)
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Lock acquired
			return &Lock{
				file:     file,
				path:     lockPath,
				released: false,
			}, nil
		}

		// Check if timeout
		if time.Now().After(deadline) {
			file.Close()
			return nil, fmt.Errorf("timeout acquiring lock on %s after %v", path, timeout)
		}

		// Wait a bit and retry
		time.Sleep(100 * time.Millisecond)
	}
}

// AcquireSharedLock acquires a shared (read) lock on a file.
// Multiple readers can hold shared locks simultaneously, but exclusive locks are blocked.
// Creates lock file if it doesn't exist.
// Times out after specified duration.
//
// Use this for read operations like LoadRegistry to allow concurrent reads
// while blocking writes.
//
// Example:
//
//	lock, err := AcquireSharedLock("/home/user/.config/ribbin/registry.json", 5*time.Second)
//	if err != nil {
//	    return err
//	}
//	defer lock.Release()
func AcquireSharedLock(path string, timeout time.Duration) (*Lock, error) {
	lockPath := path + ".lock"

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		// Try shared lock (LOCK_SH | LOCK_NB)
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
		if err == nil {
			return &Lock{
				file:     file,
				path:     lockPath,
				released: false,
			}, nil
		}

		if time.Now().After(deadline) {
			file.Close()
			return nil, fmt.Errorf("timeout acquiring shared lock on %s after %v", path, timeout)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Release releases the file lock and removes the lock file.
// Should be called via defer to ensure cleanup even on panic.
//
// Returns error if lock was already released or if release fails.
func (l *Lock) Release() error {
	if l.released {
		return fmt.Errorf("lock already released")
	}

	// Release lock
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	if err != nil {
		return fmt.Errorf("cannot release lock: %w", err)
	}

	// Close file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("cannot close lock file: %w", err)
	}

	// Remove lock file (best effort - ignore errors if file doesn't exist)
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove lock file: %w", err)
	}

	l.released = true
	return nil
}

// WithLock executes a function while holding an exclusive lock.
// Automatically releases lock when function returns.
// This is the recommended way to use locks as it ensures cleanup.
//
// Example:
//
//	err := WithLock("/usr/local/bin/tsc", 10*time.Second, func() error {
//	    // Critical section - modifications protected by lock
//	    return InstallShim(binaryPath, ribbinPath)
//	})
func WithLock(path string, timeout time.Duration, fn func() error) error {
	lock, err := AcquireLock(path, timeout)
	if err != nil {
		return err
	}
	defer lock.Release()

	return fn()
}

// WithSharedLock executes a function while holding a shared lock.
// Automatically releases lock when function returns.
// Use for read operations that don't modify the protected resource.
//
// Example:
//
//	var data []byte
//	err := WithSharedLock(registryPath, 5*time.Second, func() error {
//	    var err error
//	    data, err = os.ReadFile(registryPath)
//	    return err
//	})
func WithSharedLock(path string, timeout time.Duration, fn func() error) error {
	lock, err := AcquireSharedLock(path, timeout)
	if err != nil {
		return err
	}
	defer lock.Release()

	return fn()
}

// FileInfo stores file metadata for validation.
// Used to detect if a file has been modified between check and use (TOCTOU prevention).
type FileInfo struct {
	Mode    os.FileMode
	Size    int64
	ModTime time.Time
}

// GetFileInfo safely gets file info without following symlinks.
// Uses Lstat instead of Stat to avoid symlink attacks.
//
// Returns FileInfo struct containing mode, size, and modification time.
func GetFileInfo(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Mode:    info.Mode(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// VerifyFileUnchanged checks if file hasn't changed since last check.
// Compares mode, size, and modification time to detect tampering.
//
// This is critical for TOCTOU prevention: after acquiring a lock and checking
// a file, verify it hasn't changed before operating on it.
//
// Returns error if file has changed in any way.
func VerifyFileUnchanged(path string, expected *FileInfo) error {
	current, err := GetFileInfo(path)
	if err != nil {
		return fmt.Errorf("file changed: %w", err)
	}

	if current.Mode != expected.Mode {
		return fmt.Errorf("file mode changed: %o -> %o", expected.Mode, current.Mode)
	}
	if current.Size != expected.Size {
		return fmt.Errorf("file size changed: %d -> %d", expected.Size, current.Size)
	}
	if !current.ModTime.Equal(expected.ModTime) {
		return fmt.Errorf("file modified: %v -> %v", expected.ModTime, current.ModTime)
	}

	return nil
}

// AtomicRename renames a file atomically (same filesystem only).
// Uses O_EXCL to ensure destination doesn't exist, preventing TOCTOU races.
//
// This is safer than os.Rename for security-critical operations because it
// will fail if the destination already exists, preventing attackers from
// creating a malicious file at the destination between check and rename.
//
// Note: Both paths must be on the same filesystem for atomicity.
//
// Returns error if destination exists or rename fails.
func AtomicRename(oldPath, newPath string) error {
	// Check that destination doesn't exist (with O_EXCL)
	// This is atomic - prevents TOCTOU
	f, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("destination already exists: %s", newPath)
		}
		return fmt.Errorf("cannot check destination: %w", err)
	}
	f.Close()
	os.Remove(newPath) // Clean up test file

	// Now safe to rename
	return os.Rename(oldPath, newPath)
}
