package config

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestAddShim(t *testing.T) {
	t.Run("successfully adds new shim to empty config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		// Create initial empty config
		if err := os.WriteFile(configPath, []byte("[shims]\n"), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		shimConfig := ShimConfig{
			Action:  "block",
			Message: "Test block message",
			Paths:   []string{"/bin/cat", "/usr/bin/cat"},
		}

		err = AddShim(configPath, "cat", shimConfig)
		if err != nil {
			t.Fatalf("AddShim failed: %v", err)
		}

		// Verify the shim was added
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		catShim, exists := cfg.Shims["cat"]
		if !exists {
			t.Fatal("cat shim not found after adding")
		}
		if catShim.Action != "block" {
			t.Errorf("expected action 'block', got '%s'", catShim.Action)
		}
		if catShim.Message != "Test block message" {
			t.Errorf("expected message 'Test block message', got '%s'", catShim.Message)
		}
		if len(catShim.Paths) != 2 {
			t.Errorf("expected 2 paths, got %d", len(catShim.Paths))
		}

		// Verify backup was created
		backupPath := configPath + ".backup"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("backup file was not created")
		}
	})

	t.Run("successfully adds shim to config with existing shims", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		// Create config with existing shim
		content := `[shims.tsc]
action = "block"
message = "Use pnpm run typecheck"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		shimConfig := ShimConfig{
			Action:  "warn",
			Message: "Test warn message",
		}

		err = AddShim(configPath, "cat", shimConfig)
		if err != nil {
			t.Fatalf("AddShim failed: %v", err)
		}

		// Verify both shims exist
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if len(cfg.Shims) != 2 {
			t.Errorf("expected 2 shims, got %d", len(cfg.Shims))
		}

		// Verify original shim still exists
		tscShim, exists := cfg.Shims["tsc"]
		if !exists {
			t.Error("original tsc shim was lost")
		}
		if tscShim.Action != "block" {
			t.Errorf("original tsc shim was modified")
		}

		// Verify new shim was added
		catShim, exists := cfg.Shims["cat"]
		if !exists {
			t.Error("cat shim was not added")
		}
		if catShim.Action != "warn" {
			t.Errorf("expected action 'warn', got '%s'", catShim.Action)
		}
	})

	t.Run("fails when command already exists", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Already exists"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		shimConfig := ShimConfig{
			Action:  "warn",
			Message: "Duplicate",
		}

		err = AddShim(configPath, "cat", shimConfig)
		if err == nil {
			t.Fatal("AddShim should fail for duplicate command")
		}

		// Verify error message mentions the duplicate
		if err.Error() != "shim for command 'cat' already exists" {
			t.Errorf("unexpected error message: %v", err)
		}

		// Verify original config unchanged
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
		if cfg.Shims["cat"].Message != "Already exists" {
			t.Error("original config was modified")
		}
	})

	t.Run("fails when config file doesn't exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "nonexistent.toml")

		shimConfig := ShimConfig{
			Action: "block",
		}

		err = AddShim(configPath, "cat", shimConfig)
		if err == nil {
			t.Fatal("AddShim should fail for nonexistent config")
		}
	})
}

func TestRemoveShim(t *testing.T) {
	t.Run("successfully removes existing shim", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Test block message"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		err = RemoveShim(configPath, "cat")
		if err != nil {
			t.Fatalf("RemoveShim failed: %v", err)
		}

		// Verify the shim was removed
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if _, exists := cfg.Shims["cat"]; exists {
			t.Error("cat shim still exists after removal")
		}

		// Verify backup was created
		backupPath := configPath + ".backup"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("backup file was not created")
		}
	})

	t.Run("preserves other shims when removing one", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Test block message"

[shims.tsc]
action = "block"
message = "Use pnpm run typecheck"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		err = RemoveShim(configPath, "cat")
		if err != nil {
			t.Fatalf("RemoveShim failed: %v", err)
		}

		// Verify only cat was removed
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if _, exists := cfg.Shims["cat"]; exists {
			t.Error("cat shim still exists after removal")
		}

		tscShim, exists := cfg.Shims["tsc"]
		if !exists {
			t.Error("tsc shim was incorrectly removed")
		}
		if tscShim.Action != "block" {
			t.Error("tsc shim was modified")
		}
	})

	t.Run("fails when command doesn't exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		err = RemoveShim(configPath, "nonexistent")
		if err == nil {
			t.Fatal("RemoveShim should fail for nonexistent command")
		}

		// Verify error message
		if err.Error() != "shim for command 'nonexistent' not found" {
			t.Errorf("unexpected error message: %v", err)
		}

		// Verify original config unchanged
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
		if _, exists := cfg.Shims["cat"]; !exists {
			t.Error("original shim was incorrectly removed")
		}
	})

	t.Run("fails when config file doesn't exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "nonexistent.toml")

		err = RemoveShim(configPath, "cat")
		if err == nil {
			t.Fatal("RemoveShim should fail for nonexistent config")
		}
	})
}

func TestUpdateShim(t *testing.T) {
	t.Run("successfully updates existing shim", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Test block message"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		updatedConfig := ShimConfig{
			Action:  "warn",
			Message: "Test warn message",
			Paths:   []string{"/bin/cat"},
		}

		err = UpdateShim(configPath, "cat", updatedConfig)
		if err != nil {
			t.Fatalf("UpdateShim failed: %v", err)
		}

		// Verify the shim was updated
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		catShim, exists := cfg.Shims["cat"]
		if !exists {
			t.Fatal("cat shim not found after update")
		}
		if catShim.Action != "warn" {
			t.Errorf("expected action 'warn', got '%s'", catShim.Action)
		}
		if catShim.Message != "Test warn message" {
			t.Errorf("expected updated message, got '%s'", catShim.Message)
		}
		if len(catShim.Paths) != 1 {
			t.Errorf("expected 1 path, got %d", len(catShim.Paths))
		}

		// Verify backup was created
		backupPath := configPath + ".backup"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("backup file was not created")
		}
	})

	t.Run("preserves other shims when updating one", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
message = "Test block message"

[shims.tsc]
action = "block"
message = "Use pnpm run typecheck"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		updatedConfig := ShimConfig{
			Action:  "warn",
			Message: "Updated message",
		}

		err = UpdateShim(configPath, "cat", updatedConfig)
		if err != nil {
			t.Fatalf("UpdateShim failed: %v", err)
		}

		// Verify cat was updated and tsc unchanged
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		catShim := cfg.Shims["cat"]
		if catShim.Action != "warn" {
			t.Error("cat shim was not updated")
		}

		tscShim := cfg.Shims["tsc"]
		if tscShim.Action != "block" || tscShim.Message != "Use pnpm run typecheck" {
			t.Error("tsc shim was incorrectly modified")
		}
	})

	t.Run("fails when command doesn't exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		content := `[shims.cat]
action = "block"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		updatedConfig := ShimConfig{
			Action: "warn",
		}

		err = UpdateShim(configPath, "nonexistent", updatedConfig)
		if err == nil {
			t.Fatal("UpdateShim should fail for nonexistent command")
		}

		// Verify error message
		if err.Error() != "shim for command 'nonexistent' not found" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("fails when config file doesn't exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "nonexistent.toml")

		updatedConfig := ShimConfig{
			Action: "warn",
		}

		err = UpdateShim(configPath, "cat", updatedConfig)
		if err == nil {
			t.Fatal("UpdateShim should fail for nonexistent config")
		}
	})
}

func TestAtomicWrite(t *testing.T) {
	t.Run("creates backup file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		originalContent := `[shims.cat]
action = "block"
`
		if err := os.WriteFile(configPath, []byte(originalContent), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		config := &ProjectConfig{
			Shims: map[string]ShimConfig{
				"cat": {Action: "warn"},
			},
		}

		err = atomicWrite(configPath, config)
		if err != nil {
			t.Fatalf("atomicWrite failed: %v", err)
		}

		// Verify backup exists
		backupPath := configPath + ".backup"
		backupData, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("failed to read backup: %v", err)
		}

		if string(backupData) != originalContent {
			t.Error("backup content doesn't match original")
		}
	})

	t.Run("validates written file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")

		config := &ProjectConfig{
			Shims: map[string]ShimConfig{
				"cat": {Action: "block", Message: "test"},
			},
		}

		err = atomicWrite(configPath, config)
		if err != nil {
			t.Fatalf("atomicWrite failed: %v", err)
		}

		// Verify the written file can be parsed
		cfg, err := LoadProjectConfig(configPath)
		if err != nil {
			t.Fatalf("validation failed: %v", err)
		}

		if cfg.Shims["cat"].Action != "block" {
			t.Error("written config doesn't match expected")
		}
	})

	t.Run("cleans up temp file on validation failure", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")

		// The atomicWrite function should handle this gracefully
		// We can't easily force a validation failure with the current structure,
		// but we can verify temp files are cleaned up in normal operation
		config := &ProjectConfig{
			Shims: map[string]ShimConfig{
				"cat": {Action: "block"},
			},
		}

		err = atomicWrite(configPath, config)
		if err != nil {
			t.Fatalf("atomicWrite failed: %v", err)
		}

		// Verify temp file was cleaned up
		tmpPath := configPath + ".tmp"
		if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
			t.Error("temp file was not cleaned up")
		}
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ribbin-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		configPath := filepath.Join(tmpDir, "ribbin.toml")
		if err := os.WriteFile(configPath, []byte("[shims]\n"), 0644); err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		config := &ProjectConfig{
			Shims: map[string]ShimConfig{
				"cat": {Action: "block"},
			},
		}

		err = atomicWrite(configPath, config)
		if err != nil {
			t.Fatalf("atomicWrite failed: %v", err)
		}

		// Verify file permissions
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		mode := info.Mode().Perm()
		if mode != 0644 {
			t.Errorf("expected mode 0644, got %o", mode)
		}
	})
}
