package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DirectoryCategory defines security levels for directories
type DirectoryCategory int

const (
	CategoryAllowed DirectoryCategory = iota // Safe to shim
	CategoryRequiresConfirmation              // System dirs, need confirmation
	CategoryForbidden                         // Critical dirs, never allow
)

// SecurityConfig defines security rules for shimming.
// Uses a blacklist model: everything is allowed except known system directories.
type SecurityConfig struct {
	// SystemDirs are directories that require --confirm-system-dir flag.
	// These are directories that affect system-wide behavior.
	SystemDirs []string

	// CriticalBinaries are specific binaries that must never be shimmed
	CriticalBinaries []string
}

// DefaultSecurityConfig returns the default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		SystemDirs: []string{
			"/bin",
			"/sbin",
			"/usr/bin",
			"/usr/sbin",
			"/usr/libexec",
			"/System",
		},

		CriticalBinaries: []string{
			"bash", "sh", "zsh", "fish", // Shells
			"sudo", "su", "doas", // Privilege escalation
			"ssh", "sshd", // Remote access
			"login", "passwd", // Authentication
			"init", "systemd", // System init
			"launchd", // macOS init
		},
	}
}

// IsCriticalSystemBinary checks if binary name is critical
func IsCriticalSystemBinary(path string) bool {
	config := DefaultSecurityConfig()
	binName := filepath.Base(path)

	for _, critical := range config.CriticalBinaries {
		if binName == critical {
			return true
		}
	}

	return false
}

// RequiresConfirmation checks if path needs user confirmation (is in a system directory)
func RequiresConfirmation(path string) bool {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return true // Err on side of caution
	}

	config := DefaultSecurityConfig()

	for _, sysDir := range config.SystemDirs {
		if isWithinDir(abs, sysDir) {
			return true
		}
	}

	return false
}

// GetDirectoryCategory returns the security category for a path
func GetDirectoryCategory(path string) (DirectoryCategory, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return CategoryForbidden, err
	}

	// Check if requires confirmation (system directory)
	if RequiresConfirmation(abs) {
		return CategoryRequiresConfirmation, nil
	}

	// Default: allow all other directories
	return CategoryAllowed, nil
}

// ValidateBinaryForShim performs comprehensive validation
func ValidateBinaryForShim(path string, allowConfirmed bool) error {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	// Check if critical binary
	if IsCriticalSystemBinary(abs) {
		return fmt.Errorf("cannot shim critical system binary: %s\n\nShimming %s could compromise system security and stability.",
			filepath.Base(abs), filepath.Base(abs))
	}

	// Check directory category
	category, err := GetDirectoryCategory(abs)
	if err != nil {
		return err
	}

	switch category {
	case CategoryRequiresConfirmation:
		if !allowConfirmed {
			return fmt.Errorf("shimming %s requires explicit confirmation\n\nUse --confirm-system-dir flag if you understand the security implications",
				abs)
		}
		// Allowed with confirmation
		return nil

	case CategoryAllowed:
		// Safe to proceed
		return nil

	default:
		return fmt.Errorf("unknown directory category")
	}
}

// isWithinDir checks if path is within dir (handling symlinks)
func isWithinDir(path, dir string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..")
}
