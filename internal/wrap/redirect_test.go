package wrap

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestResolveRedirectScript(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "ribbin-redirect-test-*")
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectDir, 0755)

	scriptsDir := filepath.Join(projectDir, "scripts")
	os.MkdirAll(scriptsDir, 0755)

	configPath := filepath.Join(projectDir, "ribbin.toml")

	t.Run("resolves relative path", func(t *testing.T) {
		scriptPath := filepath.Join(scriptsDir, "test.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/sh\necho test\n"), 0755)

		resolved, err := resolveRedirectScript("./scripts/test.sh", configPath)
		if err != nil {
			t.Fatalf("should resolve: %v", err)
		}
		if resolved != scriptPath {
			t.Errorf("expected %s, got %s", scriptPath, resolved)
		}
	})

	t.Run("resolves absolute path", func(t *testing.T) {
		scriptPath := filepath.Join(tmpDir, "absolute.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/sh\necho test\n"), 0755)

		resolved, err := resolveRedirectScript(scriptPath, configPath)
		if err != nil {
			t.Fatalf("should resolve: %v", err)
		}
		if resolved != scriptPath {
			t.Errorf("expected %s, got %s", scriptPath, resolved)
		}
	})

	t.Run("errors on missing script", func(t *testing.T) {
		_, err := resolveRedirectScript("./missing.sh", configPath)
		if err == nil {
			t.Error("should error on missing script")
		}
	})

	t.Run("errors on non-executable script", func(t *testing.T) {
		scriptPath := filepath.Join(scriptsDir, "noexec.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/sh\necho test\n"), 0644) // Not executable

		_, err := resolveRedirectScript("./scripts/noexec.sh", configPath)
		if err == nil {
			t.Error("should error on non-executable script")
		}
	})

	t.Run("resolves parent directory references", func(t *testing.T) {
		// Create a script in the parent directory
		parentScript := filepath.Join(tmpDir, "parent.sh")
		os.WriteFile(parentScript, []byte("#!/bin/sh\necho parent\n"), 0755)

		// Reference it from the project directory using ../
		resolved, err := resolveRedirectScript("../parent.sh", configPath)
		if err != nil {
			t.Fatalf("should resolve parent directory reference: %v", err)
		}
		if resolved != parentScript {
			t.Errorf("expected %s, got %s", parentScript, resolved)
		}
	})
}

func TestValidateExecutable(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "ribbin-validate-test-*")
	defer os.RemoveAll(tmpDir)

	t.Run("validates executable file", func(t *testing.T) {
		script := filepath.Join(tmpDir, "exec.sh")
		os.WriteFile(script, []byte("#!/bin/sh\n"), 0755)

		_, err := validateExecutable(script)
		if err != nil {
			t.Errorf("should validate executable: %v", err)
		}
	})

	t.Run("rejects directory", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "dir")
		os.MkdirAll(dir, 0755)

		_, err := validateExecutable(dir)
		if err == nil {
			t.Error("should reject directory")
		}
	})

	t.Run("rejects non-executable", func(t *testing.T) {
		script := filepath.Join(tmpDir, "noexec.sh")
		os.WriteFile(script, []byte("#!/bin/sh\n"), 0644)

		_, err := validateExecutable(script)
		if err == nil {
			t.Error("should reject non-executable")
		}
	})

	t.Run("rejects missing file", func(t *testing.T) {
		missing := filepath.Join(tmpDir, "missing.sh")

		_, err := validateExecutable(missing)
		if err == nil {
			t.Error("should reject missing file")
		}
	})
}
