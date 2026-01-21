package security

import (
	"os"
	"path/filepath"
	"strings"
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
			name:     "user local bin - allowed (not a system dir)",
			path:     filepath.Join(home, ".local/bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "user go bin - allowed",
			path:     filepath.Join(home, "go/bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "user cargo bin - allowed",
			path:     filepath.Join(home, ".cargo/bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "user bin - allowed",
			path:     filepath.Join(home, "bin/tool"),
			category: CategoryAllowed,
		},
		{
			name:     "usr local bin - allowed (not a system dir)",
			path:     "/usr/local/bin/tool",
			category: CategoryAllowed,
		},
		{
			name:     "opt homebrew bin - allowed",
			path:     "/opt/homebrew/bin/node",
			category: CategoryAllowed,
		},
		{
			name:     "usr bin - system dir",
			path:     "/usr/bin/myapp",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "bin - system dir",
			path:     "/bin/myapp",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "sbin - system dir",
			path:     "/sbin/myapp",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "usr sbin - system dir",
			path:     "/usr/sbin/myapp",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "usr libexec - system dir",
			path:     "/usr/libexec/something",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "System directory (macOS)",
			path:     "/System/Library/something",
			category: CategoryRequiresConfirmation,
		},
		{
			name:     "random directory - allowed (blacklist model)",
			path:     "/tmp/mybinary",
			category: CategoryAllowed,
		},
		{
			name:     "project test-bin - allowed",
			path:     "/home/user/myproject/test-bin/tool",
			category: CategoryAllowed,
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

func TestRequiresConfirmation(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		requires bool
	}{
		{
			name:     "usr local bin - not a system dir",
			path:     "/usr/local/bin/tool",
			requires: false,
		},
		{
			name:     "opt homebrew bin - not a system dir",
			path:     "/opt/homebrew/bin/node",
			requires: false,
		},
		{
			name:     "usr bin - system dir",
			path:     "/usr/bin/myapp",
			requires: true,
		},
		{
			name:     "bin - system dir",
			path:     "/bin/myapp",
			requires: true,
		},
		{
			name:     "user local bin - not a system dir",
			path:     filepath.Join(os.Getenv("HOME"), ".local/bin/tool"),
			requires: false,
		},
		{
			name:     "random tmp dir - not a system dir",
			path:     "/tmp/my-tool",
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
			if !strings.Contains(err.Error(), "critical system binary") {
				t.Errorf("ValidateBinaryForShim(%s, false) error should mention 'critical system binary', got: %v", binPath, err)
			}
		})
	}
}

func TestValidateBinaryForShim_SystemDirectory(t *testing.T) {
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
			// Without confirmation flag - should fail
			err := ValidateBinaryForShim(tt.path, false)
			if err == nil {
				t.Errorf("ValidateBinaryForShim(%s, false) expected error, got nil", tt.path)
				return
			}
			if !strings.Contains(err.Error(), "confirmation") {
				t.Errorf("ValidateBinaryForShim(%s, false) error should mention 'confirmation', got: %v", tt.path, err)
			}

			// With confirmation flag - should succeed
			err = ValidateBinaryForShim(tt.path, true)
			if err != nil {
				t.Errorf("ValidateBinaryForShim(%s, true) expected no error, got: %v", tt.path, err)
			}
		})
	}
}

func TestValidateBinaryForShim_UsrLocalBinAllowed(t *testing.T) {
	// /usr/local/bin is allowed without confirmation (not a system dir)
	err := ValidateBinaryForShim("/usr/local/bin/myapp", false)
	if err != nil {
		t.Errorf("ValidateBinaryForShim(/usr/local/bin/myapp, false) expected no error, got: %v", err)
	}
}

func TestValidateBinaryForShim_NonSystemDirectory(t *testing.T) {
	home, _ := os.UserHomeDir()
	allowedPaths := []string{
		filepath.Join(home, ".local/bin/myapp"),
		filepath.Join(home, "go/bin/myapp"),
		filepath.Join(home, ".cargo/bin/myapp"),
		filepath.Join(home, "bin/myapp"),
		"/tmp/my-tool",
		"/opt/homebrew/bin/node",
		"/some/random/path/tool",
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

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	// Verify critical binaries are included
	if len(config.CriticalBinaries) == 0 {
		t.Error("DefaultSecurityConfig() CriticalBinaries should not be empty")
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
			t.Errorf("DefaultSecurityConfig() CriticalBinaries should include %s", shell)
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
			t.Errorf("DefaultSecurityConfig() CriticalBinaries should include %s", tool)
		}
	}

	// Verify system directories are included
	systemDirs := []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin"}
	for _, dir := range systemDirs {
		found := false
		for _, sysDir := range config.SystemDirs {
			if sysDir == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultSecurityConfig() SystemDirs should include %s", dir)
		}
	}
}
