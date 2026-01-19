package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuditEvent represents a security-relevant event
type AuditEvent struct {
	Timestamp time.Time         `json:"timestamp"`
	Event     string            `json:"event"`   // Event type
	User      string            `json:"user"`    // Username
	UID       int               `json:"uid"`     // User ID
	Elevated  bool              `json:"elevated"` // Running as root?
	Binary    string            `json:"binary,omitempty"`
	Path      string            `json:"path,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

// Event types
const (
	EventShimInstall       = "shim.install"
	EventShimUninstall     = "shim.uninstall"
	EventBypassUsed        = "bypass.used"
	EventSecurityViolation = "security.violation"
	EventPrivilegedOp      = "privileged.operation"
	EventConfigLoad        = "config.load"
	EventRegistryUpdate    = "registry.update"
)

// GetAuditLogPath returns the path to the audit log.
// It uses validated environment variables to prevent injection attacks.
func GetAuditLogPath() (string, error) {
	stateDir, err := EnsureStateDir()
	if err != nil {
		return "", fmt.Errorf("cannot get state directory: %w", err)
	}

	return filepath.Join(stateDir, "audit.log"), nil
}

// LogEvent writes an audit event to the log
func LogEvent(event *AuditEvent) error {
	// Get log path
	logPath, err := GetAuditLogPath()
	if err != nil {
		// Don't fail the operation if we can't log
		fmt.Fprintf(os.Stderr, "Warning: cannot get audit log path: %v\n", err)
		return nil
	}

	// Fill in automatic fields
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.User == "" {
		event.User = os.Getenv("USER")
	}
	event.UID = os.Getuid()
	event.Elevated = os.Getuid() == 0

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot marshal audit event: %v\n", err)
		return nil
	}

	// Append to log file (create if doesn't exist)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot open audit log: %v\n", err)
		return nil
	}
	defer f.Close()

	// Write event (newline-delimited JSON)
	if _, err := f.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot write audit log: %v\n", err)
		return nil
	}

	return nil
}

// Convenience functions for common events

// LogShimInstall logs a shim installation attempt
func LogShimInstall(binary string, success bool, err error) {
	event := &AuditEvent{
		Event:   EventShimInstall,
		Binary:  binary,
		Path:    binary,
		Success: success,
	}
	if err != nil {
		event.Error = err.Error()
	}
	LogEvent(event)
}

// LogShimUninstall logs a shim uninstallation attempt
func LogShimUninstall(binary string, success bool, err error) {
	event := &AuditEvent{
		Event:   EventShimUninstall,
		Binary:  binary,
		Path:    binary,
		Success: success,
	}
	if err != nil {
		event.Error = err.Error()
	}
	LogEvent(event)
}

// LogBypassUsage logs when RIBBIN_BYPASS is used
func LogBypassUsage(command string, pid int) {
	event := &AuditEvent{
		Event:   EventBypassUsed,
		Binary:  command,
		Success: true,
		Details: map[string]string{
			"pid": fmt.Sprintf("%d", pid),
		},
	}
	LogEvent(event)
}

// LogSecurityViolation logs a security policy violation
func LogSecurityViolation(violation, path string, details map[string]string) {
	event := &AuditEvent{
		Event:   EventSecurityViolation,
		Path:    path,
		Success: false,
		Error:   violation,
		Details: details,
	}
	LogEvent(event)
}

// LogPrivilegedOperation logs a privileged operation
func LogPrivilegedOperation(operation, path string, success bool, err error) {
	event := &AuditEvent{
		Event:   EventPrivilegedOp,
		Path:    path,
		Success: success,
		Details: map[string]string{
			"operation": operation,
		},
	}
	if err != nil {
		event.Error = err.Error()
	}
	LogEvent(event)
}

// LogConfigLoad logs configuration file loading
func LogConfigLoad(configPath string, success bool, err error) {
	event := &AuditEvent{
		Event:   EventConfigLoad,
		Path:    configPath,
		Success: success,
	}
	if err != nil {
		event.Error = err.Error()
	}
	LogEvent(event)
}

// LogRegistryUpdate logs registry updates
func LogRegistryUpdate(operation string, success bool, err error) {
	event := &AuditEvent{
		Event:   EventRegistryUpdate,
		Success: success,
		Details: map[string]string{
			"operation": operation,
		},
	}
	if err != nil {
		event.Error = err.Error()
	}
	LogEvent(event)
}

// Query and Analysis Functions

// AuditQuery filters for querying audit events
type AuditQuery struct {
	StartTime *time.Time
	EndTime   *time.Time
	EventType string
	User      string
	Binary    string
	Elevated  *bool
	Success   *bool
}

// QueryAuditLog reads and filters audit events
func QueryAuditLog(query *AuditQuery) ([]*AuditEvent, error) {
	logPath, err := GetAuditLogPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*AuditEvent{}, nil
		}
		return nil, err
	}

	var events []*AuditEvent
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed lines
		}

		// Apply filters
		if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
			continue
		}
		if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
			continue
		}
		if query.EventType != "" && event.Event != query.EventType {
			continue
		}
		if query.User != "" && event.User != query.User {
			continue
		}
		if query.Binary != "" && event.Binary != query.Binary {
			continue
		}
		if query.Elevated != nil && event.Elevated != *query.Elevated {
			continue
		}
		if query.Success != nil && event.Success != *query.Success {
			continue
		}

		events = append(events, &event)
	}

	return events, nil
}

// AuditSummary provides statistics about audit events
type AuditSummary struct {
	TotalEvents        int
	SuccessfulOps      int
	FailedOps          int
	ElevatedOps        int
	SecurityViolations int
	BypassUsages       int
}

// GetAuditSummary provides statistics about audit events
func GetAuditSummary(since *time.Time) (*AuditSummary, error) {
	query := &AuditQuery{StartTime: since}
	events, err := QueryAuditLog(query)
	if err != nil {
		return nil, err
	}

	summary := &AuditSummary{}
	for _, event := range events {
		summary.TotalEvents++
		if event.Success {
			summary.SuccessfulOps++
		} else {
			summary.FailedOps++
		}
		if event.Elevated {
			summary.ElevatedOps++
		}
		if event.Event == EventSecurityViolation {
			summary.SecurityViolations++
		}
		if event.Event == EventBypassUsed {
			summary.BypassUsages++
		}
	}

	return summary, nil
}
