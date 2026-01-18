package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/happycollision/ribbin/internal/security"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View and query security audit log",
	Long: `View and query the security audit log.

The audit log tracks all security-relevant operations including:
- Shim installations and uninstallations
- Bypass usage (RIBBIN_BYPASS=1)
- Security violations (path traversal, forbidden directories)
- Privileged operations (running as root)
- Configuration file loads

Examples:
  ribbin audit show                    Show recent audit events
  ribbin audit show --since 7d         Show events from last 7 days
  ribbin audit show --type bypass.used Filter by event type
  ribbin audit summary                 Show summary statistics
`,
}

var auditShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show recent audit events",
	Long: `Show recent audit events from the security log.

The audit log is stored in JSONL format at:
  ~/.local/state/ribbin/audit.log (or $XDG_STATE_HOME/ribbin/audit.log)

Event types include:
  shim.install          - Shim installed
  shim.uninstall        - Shim uninstalled
  bypass.used           - RIBBIN_BYPASS=1 used
  security.violation    - Security policy violated
  privileged.operation  - Operation performed as root
  config.load           - Configuration loaded
  registry.update       - Registry updated

Examples:
  ribbin audit show                          Show last 50 events
  ribbin audit show --since 24h              Show events from last 24 hours
  ribbin audit show --since 7d               Show events from last 7 days
  ribbin audit show --type bypass.used       Show only bypass events
  ribbin audit show --limit 100              Show last 100 events
`,
	RunE: runAuditShow,
}

var auditSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show audit summary statistics",
	Long: `Show summary statistics from the security audit log.

Displays counts for:
- Total events
- Successful operations
- Failed operations
- Elevated (root) operations
- Security violations
- Bypass usages

Statistics are calculated for the last 30 days by default.
`,
	RunE: runAuditSummary,
}

var (
	auditSince     string
	auditEventType string
	auditLimit     int
)

func init() {
	auditShowCmd.Flags().StringVar(&auditSince, "since", "24h", "Show events since duration (e.g., 24h, 7d, 30d)")
	auditShowCmd.Flags().StringVar(&auditEventType, "type", "", "Filter by event type")
	auditShowCmd.Flags().IntVar(&auditLimit, "limit", 50, "Limit number of events")

	auditCmd.AddCommand(auditShowCmd)
	auditCmd.AddCommand(auditSummaryCmd)
	rootCmd.AddCommand(auditCmd)
}

func runAuditShow(cmd *cobra.Command, args []string) error {
	// Parse since duration
	duration, err := time.ParseDuration(auditSince)
	if err != nil {
		return fmt.Errorf("invalid duration '%s': %w\nValid examples: 1h, 24h, 7d, 30d", auditSince, err)
	}
	startTime := time.Now().Add(-duration)

	// Query events
	query := &security.AuditQuery{
		StartTime: &startTime,
		EventType: auditEventType,
	}
	events, err := security.QueryAuditLog(query)
	if err != nil {
		return fmt.Errorf("cannot query audit log: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("No audit events found")
		return nil
	}

	// Limit results
	if len(events) > auditLimit {
		events = events[len(events)-auditLimit:]
	}

	// Display events
	fmt.Printf("Showing %d audit events (since %s ago):\n\n", len(events), auditSince)
	for _, event := range events {
		// Format timestamp
		timestamp := event.Timestamp.Format("2006-01-02 15:04:05")

		// Build status indicator
		statusIcon := "✓"
		if !event.Success {
			statusIcon = "✗"
		}

		// Build event line
		fmt.Printf("[%s] %s %s", timestamp, statusIcon, event.Event)

		// Add path/binary if present
		if event.Binary != "" {
			fmt.Printf(": %s", event.Binary)
		} else if event.Path != "" {
			fmt.Printf(": %s", event.Path)
		}

		// Add elevated indicator
		if event.Elevated {
			fmt.Printf(" [ROOT]")
		}

		// Add error if failed
		if !event.Success && event.Error != "" {
			fmt.Printf("\n    Error: %s", event.Error)
		}

		// Add details if present
		if len(event.Details) > 0 {
			fmt.Printf("\n    Details: ")
			first := true
			for k, v := range event.Details {
				if !first {
					fmt.Printf(", ")
				}
				fmt.Printf("%s=%s", k, v)
				first = false
			}
		}

		fmt.Println()
	}

	// Show log path
	logPath, err := security.GetAuditLogPath()
	if err == nil {
		fmt.Printf("\nAudit log: %s\n", logPath)
	}

	return nil
}

func runAuditSummary(cmd *cobra.Command, args []string) error {
	// Get summary for last 30 days
	since := time.Now().AddDate(0, 0, -30)
	summary, err := security.GetAuditSummary(&since)
	if err != nil {
		return fmt.Errorf("cannot generate audit summary: %w", err)
	}

	// Display summary
	fmt.Println("Audit Summary (last 30 days):")
	fmt.Println()
	fmt.Printf("  Total Events:         %d\n", summary.TotalEvents)
	fmt.Printf("  Successful Ops:       %d\n", summary.SuccessfulOps)
	fmt.Printf("  Failed Ops:           %d\n", summary.FailedOps)
	fmt.Printf("  Elevated Ops:         %d\n", summary.ElevatedOps)
	fmt.Printf("  Security Violations:  %d\n", summary.SecurityViolations)
	fmt.Printf("  Bypass Usages:        %d\n", summary.BypassUsages)
	fmt.Println()

	// Show warnings if needed
	if summary.SecurityViolations > 0 {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: %d security violations detected\n", summary.SecurityViolations)
		fmt.Fprintf(os.Stderr, "   Run 'ribbin audit show --type security.violation' for details\n\n")
	}

	if summary.BypassUsages > 10 {
		fmt.Fprintf(os.Stderr, "ℹ️  Note: RIBBIN_BYPASS was used %d times\n", summary.BypassUsages)
		fmt.Fprintf(os.Stderr, "   Run 'ribbin audit show --type bypass.used' for details\n\n")
	}

	// Show log path
	logPath, err := security.GetAuditLogPath()
	if err == nil {
		fmt.Printf("Audit log: %s\n", logPath)
	}

	return nil
}
