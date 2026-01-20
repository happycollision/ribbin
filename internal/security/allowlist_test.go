package security

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestIsCriticalSystemBinary(t *testing.T) {
	tests := []struct {
		path     string
		critical bool
	}{
		{"/usr/bin/bash", true},
		{"/bin/sh", true},
		{"/usr/bin/sudo", true},
		{"/usr/bin/zsh", true},
		{"/usr/bin/fish", true},
		{"/usr/bin/su", true},
		{"/usr/bin/doas", true},
		{"/usr/bin/ssh", true},
		{"/usr/bin/sshd", true},
		{"/usr/bin/login", true},
		{"/usr/bin/passwd", true},
		{"/usr/bin/init", true},
		{"/usr/bin/systemd", true},
		{"/usr/bin/launchd", true},
		{"/usr/local/bin/node", false},
		{"/home/user/.local/bin/myapp", false},
		{"/usr/bin/ls", false},
		{"/usr/bin/cat", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsCriticalSystemBinary(tt.path)
			if result != tt.critical {
				t.Errorf("IsCriticalSystemBinary(%s) = %v, want %v",
					tt.path, result, tt.critical)
			}
		})
	}
}

func TestGetDirectoryCategory(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		path     string
		category DirectoryCategory
	}{
		{
			name:     "user local bin",
			path:     filepath.Join(home, ".local/bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "user go bin",
			path:     filepath.Join(home, "go/bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "user cargo bin",
			path:     filepath.Join(home, ".cargo/bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "user bin",
			path:     filepath.Join(home, "bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "usr local bin",
			path:     "/usr/local/bin/tool",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "opt homebrew bin",
			path:     "/opt/homebrew/bin/node",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "usr bin",
			path:     "/usr/bin/bash",
			category: CategoryForbidden,
		},
		{
			name:     "bin",
			path:     "/bin/sh",
			category: CategoryForbidden,
		},
		{
			name:     "sbin",
			path:     "/sbin/init",
			category: CategoryForbidden,
		},
		{
			name:     "usr sbin",
			path:     "/usr/sbin/sshd",
			category: CategoryForbidden,
		},
		{
			name:     "usr libexec",
			path:     "/usr/libexec/something",
			category: CategoryForbidden,
		},
		{
			name:     "System directory (macOS)",
			path:     "/System/Library/something",
			category: CategoryForbidden,
		},
		{
			name:     "random directory",
			path:     "/tmp/mybinary",
			category: CategoryForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, err := GetDirectoryCategory(tt.path)
			if err != nil {
				t.Errorf("GetDirectoryCategory(%s) returned error: %v", tt.path, err)
				return
			}
			if cat != tt.category {
				t.Errorf("GetDirectoryCategory(%s) = %v, want %v", tt.path, cat, tt.category)
			}
		})
	}
}

func TestIsAllowedDirectory(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name    string
		path    string
		allowed bool
	}{
		{
			name:    "user local bin",
			path:    filepath.Join(home, ".local/bin/tool"),
			allowed: true,
		},
		{
			name:    "user go bin",
			path:    filepath.Join(home, "go/bin/tool"),
			allowed: true,
		},
		{
			name:    "usr bin forbidden",
			path:    "/usr/bin/bash",
			allowed: false,
		},
		{
			name:    "bin forbidden",
			path:    "/bin/sh",
			allowed: false,
		},
		{
			name:    "usr local bin not allowed by default",
			path:    "/usr/local/bin/tool",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := IsAllowedDirectory(tt.path)
			if err != nil {
				t.Errorf("IsAllowedDirectory(%s) returned error: %v", tt.path, err)
				return
			}
			if allowed != tt.allowed {
				t.Errorf("IsAllowedDirectory(%s) = %v, want %v", tt.path, allowed, tt.allowed)
			}
		})
	}
}

func TestRequiresConfirmation(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		requires bool
	}{
		{
			name:     "usr local bin requires confirmation",
			path:     "/usr/local/bin/tool",
			requires: true,
		},
		{
			name:     "opt homebrew bin requires confirmation",
			path:     "/opt/homebrew/bin/node",
			requires: true,
		},
		{
			name:     "usr bin does not require confirmation (forbidden)",
			path:     "/usr/bin/bash",
			requires: false,
		},
		{
			name:     "user bin does not require confirmation (allowed)",
			path:     filepath.Join(os.Getenv("HOME"), ".local/bin/tool"),
			requires: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requires := RequiresConfirmation(tt.path)
			if requires != tt.requires {
				t.Errorf("RequiresConfirmation(%s) = %v, want %v", tt.path, requires, tt.requires)
			}
		})
	}
}

func TestValidateBinaryForShim_CriticalBinary(t *testing.T) {
	criticalBinaries := []string{
		"/usr/bin/bash",
		"/bin/sh",
		"/usr/bin/sudo",
		"/usr/bin/su",
		"/usr/bin/ssh",
	}

	for _, binPath := range criticalBinaries {
		t.Run(binPath, func(t *testing.T) {
			err := ValidateBinaryForShim(binPath, false)
			if err == nil {
				t.Errorf("ValidateBinaryForShim(%s, false) expected error, got nil", binPath)
				return
			}
			if !containsString(err.Error(), "critical system binary") {
				t.Errorf("ValidateBinaryForShim(%s, false) error should mention 'critical system binary', got: %v", binPath, err)
			}
		})
	}
}

func TestValidateBinaryForShim_ForbiddenDirectory(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "usr bin",
			path: "/usr/bin/myapp",
		},
		{
			name: "bin",
			path: "/bin/myapp",
		},
		{
			name: "sbin",
			path: "/sbin/myapp",
		},
		{
			name: "usr sbin",
			path: "/usr/sbin/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBinaryForShim(tt.path, false)
			if err == nil {
				t.Errorf("ValidateBinaryForShim(%s, false) expected error, got nil", tt.path)
				return
			}
			if !containsString(err.Error(), "protected") && !containsString(err.Error(), "system directory") {
				t.Errorf("ValidateBinaryForShim(%s, false) error should mention 'protected' or 'system directory', got: %v", tt.path, err)
			}
		})
	}
}

func TestValidateBinaryForShim_RequiresConfirmation(t *testing.T) {
	// Without confirmation flag
	err := ValidateBinaryForShim("/usr/local/bin/myapp", false)
	if err == nil {
		t.Error("ValidateBinaryForShim(/usr/local/bin/myapp, false) expected error, got nil")
	} else if !containsString(err.Error(), "confirmation") {
		t.Errorf("ValidateBinaryForShim(/usr/local/bin/myapp, false) error should mention 'confirmation', got: %v", err)
	}

	// With confirmation flag
	err = ValidateBinaryForShim("/usr/local/bin/myapp", true)
	if err != nil {
		t.Errorf("ValidateBinaryForShim(/usr/local/bin/myapp, true) expected no error, got: %v", err)
	}
}

func TestValidateBinaryForShim_AllowedDirectory(t *testing.T) {
	home, _ := os.UserHomeDir()
	allowedPaths := []string{
		filepath.Join(home, ".local/bin/myapp"),
		filepath.Join(home, "go/bin/myapp"),
		filepath.Join(home, ".cargo/bin/myapp"),
		filepath.Join(home, "bin/myapp"),
	}

	for _, path := range allowedPaths {
		t.Run(path, func(t *testing.T) {
			err := ValidateBinaryForShim(path, false)
			if err != nil {
				t.Errorf("ValidateBinaryForShim(%s, false) expected no error, got: %v", path, err)
			}
		})
	}
}

func TestIsWithinDir(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		dir    string
		within bool
	}{
		{
			name:   "direct child",
			path:   "/usr/bin/bash",
			dir:    "/usr/bin",
			within: true,
		},
		{
			name:   "nested path",
			path:   "/usr/local/bin/node",
			dir:    "/usr/local",
			within: true,
		},
		{
			name:   "not within",
			path:   "/usr/bin/bash",
			dir:    "/opt",
			within: false,
		},
		{
			name:   "parent directory not within child",
			path:   "/usr",
			dir:    "/usr/bin",
			within: false,
		},
		{
			name:   "exact match",
			path:   "/usr/bin",
			dir:    "/usr/bin",
			within: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWithinDir(tt.path, tt.dir)
			if result != tt.within {
				t.Errorf("isWithinDir(%s, %s) = %v, want %v", tt.path, tt.dir, result, tt.within)
			}
		})
	}
}

func TestDefaultAllowlist(t *testing.T) {
	config := DefaultAllowlist()

	// Verify critical binaries are included
	if len(config.CriticalBinaries) == 0 {
		t.Error("DefaultAllowlist() CriticalBinaries should not be empty")
	}

	// Verify shells are in critical binaries
	criticalShells := []string{"bash", "sh", "zsh", "fish"}
	for _, shell := range criticalShells {
		found := false
		for _, critical := range config.CriticalBinaries {
			if critical == shell {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultAllowlist() CriticalBinaries should include %s", shell)
		}
	}

	// Verify privilege escalation tools are in critical binaries
	privilegeTools := []string{"sudo", "su", "doas"}
	for _, tool := range privilegeTools {
		found := false
		for _, critical := range config.CriticalBinaries {
			if critical == tool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultAllowlist() CriticalBinaries should include %s", tool)
		}
	}

	// Verify forbidden directories are included
	forbiddenDirs := []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin"}
	for _, dir := range forbiddenDirs {
		found := false
		for _, forbidden := range config.ForbiddenDirs {
			if forbidden == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultAllowlist() ForbiddenDirs should include %s", dir)
		}
	}

	// Verify allowed directories are included
	home, _ := os.UserHomeDir()
	allowedDirs := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "go", "bin"),
	}
	for _, dir := range allowedDirs {
		found := false
		for _, allowed := range config.AllowedDirs {
			if allowed == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultAllowlist() AllowedDirs should include %s", dir)
		}
	}

	// Verify confirmation directories are included
	confirmDirs := []string{"/usr/local/bin", "/opt/homebrew/bin"}
	for _, dir := range confirmDirs {
		found := false
		for _, confirm := range config.ConfirmationDirs {
			if confirm == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultAllowlist() ConfirmationDirs should include %s", dir)
		}
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
