//go:build !production

// Package testsafety provides a safety check that prevents tests from running
// directly on the host machine.
//
// IMPORTANT: All test files must import this package to ensure the safety check runs:
//
//	import _ "github.com/happycollision/ribbin/internal/testsafety"
//
// This prevents tests from accidentally running on the host machine where they
// could modify system binaries.
package testsafety

import (
	"fmt"
	"os"
	"sync"
)

var safetyCheckOnce sync.Once

// RequireSafeEnvironment checks that tests are running in a safe environment
// (Docker or with explicit opt-in). Call this at the start of any test that
// manipulates the filesystem.
//
// This is automatically called via init() when testutil is imported.
func RequireSafeEnvironment() {
	safetyCheckOnce.Do(func() {
		checkSafeEnvironment()
	})
}

func checkSafeEnvironment() {
	// Allow explicit bypass for debugging
	if os.Getenv("RIBBIN_DANGEROUSLY_ALLOW_HOST_TESTS") == "1" {
		return
	}

	// Check if running in Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return
	}

	// Check for container environment variable (podman, etc.)
	if os.Getenv("container") != "" {
		return
	}

	// Check if running inside the Docker test image (has /app directory)
	if _, err := os.Stat("/app/go.mod"); err == nil {
		return
	}

	fmt.Fprint(os.Stderr, `
═══════════════════════════════════════════════════════════════════════════════
  RIBBIN TEST SAFETY CHECK FAILED
═══════════════════════════════════════════════════════════════════════════════

  Tests must be run inside Docker to prevent accidental modification of
  binaries and system files on your machine.

  To run tests safely:
    make test

  To bypass this check (for debugging only):
    RIBBIN_DANGEROUSLY_ALLOW_HOST_TESTS=1 go test ./...

═══════════════════════════════════════════════════════════════════════════════
`)
	os.Exit(1)
}

func init() {
	RequireSafeEnvironment()
}
