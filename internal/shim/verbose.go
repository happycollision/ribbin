package shim

import (
	"fmt"
	"os"
)

// verboseLog writes a message to stderr if RIBBIN_VERBOSE=1 is set.
// Format: [ribbin] command -> ACTION: reason
func verboseLog(format string, args ...interface{}) {
	if os.Getenv("RIBBIN_VERBOSE") != "1" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[ribbin] %s\n", msg)
}

// verboseLogDecision logs a shim decision in the standard format.
// action should be one of: BLOCKED, PASS, REDIRECT
func verboseLogDecision(cmd, action, reason string) {
	verboseLog("%s -> %s: %s", cmd, action, reason)
}
