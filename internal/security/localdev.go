package security

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalDevContext represents the local development mode state.
// When ribbin is installed as a dev dependency (inside a git repo),
// it can only shim binaries within that same repository.
type LocalDevContext struct {
	// IsLocalDev is true if ribbin is inside a git repository
	IsLocalDev bool
	// RepoRoot is the absolute path to the git repository root
	RepoRoot string
	// RibbinPath is the resolved path to the ribbin executable
	RibbinPath string
}

// DetectLocalDevMode checks if ribbin is installed as a dev dependency
// by looking for a .git directory above ribbin's location.
// Returns a context with IsLocalDev=false if not in a git repo.
func DetectLocalDevMode() (*LocalDevContext, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot get executable path: %w", err)
	}

	// Resolve symlinks to get the actual ribbin location
	ribbinPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve executable path: %w", err)
	}

	// Look for a git repo containing ribbin
	repoRoot := findGitRoot(filepath.Dir(ribbinPath))

	return &LocalDevContext{
		IsLocalDev: repoRoot != "",
		RepoRoot:   repoRoot,
		RibbinPath: ribbinPath,
	}, nil
}

// ValidateBinaryPath checks if a binary path is allowed in local dev mode.
// In local dev mode, only binaries within the same repository can be shimmed.
// Returns nil if allowed, error with explanation if not.
func (ctx *LocalDevContext) ValidateBinaryPath(binaryPath string) error {
	// If not in local dev mode, all paths are allowed (other checks apply)
	if !ctx.IsLocalDev {
		return nil
	}

	// Resolve the binary path to absolute
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	// Resolve symlinks for consistent comparison
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If we can't resolve, use the absolute path
		resolvedPath = absPath
	}

	// Check if the binary is within the repository
	if !isWithinDir(resolvedPath, ctx.RepoRoot) {
		return fmt.Errorf("Local Development Mode active\n"+
			"  ribbin location: %s\n"+
			"  repository root: %s\n\n"+
			"Cannot shim '%s': path is outside repository\n"+
			"  Use a repo-local binary instead (e.g., ./node_modules/.bin/...)",
			ctx.RibbinPath, ctx.RepoRoot, binaryPath)
	}

	return nil
}

// findGitRoot walks up from startPath looking for a .git directory or file.
// Returns the directory containing .git, or empty string if not found.
func findGitRoot(startPath string) string {
	// Get absolute path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}

	dir := absPath
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			// .git can be a directory (normal repo) or file (worktree/submodule)
			if info.IsDir() || info.Mode().IsRegular() {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return ""
		}
		dir = parent
	}
}

// Note: isWithinDir is defined in allowlist.go and reused here
