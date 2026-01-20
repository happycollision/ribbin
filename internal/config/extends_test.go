package config

import (
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestParseExtendsRef_LocalReferences(t *testing.T) {
	configDir := "/project"

	tests := []struct {
		name         string
		ref          string
		wantFilePath string
		wantFragment string
		wantIsLocal  bool
	}{
		{
			name:         "root only",
			ref:          "root",
			wantFilePath: "",
			wantFragment: "root",
			wantIsLocal:  true,
		},
		{
			name:         "root with scope",
			ref:          "root.backend",
			wantFilePath: "",
			wantFragment: "root.backend",
			wantIsLocal:  true,
		},
		{
			name:         "root with hyphenated scope",
			ref:          "root.hardened",
			wantFilePath: "",
			wantFragment: "root.hardened",
			wantIsLocal:  true,
		},
		{
			name:         "root with nested scope name",
			ref:          "root.frontend.react",
			wantFilePath: "",
			wantFragment: "root.frontend.react",
			wantIsLocal:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseExtendsRef(tt.ref, configDir)
			if err != nil {
				t.Fatalf("ParseExtendsRef(%q) error = %v", tt.ref, err)
			}
			if got.FilePath != tt.wantFilePath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.wantFilePath)
			}
			if got.Fragment != tt.wantFragment {
				t.Errorf("Fragment = %q, want %q", got.Fragment, tt.wantFragment)
			}
			if got.IsLocal != tt.wantIsLocal {
				t.Errorf("IsLocal = %v, want %v", got.IsLocal, tt.wantIsLocal)
			}
		})
	}
}

func TestParseExtendsRef_RelativeFilePaths(t *testing.T) {
	configDir := "/project/config"

	tests := []struct {
		name         string
		ref          string
		wantFilePath string
		wantFragment string
		wantIsLocal  bool
	}{
		{
			name:         "relative sibling file",
			ref:          "./other.toml",
			wantFilePath: "/project/config/other.toml",
			wantFragment: "",
			wantIsLocal:  false,
		},
		{
			name:         "relative parent directory",
			ref:          "../other.toml",
			wantFilePath: "/project/other.toml",
			wantFragment: "",
			wantIsLocal:  false,
		},
		{
			name:         "relative with fragment",
			ref:          "./file.toml#root",
			wantFilePath: "/project/config/file.toml",
			wantFragment: "root",
			wantIsLocal:  false,
		},
		{
			name:         "relative with scope fragment",
			ref:          "./file.toml#root.scope",
			wantFilePath: "/project/config/file.toml",
			wantFragment: "root.scope",
			wantIsLocal:  false,
		},
		{
			name:         "parent with subdirectory",
			ref:          "../team-standards/ribbin.toml",
			wantFilePath: "/project/team-standards/ribbin.toml",
			wantFragment: "",
			wantIsLocal:  false,
		},
		{
			name:         "current dir with subdirectory",
			ref:          "./configs/base.toml#root.hardened",
			wantFilePath: "/project/config/configs/base.toml",
			wantFragment: "root.hardened",
			wantIsLocal:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseExtendsRef(tt.ref, configDir)
			if err != nil {
				t.Fatalf("ParseExtendsRef(%q) error = %v", tt.ref, err)
			}
			if got.FilePath != tt.wantFilePath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.wantFilePath)
			}
			if got.Fragment != tt.wantFragment {
				t.Errorf("Fragment = %q, want %q", got.Fragment, tt.wantFragment)
			}
			if got.IsLocal != tt.wantIsLocal {
				t.Errorf("IsLocal = %v, want %v", got.IsLocal, tt.wantIsLocal)
			}
		})
	}
}

func TestParseExtendsRef_AbsoluteFilePaths(t *testing.T) {
	configDir := "/project"

	tests := []struct {
		name         string
		ref          string
		wantFilePath string
		wantFragment string
	}{
		{
			name:         "absolute path",
			ref:          "/abs/path/ribbin.toml",
			wantFilePath: "/abs/path/ribbin.toml",
			wantFragment: "",
		},
		{
			name:         "absolute path with fragment",
			ref:          "/abs/path/ribbin.toml#root",
			wantFilePath: "/abs/path/ribbin.toml",
			wantFragment: "root",
		},
		{
			name:         "absolute path with scope fragment",
			ref:          "/abs/path/ribbin.toml#root.myScope",
			wantFilePath: "/abs/path/ribbin.toml",
			wantFragment: "root.myScope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseExtendsRef(tt.ref, configDir)
			if err != nil {
				t.Fatalf("ParseExtendsRef(%q) error = %v", tt.ref, err)
			}
			if got.FilePath != tt.wantFilePath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.wantFilePath)
			}
			if got.Fragment != tt.wantFragment {
				t.Errorf("Fragment = %q, want %q", got.Fragment, tt.wantFragment)
			}
			if got.IsLocal {
				t.Errorf("IsLocal = true, want false")
			}
		})
	}
}

func TestParseExtendsRef_Errors(t *testing.T) {
	configDir := "/project"

	tests := []struct {
		name    string
		ref     string
		wantErr string
	}{
		{
			name:    "empty reference",
			ref:     "",
			wantErr: "extends reference cannot be empty",
		},
		{
			name:    "bare filename without prefix",
			ref:     "other.toml",
			wantErr: "relative path must start with './' or '../'",
		},
		{
			name:    "just a hash",
			ref:     "#root",
			wantErr: "missing file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseExtendsRef(tt.ref, configDir)
			if err == nil {
				t.Fatalf("ParseExtendsRef(%q) expected error containing %q, got nil", tt.ref, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseExtendsRef_PathCleaning(t *testing.T) {
	configDir := "/project/config"

	// Paths with redundant components should be cleaned
	got, err := ParseExtendsRef("./foo/../bar.toml", configDir)
	if err != nil {
		t.Fatalf("ParseExtendsRef error = %v", err)
	}
	want := filepath.Clean("/project/config/bar.toml")
	if got.FilePath != want {
		t.Errorf("FilePath = %q, want %q", got.FilePath, want)
	}
}

func TestIsLocalRef(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"root", true},
		{"root.backend", true},
		{"root.hardened", true},
		{"root.", false},            // incomplete
		{"rootish", false},          // not "root" or "root."
		{"./root", false},           // file path
		{"../root", false},          // file path
		{"/root", false},            // absolute path
		{"other.toml", false},       // file without prefix
		{"./file.toml#root", false}, // file with fragment
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := isLocalRef(tt.ref)
			if got != tt.want {
				t.Errorf("isLocalRef(%q) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}

func TestSplitFileAndFragment(t *testing.T) {
	tests := []struct {
		ref          string
		wantFile     string
		wantFragment string
	}{
		{"./file.toml", "./file.toml", ""},
		{"./file.toml#root", "./file.toml", "root"},
		{"./file.toml#root.scope", "./file.toml", "root.scope"},
		{"/abs/path.toml#frag", "/abs/path.toml", "frag"},
		{"#fragment", "", "fragment"},
		{"file#with#multiple#hashes", "file#with#multiple", "hashes"},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			gotFile, gotFragment := splitFileAndFragment(tt.ref)
			if gotFile != tt.wantFile {
				t.Errorf("file = %q, want %q", gotFile, tt.wantFile)
			}
			if gotFragment != tt.wantFragment {
				t.Errorf("fragment = %q, want %q", gotFragment, tt.wantFragment)
			}
		})
	}
}
