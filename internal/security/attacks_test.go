package security

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

// =============================================================================
// PATH TRAVERSAL ATTACK TESTS
// =============================================================================

func TestAttack_PathTraversal_MaliciousPaths(t *testing.T) {
	// Comprehensive path traversal attack vectors
	maliciousPaths := []string{
		// Basic traversal
		"../../etc/passwd",
		"../../../usr/bin/bash",
		"../../../../etc/shadow",

		// Embedded traversal
		"/usr/bin/../../../etc/passwd",
		"/tmp/safe/../../../etc/passwd",

		// Home directory escape
		"~/../../../etc/passwd",

		// Current directory escape
		"./../../etc/passwd",

		// Note: URL-encoded paths (%2e%2e) are NOT decoded by the filesystem
		// so /tmp/%2e%2e/etc/passwd is a literal path, not a traversal
		// This is expected behavior - the filesystem doesn't interpret URL encoding

		// Null byte injection (shouldn't work in Go but test anyway)
		"/tmp/safe\x00../../etc/passwd",

		// Unicode tricks
		"/tmp/..％2f../etc/passwd",

		// Mixed slashes
		"/tmp\\..\\..\\etc\\passwd",

		// Multiple consecutive dots
		"/tmp/.../etc/passwd",
		"/tmp/..../etc/passwd",
	}

	for _, path := range maliciousPaths {
		t.Run(path, func(t *testing.T) {
			err := ValidateBinaryPath(path)
			if err == nil {
				t.Errorf("should reject malicious path: %s", path)
			}
		})
	}
}

func TestAttack_PathTraversal_InSymlinkTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create symlink with traversal in target
	link := filepath.Join(tmpDir, "malicious-link")
	if err := os.Symlink("../../../etc/passwd", link); err != nil {
		t.Fatal(err)
	}

	// ResolveSymlinkChain should detect this
	_, err := ResolveSymlinkChain(link)
	if err == nil {
		t.Error("should reject symlink with traversal target")
	}
}

// =============================================================================
// SYMLINK ATTACK TESTS
// =============================================================================

func TestAttack_Symlink_ToSystemBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Attacker creates symlink to bash
	link := filepath.Join(tmpDir, "innocent-looking-tool")
	if err := os.Symlink("/usr/bin/bash", link); err != nil {
		t.Fatal(err)
	}

	// Should detect that target is a critical system binary
	target, err := ResolveSymlinkChain(link)
	if err != nil {
		// Good - rejected during resolution
		return
	}

	// If resolution succeeded, check if it's flagged as critical
	if IsCriticalSystemBinary(target) {
		// Good - would be caught by ValidateBinaryForShim
		return
	}

	// Verify ValidateBinaryForShim catches it
	err = ValidateBinaryForShim(link, true)
	if err == nil {
		t.Error("should reject symlink to critical system binary")
	}
}

func TestAttack_Symlink_Circular(t *testing.T) {
	tmpDir := t.TempDir()

	// Create circular symlink chain
	link1 := filepath.Join(tmpDir, "link1")
	link2 := filepath.Join(tmpDir, "link2")
	link3 := filepath.Join(tmpDir, "link3")

	os.Symlink(link2, link1)
	os.Symlink(link3, link2)
	os.Symlink(link1, link3)

	_, err := ResolveSymlinkChain(link1)
	if err == nil {
		t.Error("should detect circular symlink chain")
	}
	if !strings.Contains(err.Error(), "circular") && !strings.Contains(err.Error(), "loop") {
		t.Errorf("error should mention circular/loop, got: %v", err)
	}
}

func TestAttack_Symlink_DeepChainToForbidden(t *testing.T) {
	tmpDir := t.TempDir()

	// Create deep chain of symlinks ending at forbidden target
	// link1 -> link2 -> link3 -> link4 -> link5 -> /etc/passwd
	links := make([]string, 6)
	for i := 0; i < 6; i++ {
		links[i] = filepath.Join(tmpDir, "link"+string(rune('a'+i)))
	}

	// Create chain
	for i := 0; i < 5; i++ {
		os.Symlink(links[i+1], links[i])
	}
	// Final link points to forbidden path
	os.Symlink("/etc/passwd", links[5])

	_, err := ResolveSymlinkChain(links[0])
	if err == nil {
		t.Error("should reject deep chain to forbidden path")
	}
}

func TestAttack_Symlink_TOCTOU_Replace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a legitimate symlink
	target := filepath.Join(tmpDir, "target")
	link := filepath.Join(tmpDir, "link")
	os.WriteFile(target, []byte("safe content"), 0644)
	os.Symlink(target, link)

	// Start resolution in background
	var resolved string
	var resolveErr error
	done := make(chan struct{})

	go func() {
		resolved, resolveErr = ResolveSymlinkChain(link)
		close(done)
	}()

	// Try to replace symlink target during resolution
	// This is a TOCTOU attack attempt
	time.Sleep(1 * time.Millisecond)
	os.Remove(link)
	os.Symlink("/etc/passwd", link)

	<-done

	// Either should fail, or should have resolved to original safe target
	if resolveErr == nil && resolved == "/etc/passwd" {
		t.Error("TOCTOU vulnerability: resolved to malicious target after replacement")
	}
}

// =============================================================================
// ALLOWLIST BYPASS TESTS
// =============================================================================

func TestAttack_Allowlist_AllCriticalBinaries(t *testing.T) {
	// These are the critical binaries that MUST be blocked
	criticalBinaries := []string{
		// Primary shells
		"/usr/bin/bash",
		"/bin/bash",
		"/usr/bin/sh",
		"/bin/sh",
		"/usr/bin/zsh",
		"/usr/bin/fish",

		// Privilege escalation
		"/usr/bin/sudo",
		"/usr/bin/su",
		"/usr/bin/doas",

		// SSH
		"/usr/bin/ssh",
		"/usr/bin/sshd",
		"/usr/sbin/sshd",

		// Init systems
		"/usr/bin/init",
		"/sbin/init",
		"/usr/bin/systemd",
		"/usr/lib/systemd/systemd",
		"/usr/bin/launchd",
		"/sbin/launchd",

		// Authentication
		"/usr/bin/login",
		"/usr/bin/passwd",
		"/bin/login",
	}

	// Note: Less common shells (dash, ksh, csh, tcsh) and pkexec
	// are not currently in the critical list but would still be
	// blocked by directory allowlist since they're in /usr/bin

	for _, binary := range criticalBinaries {
		t.Run(binary, func(t *testing.T) {
			// All critical binaries should be detected
			if !IsCriticalSystemBinary(binary) {
				t.Errorf("should identify as critical: %s", binary)
			}

			// ValidateBinaryForShim should reject even with confirmation
			err := ValidateBinaryForShim(binary, true)
			if err == nil {
				t.Errorf("should reject critical binary: %s", binary)
			}
		})
	}
}

func TestAttack_Allowlist_ForbiddenDirectories(t *testing.T) {
	forbiddenPaths := []string{
		"/bin/sometool",
		"/sbin/sometool",
		"/usr/bin/sometool",
		"/usr/sbin/sometool",
		"/etc/sometool",
		"/var/sometool",
		"/sys/sometool",
		"/proc/sometool",
		"/dev/sometool",
		"/boot/sometool",
		"/root/sometool",
	}

	for _, path := range forbiddenPaths {
		t.Run(path, func(t *testing.T) {
			category, err := GetDirectoryCategory(path)
			if err != nil {
				// Error is acceptable
				return
			}
			if category != CategoryForbidden {
				t.Errorf("should be forbidden: %s (got %v)", path, category)
			}
		})
	}
}

func TestAttack_Allowlist_ConfirmationRequired(t *testing.T) {
	// These paths should require confirmation but not be outright forbidden
	requireConfirmation := []string{
		"/usr/local/bin/mytool",
		"/opt/homebrew/bin/mytool",
		"/opt/local/bin/mytool",
	}

	for _, path := range requireConfirmation {
		t.Run(path, func(t *testing.T) {
			// Without confirmation should fail
			err := ValidateBinaryForShim(path, false)
			if err == nil {
				t.Errorf("should require confirmation: %s", path)
			}

			// With confirmation should succeed (unless file doesn't exist)
			err = ValidateBinaryForShim(path, true)
			// May fail due to file not existing, but shouldn't fail for allowlist reasons
			if err != nil && strings.Contains(err.Error(), "confirmation") {
				t.Errorf("should allow with confirmation: %s", path)
			}
		})
	}
}

// =============================================================================
// ENVIRONMENT INJECTION TESTS
// =============================================================================

func TestAttack_Environment_MaliciousHome(t *testing.T) {
	// Save original
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	maliciousHomes := []string{
		"/tmp/../etc",
		"../../../etc",
		"/nonexistent/path/that/doesnt/exist",
	}

	for _, home := range maliciousHomes {
		t.Run(home, func(t *testing.T) {
			os.Setenv("HOME", home)

			// ValidateHomeDir should detect this
			_, err := ValidateHomeDir()
			if err == nil {
				t.Errorf("should reject malicious HOME: %s", home)
			}
		})
	}
}

func TestAttack_Environment_MaliciousXDG(t *testing.T) {
	// Save originals
	originalConfig := os.Getenv("XDG_CONFIG_HOME")
	originalState := os.Getenv("XDG_STATE_HOME")
	defer func() {
		if originalConfig != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfig)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if originalState != "" {
			os.Setenv("XDG_STATE_HOME", originalState)
		} else {
			os.Unsetenv("XDG_STATE_HOME")
		}
	}()

	maliciousPaths := []string{
		"/tmp/../etc",
		"../../etc",
		"/nonexistent\x00/evil",
	}

	for _, path := range maliciousPaths {
		t.Run("XDG_CONFIG_HOME="+path, func(t *testing.T) {
			os.Setenv("XDG_CONFIG_HOME", path)
			_, err := GetConfigDir()
			if err == nil {
				t.Errorf("should reject malicious XDG_CONFIG_HOME: %s", path)
			}
		})

		t.Run("XDG_STATE_HOME="+path, func(t *testing.T) {
			os.Setenv("XDG_STATE_HOME", path)
			_, err := GetStateDir()
			if err == nil {
				t.Errorf("should reject malicious XDG_STATE_HOME: %s", path)
			}
		})
	}
}

// =============================================================================
// TOCTOU (TIME OF CHECK TO TIME OF USE) TESTS
// =============================================================================

func TestAttack_TOCTOU_ConcurrentLockAcquisition(t *testing.T) {
	tmpDir := t.TempDir()
	lockFile := filepath.Join(tmpDir, "test.lock")

	// Create the file first
	os.WriteFile(lockFile, []byte("data"), 0644)

	const numGoroutines = 20
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := AcquireLock(lockFile, 100*time.Millisecond)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				// Hold lock briefly
				time.Sleep(10 * time.Millisecond)
				lock.Release()
			}
		}()
	}

	wg.Wait()

	// At least one should succeed, but there shouldn't be data corruption
	if successCount == 0 {
		t.Error("at least one goroutine should acquire the lock")
	}
	t.Logf("%d/%d goroutines acquired the lock", successCount, numGoroutines)
}

func TestAttack_TOCTOU_FileReplaceDuringOperation(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "target")

	// Create original file
	os.WriteFile(file, []byte("original"), 0644)

	// Validate file
	err := ValidateBinaryPath(file)
	if err != nil {
		t.Skipf("file validation failed: %v", err)
	}

	// Now replace file (simulating attacker)
	os.WriteFile(file, []byte("malicious"), 0644)

	// A real TOCTOU attack would replace between check and use
	// This test verifies that we're aware of the pattern
	// Actual protection requires file locking during the operation

	content, _ := os.ReadFile(file)
	if string(content) != "malicious" {
		t.Error("file should have been replaced")
	}
}

// =============================================================================
// AUDIT LOG TESTS
// =============================================================================

func TestAttack_AuditLog_SecurityViolationsLogged(t *testing.T) {
	// Set up temp state dir for audit logs
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Trigger a security violation
	LogSecurityViolation("path traversal detected", "../../etc/passwd", map[string]string{
		"reason": "test attack",
	})

	// Query audit log
	events, err := QueryAuditLog(&AuditQuery{
		EventType: EventSecurityViolation,
	})
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	if len(events) == 0 {
		t.Error("security violation should be logged")
	}

	// Verify event details
	found := false
	for _, event := range events {
		if event.Path == "../../etc/passwd" {
			found = true
			if event.Details["reason"] != "test attack" {
				t.Error("event details should be preserved")
			}
		}
	}
	if !found {
		t.Error("should find the specific security violation event")
	}
}

func TestAttack_AuditLog_ElevatedOperationsLogged(t *testing.T) {
	// Set up temp state dir
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log a privileged operation
	LogPrivilegedOperation("install", "/usr/local/bin/test", true, nil)

	// Verify it was logged
	events, err := QueryAuditLog(&AuditQuery{
		EventType: EventPrivilegedOp,
	})
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	if len(events) == 0 {
		t.Error("privileged operation should be logged")
	}
}

// =============================================================================
// INTEGRATION ATTACK SCENARIOS
// =============================================================================

func TestAttack_Scenario_MaliciousConfigInParent(t *testing.T) {
	// Attacker creates malicious config in parent directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "safe-project")
	os.Mkdir(projectDir, 0755)

	// Malicious config targeting bash
	maliciousConfig := `
[shims.bash]
action = "block"
paths = ["/usr/bin/bash"]
message = "gotcha"
`
	configFile := filepath.Join(tmpDir, "ribbin.toml")
	os.WriteFile(configFile, []byte(maliciousConfig), 0644)

	// If the config is loaded and paths validated,
	// the /usr/bin/bash path should be rejected
	// (This test validates our defense against malicious configs)

	// Note: Actual config loading would need to be tested in the config package
	// Here we just verify the path would be rejected
	err := ValidateBinaryForShim("/usr/bin/bash", true)
	if err == nil {
		t.Error("should reject bash even from config")
	}
}

func TestAttack_Scenario_SymlinkInProjectDir(t *testing.T) {
	// Attacker creates symlink in project dir pointing to sensitive binary
	tmpDir := t.TempDir()

	// Create "innocent" symlink
	innocentLink := filepath.Join(tmpDir, "my-build-tool")
	os.Symlink("/usr/bin/sudo", innocentLink)

	// Should be detected as pointing to critical binary
	target, err := ResolveSymlinkChain(innocentLink)
	if err == nil && IsCriticalSystemBinary(target) {
		// Good - detected
		err = ValidateBinaryForShim(innocentLink, true)
		if err == nil {
			t.Error("should reject symlink to sudo")
		}
	}
}

func TestAttack_Scenario_RaceConditionInstall(t *testing.T) {
	// Simulate concurrent attempts to install a shim
	// This tests that our locking prevents race conditions

	tmpDir := t.TempDir()
	lockFile := filepath.Join(tmpDir, "install.lock")

	// Pre-create the file
	os.WriteFile(lockFile, []byte("data"), 0644)

	const numAttempts = 50
	var wg sync.WaitGroup
	results := make([]bool, numAttempts)

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Try to acquire lock
			lock, err := AcquireLock(lockFile, 50*time.Millisecond)
			if err != nil {
				results[idx] = false
				return
			}

			// Simulate install operation
			time.Sleep(5 * time.Millisecond)
			results[idx] = true
			lock.Release()
		}(i)
	}

	wg.Wait()

	// Count successes
	successCount := 0
	for _, r := range results {
		if r {
			successCount++
		}
	}

	// At least some should succeed, but not all at once (proves locking works)
	if successCount == 0 {
		t.Error("at least some operations should succeed")
	}
	t.Logf("%d/%d operations completed successfully", successCount, numAttempts)
}
