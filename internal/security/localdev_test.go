package security

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestFindGitRoot(t *testing.T) {
	// Create a temp directory structure with a git repo
	tmpDir, err := os.MkdirTemp("", "localdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested structure: tmpDir/project/.git and tmpDir/project/node_modules/.bin
	projectDir := filepath.Join(tmpDir, "project")
	gitDir := filepath.Join(projectDir, ".git")
	nestedDir := filepath.Join(projectDir, "node_modules", ".bin")

	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	tests := []struct {
		name      string
		startPath string
		expected  string
	}{
		{
			name:      "from project root",
			startPath: projectDir,
			expected:  projectDir,
		},
		{
			name:      "from nested directory",
			startPath: nestedDir,
			expected:  projectDir,
		},
		{
			name:      "from .git directory itself",
			startPath: gitDir,
			expected:  projectDir,
		},
		{
			name:      "from directory without git",
			startPath: tmpDir,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findGitRoot(tt.startPath)
			if result != tt.expected {
				t.Errorf("findGitRoot(%q) = %q, want %q", tt.startPath, result, tt.expected)
			}
		})
	}
}

func TestFindGitRoot_WorktreeFile(t *testing.T) {
	// Test that .git as a file (worktree/submodule) is also detected
	tmpDir, err := os.MkdirTemp("", "localdev-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create .git as a file (like in worktrees)
	gitFile := filepath.Join(projectDir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /some/path/.git/worktrees/foo"), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	result := findGitRoot(projectDir)
	if result != projectDir {
		t.Errorf("findGitRoot with .git file = %q, want %q", result, projectDir)
	}
}

func TestLocalDevContext_ValidateBinaryPath(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "localdev-validate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks in tmpDir (macOS /var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve tmpDir: %v", err)
	}

	repoRoot := filepath.Join(tmpDir, "project")
	binDir := filepath.Join(repoRoot, "node_modules", ".bin")
	outsideDir := filepath.Join(tmpDir, "outside")

	for _, dir := range []string{binDir, outsideDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create test binaries
	insideBin := filepath.Join(binDir, "my-tool")
	outsideBin := filepath.Join(outsideDir, "system-tool")
	for _, bin := range []string{insideBin, outsideBin} {
		if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create binary %s: %v", bin, err)
		}
	}

	ctx := &LocalDevContext{
		IsLocalDev: true,
		RepoRoot:   repoRoot,
		RibbinPath: filepath.Join(binDir, "ribbin"),
	}

	tests := []struct {
		name        string
		binaryPath  string
		expectError bool
	}{
		{
			name:        "binary inside repo",
			binaryPath:  insideBin,
			expectError: false,
		},
		{
			name:        "binary at repo root",
			binaryPath:  filepath.Join(repoRoot, "bin", "tool"),
			expectError: false,
		},
		{
			name:        "binary outside repo",
			binaryPath:  outsideBin,
			expectError: true,
		},
		{
			name:        "system binary",
			binaryPath:  "/usr/bin/cat",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.ValidateBinaryPath(tt.binaryPath)
			if tt.expectError && err == nil {
				t.Errorf("ValidateBinaryPath(%q) expected error, got nil", tt.binaryPath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateBinaryPath(%q) unexpected error: %v", tt.binaryPath, err)
			}
		})
	}
}

func TestLocalDevContext_ValidateBinaryPath_NotLocalDev(t *testing.T) {
	// When not in local dev mode, all paths should be allowed
	ctx := &LocalDevContext{
		IsLocalDev: false,
		RepoRoot:   "",
		RibbinPath: "/usr/local/bin/ribbin",
	}

	// Even system paths should be allowed when not in local dev mode
	err := ctx.ValidateBinaryPath("/usr/bin/cat")
	if err != nil {
		t.Errorf("ValidateBinaryPath should allow all paths when not in local dev mode, got error: %v", err)
	}
}

func TestLocalDevContext_ValidateBinaryPath_RelativePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "localdev-relative-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks in tmpDir (macOS /var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve tmpDir: %v", err)
	}

	repoRoot := filepath.Join(tmpDir, "project")
	binDir := filepath.Join(repoRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create a binary
	binPath := filepath.Join(binDir, "tool")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create binary: %v", err)
	}

	ctx := &LocalDevContext{
		IsLocalDev: true,
		RepoRoot:   repoRoot,
		RibbinPath: filepath.Join(repoRoot, "node_modules", ".bin", "ribbin"),
	}

	// Change to repo root to test relative paths
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(repoRoot)

	err = ctx.ValidateBinaryPath("./bin/tool")
	if err != nil {
		t.Errorf("ValidateBinaryPath should allow relative paths within repo, got error: %v", err)
	}
}
