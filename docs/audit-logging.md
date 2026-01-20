# Audit Logging

Ribbin includes comprehensive audit logging that tracks all security-relevant operations. The audit log provides visibility into wrapper installations, security violations, bypass usage, and privileged operations.

## Overview

The audit log is stored in JSONL (newline-delimited JSON) format for easy parsing and analysis. Each event includes automatic metadata like timestamp, user, UID, and elevated status.

### Log Location

```
~/.local/state/ribbin/audit.log
```

Or if `XDG_STATE_HOME` is set:
```
$XDG_STATE_HOME/ribbin/audit.log
```

File permissions: `0600` (readable only by owner)
Directory permissions: `0700` (accessible only by owner)

## Event Types

The audit log tracks the following event types:

| Event Type | Description | When Logged |
|------------|-------------|-------------|
| `wrap.install` | Wrapper installation | When `ribbin wrap` installs a wrapper (success or failure) |
| `wrap.uninstall` | Wrapper removal | When `ribbin unwrap` removes a wrapper (success or failure) |
| `bypass.used` | Bypass mechanism used | When `RIBBIN_BYPASS=1` is used to bypass a wrapper |
| `security.violation` | Security policy violation | Path traversal detected, forbidden directory accessed, etc. |
| `privileged.operation` | Privileged operation | Any operation performed as root |
| `config.load` | Configuration loaded | When ribbin.jsonc is loaded |
| `registry.update` | Registry updated | When the global registry is modified |

## Event Structure

Each event is a JSON object with the following fields:

```json
{
  "timestamp": "2026-01-18T15:30:45Z",
  "event": "wrap.install",
  "user": "username",
  "uid": 1000,
  "elevated": false,
  "binary": "/usr/local/bin/cat",
  "path": "/usr/local/bin/cat",
  "success": true,
  "error": "",
  "details": {}
}
```

### Field Descriptions

- **timestamp**: ISO 8601 timestamp (automatically set)
- **event**: Event type (see table above)
- **user**: Username (automatically set from `$USER`)
- **uid**: User ID (automatically set)
- **elevated**: Whether running as root (automatically set)
- **binary**: Binary path (for wrapper operations)
- **path**: File path (for file operations)
- **success**: Whether the operation succeeded
- **error**: Error message (if operation failed)
- **details**: Additional context (key-value pairs)

## CLI Commands

### View Recent Events

```bash
# Show last 50 events from last 24 hours
ribbin audit show

# Show events from last 7 days
ribbin audit show --since 7d

# Show only bypass events
ribbin audit show --type bypass.used

# Show last 100 events
ribbin audit show --limit 100

# Combine filters
ribbin audit show --since 30d --type security.violation --limit 20
```

**Duration formats**: `1h`, `24h`, `7d`, `30d`, etc.

### View Summary Statistics

```bash
# Show summary for last 30 days
ribbin audit summary
```

Output includes:
- Total events
- Successful operations
- Failed operations
- Elevated (root) operations
- Security violations
- Bypass usages

The summary command will warn you if:
- Security violations are detected
- Bypass is used frequently (>10 times)

## Examples

### Example 1: Track Wrapper Installations

```bash
# Install a wrapper
ribbin wrap

# View the audit log
ribbin audit show --type wrap.install
```

Output:
```
[2026-01-18 15:30:45] ✓ wrap.install: /usr/local/bin/cat
```

### Example 2: Detect Security Violations

```bash
# Attempt to wrap a forbidden path (by specifying it in config)
# ribbin.jsonc with cat pointing to /etc/passwd

# View violations
ribbin audit show --type security.violation
```

Output:
```
[2026-01-18 15:31:10] ✗ security.violation: /etc/passwd
    Error: path outside allowed directories
    Details: original_path=/etc/passwd
```

### Example 3: Track Bypass Usage

```bash
# Use bypass
RIBBIN_BYPASS=1 cat file.txt

# View bypass usage
ribbin audit show --type bypass.used
```

Output:
```
[2026-01-18 15:32:00] ✓ bypass.used: /usr/local/bin/cat.ribbin-original
    Details: pid=12345
```

### Example 4: Monitor Privileged Operations

```bash
# Install wrappers as root
sudo ribbin wrap

# View privileged operations
ribbin audit show
```

Output:
```
[2026-01-18 15:33:00] ✓ privileged.operation: /usr/local/bin/cat [ROOT]
[2026-01-18 15:33:01] ✓ wrap.install: /usr/local/bin/cat [ROOT]
```

## Querying the Log Programmatically

The audit log uses JSONL format, making it easy to parse with standard tools:

### Using jq

```bash
# Count events by type
cat ~/.local/state/ribbin/audit.log | jq -r '.event' | sort | uniq -c

# Find all failed operations
cat ~/.local/state/ribbin/audit.log | jq 'select(.success == false)'

# Find all elevated operations
cat ~/.local/state/ribbin/audit.log | jq 'select(.elevated == true)'

# Get events from last hour
cat ~/.local/state/ribbin/audit.log | \
  jq --arg since "$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)" \
  'select(.timestamp > $since)'
```

### Using grep

```bash
# Find all security violations
grep 'security.violation' ~/.local/state/ribbin/audit.log

# Find all bypass usage
grep 'bypass.used' ~/.local/state/ribbin/audit.log

# Find events for specific binary
grep '/usr/local/bin/cat' ~/.local/state/ribbin/audit.log
```

### Using Go

```go
package main

import (
    "github.com/happycollision/ribbin/internal/security"
    "time"
)

func main() {
    // Query events from last 24 hours
    since := time.Now().Add(-24 * time.Hour)
    query := &security.AuditQuery{
        StartTime: &since,
        EventType: security.EventSecurityViolation,
    }

    events, err := security.QueryAuditLog(query)
    if err != nil {
        panic(err)
    }

    // Process events...
}
```

## Security Considerations

### Log File Security

- The audit log is stored with `0600` permissions (owner read/write only)
- The audit directory has `0700` permissions (owner access only)
- Log writes are atomic to prevent corruption
- Logging failures never block operations (fail-safe design)

### What Gets Logged

**Logged**:
- Wrapper install/uninstall operations (with full paths)
- Security violations (path traversal, forbidden directories)
- Bypass usage (with PID)
- Privileged operations (operations run as root)

**Not Logged**:
- Command arguments (to avoid logging sensitive data)
- Environment variables (except detection of `RIBBIN_BYPASS`)
- File contents
- Network activity

### Privacy Considerations

The audit log contains:
- Usernames and UIDs
- File paths
- Timestamps
- Command names

This information is stored locally and never transmitted. The log file is readable only by the owner.

## Log Rotation

The audit log does not automatically rotate. To prevent unbounded growth:

### Manual Rotation

```bash
# Archive old logs
cp ~/.local/state/ribbin/audit.log ~/.local/state/ribbin/audit.log.old
> ~/.local/state/ribbin/audit.log

# Or with compression
gzip -c ~/.local/state/ribbin/audit.log > ~/audit-$(date +%Y%m%d).log.gz
> ~/.local/state/ribbin/audit.log
```

### Automated Rotation with logrotate

Create `/etc/logrotate.d/ribbin`:

```
/home/*/.local/state/ribbin/audit.log {
    weekly
    rotate 4
    compress
    missingok
    notifempty
    create 0600
}
```

## Troubleshooting

### Log Not Being Created

Check that the state directory is writable:
```bash
ls -ld ~/.local/state/ribbin
```

Check for error messages in stderr when running commands.

### Log Shows Warnings

If `ribbin audit summary` shows warnings:

**Security violations**:
```bash
# View details
ribbin audit show --type security.violation

# Common causes:
# - Attempting to wrap files in /etc, /bin, /usr/bin
# - Path traversal attempts (.. sequences)
# - Symlink attacks
```

**High bypass usage**:
```bash
# View bypass usage
ribbin audit show --type bypass.used

# This is normal if you frequently use RIBBIN_BYPASS=1
# Consider if you need to bypass that often
```

### Log File Too Large

```bash
# Check log size
du -h ~/.local/state/ribbin/audit.log

# Rotate manually (see Log Rotation above)
```

## Integration with Monitoring Tools

The JSONL format makes integration with monitoring tools straightforward:

### Splunk

```bash
# Configure input
[monitor://~/.local/state/ribbin/audit.log]
sourcetype = ribbin:audit
index = security
```

### ELK Stack

```yaml
# Filebeat configuration
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - ~/.local/state/ribbin/audit.log
  json.keys_under_root: true
  json.add_error_key: true
```

### Grafana Loki

```yaml
# Promtail configuration
- job_name: ribbin-audit
  static_configs:
  - targets:
      - localhost
    labels:
      job: ribbin-audit
      __path__: ~/.local/state/ribbin/audit.log
  pipeline_stages:
  - json:
      expressions:
        event: event
        user: user
        success: success
```

## Compliance

The audit log helps with:

- **Incident Response**: Track what happened during security incidents
- **Compliance Auditing**: Demonstrate security controls are in place
- **Access Control**: Track privileged operations and access patterns
- **Change Management**: Track when wrappers are installed/removed

## API Reference

See [internal/security/audit.go](../internal/security/audit.go) for the full API documentation.

### Key Functions

- `LogEvent(event *AuditEvent)`: Log a custom event
- `LogWrapInstall(binary string, success bool, err error)`: Log wrapper installation
- `LogWrapUninstall(binary string, success bool, err error)`: Log wrapper removal
- `LogBypassUsage(command string, pid int)`: Log bypass usage
- `LogSecurityViolation(violation, path string, details map[string]string)`: Log security violation
- `QueryAuditLog(query *AuditQuery) ([]*AuditEvent, error)`: Query the audit log
- `GetAuditSummary(since *time.Time) (*AuditSummary, error)`: Get summary statistics
