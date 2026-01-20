package shim

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestVerboseLog(t *testing.T) {
	tests := []struct {
		name     string
		verbose  string
		format   string
		args     []interface{}
		wantLog  bool
		contains string
	}{
		{
			name:    "verbose disabled - no output",
			verbose: "",
			format:  "test message",
			wantLog: false,
		},
		{
			name:    "verbose disabled with 0 - no output",
			verbose: "0",
			format:  "test message",
			wantLog: false,
		},
		{
			name:     "verbose enabled - logs message",
			verbose:  "1",
			format:   "test message",
			wantLog:  true,
			contains: "[ribbin] test message",
		},
		{
			name:     "verbose enabled - logs with format args",
			verbose:  "1",
			format:   "cmd %s action %s",
			args:     []interface{}{"cat", "BLOCKED"},
			wantLog:  true,
			contains: "[ribbin] cmd cat action BLOCKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Set env
			if tt.verbose != "" {
				os.Setenv("RIBBIN_VERBOSE", tt.verbose)
			} else {
				os.Unsetenv("RIBBIN_VERBOSE")
			}

			// Call verboseLog
			verboseLog(tt.format, tt.args...)

			// Restore stderr and read output
			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			if tt.wantLog {
				if !strings.Contains(output, tt.contains) {
					t.Errorf("expected output to contain %q, got %q", tt.contains, output)
				}
			} else {
				if output != "" {
					t.Errorf("expected no output, got %q", output)
				}
			}

			// Cleanup
			os.Unsetenv("RIBBIN_VERBOSE")
		})
	}
}

func TestVerboseLogDecision(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		action   string
		reason   string
		contains string
	}{
		{
			name:     "blocked decision",
			cmd:      "tsc",
			action:   "BLOCKED",
			reason:   "Use 'pnpm run typecheck' instead",
			contains: "[ribbin] tsc -> BLOCKED: Use 'pnpm run typecheck' instead",
		},
		{
			name:     "pass decision",
			cmd:      "node",
			action:   "PASS",
			reason:   "no shim configured",
			contains: "[ribbin] node -> PASS: no shim configured",
		},
		{
			name:     "redirect decision",
			cmd:      "cat",
			action:   "REDIRECT",
			reason:   "bat",
			contains: "[ribbin] cat -> REDIRECT: bat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Enable verbose
			os.Setenv("RIBBIN_VERBOSE", "1")

			// Call verboseLogDecision
			verboseLogDecision(tt.cmd, tt.action, tt.reason)

			// Restore stderr and read output
			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain %q, got %q", tt.contains, output)
			}

			// Cleanup
			os.Unsetenv("RIBBIN_VERBOSE")
		})
	}
}
