package security

import (
	"fmt"
	"os"
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

// AllowlistConfig defines which directories are safe for shimming
type AllowlistConfig struct {
	// Directories where shimming is always safe
	AllowedDirs []string

	// Directories that require explicit confirmation
	ConfirmationDirs []string

	// Directories that are never allowed
	ForbiddenDirs []string

	// Specific critical binaries that must never be shimmed
	CriticalBinaries []string
}

// DefaultAllowlist returns the default secure allowlist
func DefaultAllowlist() *AllowlistConfig {
	home, _ := os.UserHomeDir()

	return &AllowlistConfig{
		AllowedDirs: []string{
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, "bin"),
			"./bin",                // Project-local bin
			"./node_modules/.bin",  // npm bins
		},

		ConfirmationDirs: []string{
			"/usr/local/bin",
			"/opt/homebrew/bin",
			"/opt/*/bin",
		},

		ForbiddenDirs: []string{
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

// IsAllowedDirectory checks if path is within an allowed directory
func IsAllowedDirectory(path string) (bool, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, err
	}

	config := DefaultAllowlist()

	// Check forbidden first
	for _, forbidden := range config.ForbiddenDirs {
		if isWithinDir(abs, forbidden) {
			return false, nil
		}
	}

	// Check allowed
	for _, allowed := range config.AllowedDirs {
		if isWithinDir(abs, allowed) {
			return true, nil
		}
	}

	// Not explicitly allowed
	return false, nil
}

// IsCriticalSystemBinary checks if binary name is critical
func IsCriticalSystemBinary(path string) bool {
	config := DefaultAllowlist()
	binName := filepath.Base(path)

	for _, critical := range config.CriticalBinaries {
		if binName == critical {
			return true
		}
	}

	return false
}

// RequiresConfirmation checks if path needs user confirmation
func RequiresConfirmation(path string) bool {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return true // Err on side of caution
	}

	config := DefaultAllowlist()

	for _, confirmDir := range config.ConfirmationDirs {
		// Handle wildcards like /opt/*/bin
		if strings.Contains(confirmDir, "*") {
			matched, _ := filepath.Match(confirmDir, abs)
			if matched {
				return true
			}
		} else if isWithinDir(abs, confirmDir) {
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

	config := DefaultAllowlist()

	// Check forbidden first
	for _, forbidden := range config.ForbiddenDirs {
		if isWithinDir(abs, forbidden) {
			return CategoryForbidden, nil
		}
	}

	// Check if requires confirmation
	if RequiresConfirmation(abs) {
		return CategoryRequiresConfirmation, nil
	}

	// Check allowed
	for _, allowed := range config.AllowedDirs {
		if isWithinDir(abs, allowed) {
			return CategoryAllowed, nil
		}
	}

	// Default: forbidden
	return CategoryForbidden, nil
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
	case CategoryForbidden:
		return fmt.Errorf("cannot shim binaries in system directory: %s\n\nDirectory %s is protected for security reasons.",
			abs, filepath.Dir(abs))

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
