package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestGetAuditLogPath(t *testing.T) {
	// Set test state directory
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	path, err := GetAuditLogPath()
	if err != nil {
		t.Fatalf("GetAuditLogPath() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "ribbin", "audit.log")
	if path != expectedPath {
		t.Errorf("GetAuditLogPath() = %v, want %v", path, expectedPath)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Errorf("audit directory not created: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("audit directory permissions = %o, want 0700", info.Mode().Perm())
	}
}

func TestLogEvent(t *testing.T) {
	// Set test state directory
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log an event
	event := &AuditEvent{
		Event:   EventShimInstall,
		Binary:  "/usr/local/bin/test",
		Path:    "/usr/local/bin/test",
		Success: true,
	}
	err := LogEvent(event)
	if err != nil {
		t.Fatalf("LogEvent() error = %v", err)
	}

	// Verify event was written
	logPath, _ := GetAuditLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("cannot read audit log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, EventShimInstall) {
		t.Errorf("audit log does not contain event type %s", EventShimInstall)
	}
	if !strings.Contains(content, "/usr/local/bin/test") {
		t.Errorf("audit log does not contain binary path")
	}

	// Verify automatic fields were set
	var logged AuditEvent
	err = json.Unmarshal(data, &logged)
	if err != nil {
		t.Fatalf("cannot unmarshal audit event: %v", err)
	}

	if logged.Timestamp.IsZero() {
		t.Error("timestamp not set automatically")
	}
	if logged.User == "" {
		t.Error("user not set automatically")
	}
	if logged.UID == 0 && os.Getuid() != 0 {
		t.Error("UID not set correctly")
	}
}

func TestLogEventMultiple(t *testing.T) {
	// Set test state directory
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log multiple events
	events := []*AuditEvent{
		{Event: EventShimInstall, Binary: "/bin/test1", Success: true},
		{Event: EventShimUninstall, Binary: "/bin/test2", Success: true},
		{Event: EventBypassUsed, Binary: "/bin/test3", Success: true},
	}

	for _, event := range events {
		if err := LogEvent(event); err != nil {
			t.Fatalf("LogEvent() error = %v", err)
		}
	}

	// Verify all events were written
	logPath, _ := GetAuditLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("cannot read audit log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 log lines, got %d", len(lines))
	}
}

func TestLogShimInstall(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log successful install
	LogShimInstall("/usr/local/bin/test", true, nil)

	// Verify
	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != EventShimInstall {
		t.Errorf("event type = %s, want %s", events[0].Event, EventShimInstall)
	}
	if !events[0].Success {
		t.Error("event should be marked as success")
	}
}

func TestLogShimInstallWithError(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log failed install
	testErr := os.ErrPermission
	LogShimInstall("/usr/local/bin/test", false, testErr)

	// Verify
	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Success {
		t.Error("event should be marked as failure")
	}
	if events[0].Error == "" {
		t.Error("error message should be set")
	}
}

func TestLogBypassUsage(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	LogBypassUsage("/bin/cat", 12345)

	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != EventBypassUsed {
		t.Errorf("event type = %s, want %s", events[0].Event, EventBypassUsed)
	}
	if events[0].Details["pid"] != "12345" {
		t.Errorf("pid detail = %s, want 12345", events[0].Details["pid"])
	}
}

func TestLogSecurityViolation(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	LogSecurityViolation("path traversal", "/tmp/../etc/passwd", map[string]string{
		"reason": "contains .. sequence",
	})

	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != EventSecurityViolation {
		t.Errorf("event type = %s, want %s", events[0].Event, EventSecurityViolation)
	}
	if events[0].Success {
		t.Error("security violation should be marked as failure")
	}
	if events[0].Details["reason"] != "contains .. sequence" {
		t.Error("details not properly logged")
	}
}

func TestQueryAuditLog(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log several events with delays to ensure different timestamps
	LogShimInstall("/bin/test1", true, nil)
	time.Sleep(10 * time.Millisecond)
	LogShimUninstall("/bin/test2", true, nil)
	time.Sleep(10 * time.Millisecond)
	LogBypassUsage("/bin/test3", 1234)

	// Query all events
	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Query by event type
	query := &AuditQuery{EventType: EventShimInstall}
	events, err = QueryAuditLog(query)
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Binary != "/bin/test1" {
		t.Errorf("binary = %s, want /bin/test1", events[0].Binary)
	}

	// Query by time range
	midTime := time.Now().Add(-5 * time.Millisecond)
	query = &AuditQuery{StartTime: &midTime}
	events, err = QueryAuditLog(query)
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) < 1 {
		t.Error("expected at least 1 event after midTime")
	}
}

func TestQueryAuditLogNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Query when no log file exists
	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestQueryAuditLogByUser(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log events with explicit user
	currentUser := os.Getenv("USER")
	event := &AuditEvent{
		Event:   EventShimInstall,
		User:    currentUser,
		Binary:  "/bin/test",
		Success: true,
	}
	LogEvent(event)

	// Query by user
	query := &AuditQuery{User: currentUser}
	events, err := QueryAuditLog(query)
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Query by non-existent user
	query = &AuditQuery{User: "nonexistentuser"}
	events, err = QueryAuditLog(query)
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestQueryAuditLogBySuccess(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log successful and failed events
	LogShimInstall("/bin/test1", true, nil)
	LogShimInstall("/bin/test2", false, os.ErrPermission)

	// Query successful events
	success := true
	query := &AuditQuery{Success: &success}
	events, err := QueryAuditLog(query)
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 successful event, got %d", len(events))
	}
	if events[0].Binary != "/bin/test1" {
		t.Error("wrong event returned")
	}

	// Query failed events
	success = false
	query = &AuditQuery{Success: &success}
	events, err = QueryAuditLog(query)
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 failed event, got %d", len(events))
	}
	if events[0].Binary != "/bin/test2" {
		t.Error("wrong event returned")
	}
}

func TestGetAuditSummary(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log various events
	LogShimInstall("/bin/test1", true, nil)
	LogShimInstall("/bin/test2", false, os.ErrPermission)
	LogShimUninstall("/bin/test3", true, nil)
	LogBypassUsage("/bin/test4", 1234)
	LogBypassUsage("/bin/test5", 5678)
	LogSecurityViolation("path traversal", "/tmp/../etc", map[string]string{})

	// Get summary
	since := time.Now().Add(-1 * time.Hour)
	summary, err := GetAuditSummary(&since)
	if err != nil {
		t.Fatalf("GetAuditSummary() error = %v", err)
	}

	if summary.TotalEvents != 6 {
		t.Errorf("TotalEvents = %d, want 6", summary.TotalEvents)
	}
	if summary.SuccessfulOps != 4 {
		t.Errorf("SuccessfulOps = %d, want 4", summary.SuccessfulOps)
	}
	if summary.FailedOps != 2 {
		t.Errorf("FailedOps = %d, want 2", summary.FailedOps)
	}
	if summary.SecurityViolations != 1 {
		t.Errorf("SecurityViolations = %d, want 1", summary.SecurityViolations)
	}
	if summary.BypassUsages != 2 {
		t.Errorf("BypassUsages = %d, want 2", summary.BypassUsages)
	}
}

func TestGetAuditSummaryEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	since := time.Now().Add(-1 * time.Hour)
	summary, err := GetAuditSummary(&since)
	if err != nil {
		t.Fatalf("GetAuditSummary() error = %v", err)
	}

	if summary.TotalEvents != 0 {
		t.Errorf("TotalEvents = %d, want 0", summary.TotalEvents)
	}
}

func TestAuditLogPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Log an event
	LogShimInstall("/bin/test", true, nil)

	// Check log file permissions
	logPath, _ := GetAuditLogPath()
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("cannot stat log file: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("log file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestAuditLogMalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Unsetenv("XDG_STATE_HOME")

	// Write malformed JSON to log file
	logPath, _ := GetAuditLogPath()
	content := `{"event":"shim.install","binary":"/bin/test","success":true}
not valid json
{"event":"shim.uninstall","binary":"/bin/test2","success":true}
`
	err := os.WriteFile(logPath, []byte(content), 0600)
	if err != nil {
		t.Fatalf("cannot write test log: %v", err)
	}

	// Query should skip malformed lines
	events, err := QueryAuditLog(&AuditQuery{})
	if err != nil {
		t.Fatalf("QueryAuditLog() error = %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 valid events, got %d", len(events))
	}
}
