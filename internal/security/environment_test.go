package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateHomeDir(t *testing.T) {
	home, err := ValidateHomeDir()
	if err != nil {
		t.Fatalf("ValidateHomeDir() error = %v", err)
	}

	// Should be absolute
	if !filepath.IsAbs(home) {
		t.Errorf("ValidateHomeDir() = %q, want absolute path", home)
	}

	// Should not contain traversal
	if strings.Contains(home, "..") {
		t.Errorf("ValidateHomeDir() = %q, should not contain '..'", home)
	}

	// Should exist and be a directory
	info, err := os.Stat(home)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", home, err)
	}
	if !info.IsDir() {
		t.Errorf("ValidateHomeDir() = %q, want directory", home)
	}
}

func TestValidateEnvPath(t *testing.T) {
	t.Run("valid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("TEST_VALID_PATH", tmpDir)
		defer os.Unsetenv("TEST_VALID_PATH")

		result, err := ValidateEnvPath("TEST_VALID_PATH")
		if err != nil {
			t.Fatalf("ValidateEnvPath() error = %v", err)
		}
		if !filepath.IsAbs(result) {
			t.Errorf("ValidateEnvPath() = %q, want absolute path", result)
		}
	})

	t.Run("unset variable", func(t *testing.T) {
		os.Unsetenv("TEST_UNSET_VAR")

		_, err := ValidateEnvPath("TEST_UNSET_VAR")
		if err == nil {
			t.Error("ValidateEnvPath() expected error for unset var")
		}
		if !strings.Contains(err.Error(), "not set") {
			t.Errorf("ValidateEnvPath() error = %q, want 'not set'", err)
		}
	})

	t.Run("path traversal", func(t *testing.T) {
		os.Setenv("TEST_TRAVERSAL", "/tmp/../etc/passwd")
		defer os.Unsetenv("TEST_TRAVERSAL")

		_, err := ValidateEnvPath("TEST_TRAVERSAL")
		if err == nil {
			t.Error("ValidateEnvPath() expected error for path traversal")
		}
		if !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("ValidateEnvPath() error = %q, want 'path traversal'", err)
		}
	})

	// Note: Environment variables cannot actually contain null bytes in most shells
	// The shell terminates strings at the null byte, so this is tested in SafeExpandPath instead
}

func TestGetConfigDir(t *testing.T) {
	t.Run("default (no XDG)", func(t *testing.T) {
		// Save and unset XDG_CONFIG_HOME
		original := os.Getenv("XDG_CONFIG_HOME")
		os.Unsetenv("XDG_CONFIG_HOME")
		defer func() {
			if original != "" {
				os.Setenv("XDG_CONFIG_HOME", original)
			}
		}()

		configDir, err := GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		if !strings.Contains(configDir, ".config/ribbin") {
			t.Errorf("GetConfigDir() = %q, want to contain '.config/ribbin'", configDir)
		}
		if !filepath.IsAbs(configDir) {
			t.Errorf("GetConfigDir() = %q, want absolute path", configDir)
		}
	})

	t.Run("with XDG_CONFIG_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Unsetenv("XDG_CONFIG_HOME")

		configDir, err := GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() error = %v", err)
		}
		expected := filepath.Join(tmpDir, "ribbin")
		if configDir != expected {
			t.Errorf("GetConfigDir() = %q, want %q", configDir, expected)
		}
	})

	t.Run("invalid XDG_CONFIG_HOME with traversal", func(t *testing.T) {
		os.Setenv("XDG_CONFIG_HOME", "/tmp/../etc")
		defer os.Unsetenv("XDG_CONFIG_HOME")

		_, err := GetConfigDir()
		if err == nil {
			t.Error("GetConfigDir() expected error for path traversal")
		}
		if !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("GetConfigDir() error = %q, want 'path traversal'", err)
		}
	})
}

func TestGetStateDir(t *testing.T) {
	t.Run("default (no XDG)", func(t *testing.T) {
		// Save and unset XDG_STATE_HOME
		original := os.Getenv("XDG_STATE_HOME")
		os.Unsetenv("XDG_STATE_HOME")
		defer func() {
			if original != "" {
				os.Setenv("XDG_STATE_HOME", original)
			}
		}()

		stateDir, err := GetStateDir()
		if err != nil {
			t.Fatalf("GetStateDir() error = %v", err)
		}
		if !strings.Contains(stateDir, ".local/state/ribbin") {
			t.Errorf("GetStateDir() = %q, want to contain '.local/state/ribbin'", stateDir)
		}
		if !filepath.IsAbs(stateDir) {
			t.Errorf("GetStateDir() = %q, want absolute path", stateDir)
		}
	})

	t.Run("with XDG_STATE_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("XDG_STATE_HOME", tmpDir)
		defer os.Unsetenv("XDG_STATE_HOME")

		stateDir, err := GetStateDir()
		if err != nil {
			t.Fatalf("GetStateDir() error = %v", err)
		}
		expected := filepath.Join(tmpDir, "ribbin")
		if stateDir != expected {
			t.Errorf("GetStateDir() = %q, want %q", stateDir, expected)
		}
	})
}

func TestSafeExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "home expansion",
			input:    "~/test",
			expected: filepath.Join(home, "test"),
			wantErr:  false,
		},
		{
			name:     "tilde only",
			input:    "~",
			expected: home,
			wantErr:  false,
		},
		{
			name:     "absolute path",
			input:    "/usr/local/bin",
			expected: "/usr/local/bin",
			wantErr:  false,
		},
		{
			name:    "empty path",
			input:   "",
			wantErr: true,
			errMsg:  "empty path",
		},
		{
			name:    "path traversal",
			input:   "/tmp/../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal",
		},
		{
			name:    "null byte",
			input:   "/tmp/test\x00evil",
			wantErr: true,
			errMsg:  "null byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeExpandPath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SafeExpandPath(%q) expected error", tt.input)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SafeExpandPath(%q) error = %q, want to contain %q", tt.input, err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("SafeExpandPath(%q) error = %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("SafeExpandPath(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestValidateRegistryPath(t *testing.T) {
	path, err := ValidateRegistryPath()
	if err != nil {
		t.Fatalf("ValidateRegistryPath() error = %v", err)
	}

	// Should end with registry.json
	if !strings.HasSuffix(path, "registry.json") {
		t.Errorf("ValidateRegistryPath() = %q, want to end with 'registry.json'", path)
	}

	// Should be within ribbin config dir
	if !strings.Contains(path, "ribbin") {
		t.Errorf("ValidateRegistryPath() = %q, want to contain 'ribbin'", path)
	}

	// Should be absolute
	if !filepath.IsAbs(path) {
		t.Errorf("ValidateRegistryPath() = %q, want absolute path", path)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// Use a temp XDG_CONFIG_HOME so we don't touch real config
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	configDir, err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() error = %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", configDir, err)
	}
	if !info.IsDir() {
		t.Errorf("EnsureConfigDir() = %q, want directory", configDir)
	}

	expected := filepath.Join(tmpDir, "ribbin")
	if configDir != expected {
		t.Errorf("EnsureConfigDir() = %q, want %q", configDir, expected)
	}
}

func TestEnsureStateDir(t *testing.T) {
	// Use a temp XDG_STATE_HOME so we don't touch real state
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	stateDir, err := EnsureStateDir()
	if err != nil {
		t.Fatalf("EnsureStateDir() error = %v", err)
	}

	// Directory should now exist with restricted permissions
	info, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", stateDir, err)
	}
	if !info.IsDir() {
		t.Errorf("EnsureStateDir() = %q, want directory", stateDir)
	}

	expected := filepath.Join(tmpDir, "ribbin")
	if stateDir != expected {
		t.Errorf("EnsureStateDir() = %q, want %q", stateDir, expected)
	}

	// Check permissions (0700 = owner read/write/execute only)
	perm := info.Mode().Perm()
	if perm != os.FileMode(0700) {
		t.Errorf("EnsureStateDir() permissions = %o, want 0700", perm)
	}
}

func TestVerifyOwnership(t *testing.T) {
	// Create a temp file owned by current user
	tmpDir := t.TempDir()

	// This should pass - temp dir is owned by current user
	err := verifyOwnership(tmpDir)
	if err != nil {
		t.Errorf("verifyOwnership(%q) error = %v", tmpDir, err)
	}
}
