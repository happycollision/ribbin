# Audit Log Format Reference

Technical reference for the audit log structure.

## File Location

```
~/.local/state/ribbin/audit.log
```

Or if `XDG_STATE_HOME` is set:
```
$XDG_STATE_HOME/ribbin/audit.log
```

## File Format

JSONL (newline-delimited JSON). Each line is a complete JSON object.

## File Permissions

- **File**: `0600` (owner read/write only)
- **Directory**: `0700` (owner access only)

## Event Structure

```json
{
  "timestamp": "2026-01-18T15:30:45Z",
  "event": "wrap.install",
  "user": "username",
  "uid": 1000,
  "elevated": false,
  "binary": "/usr/local/bin/tsc",
  "path": "/usr/local/bin/tsc",
  "success": true,
  "error": "",
  "details": {}
}
```

## Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | ISO 8601 timestamp (UTC) |
| `event` | string | Event type (see below) |
| `user` | string | Username from `$USER` |
| `uid` | integer | User ID |
| `elevated` | boolean | `true` if running as root |
| `binary` | string | Binary path (for wrapper operations) |
| `path` | string | File path (for file operations) |
| `success` | boolean | Whether operation succeeded |
| `error` | string | Error message if failed |
| `details` | object | Additional context |

## Event Types

### wrap.install

Logged when a wrapper is installed.

```json
{
  "event": "wrap.install",
  "binary": "/usr/local/bin/tsc",
  "success": true
}
```

### wrap.uninstall

Logged when a wrapper is removed.

```json
{
  "event": "wrap.uninstall",
  "binary": "/usr/local/bin/tsc",
  "success": true
}
```

### bypass.used

Logged when `RIBBIN_BYPASS=1` is used.

```json
{
  "event": "bypass.used",
  "binary": "/usr/local/bin/tsc.ribbin-original",
  "success": true,
  "details": {
    "pid": "12345"
  }
}
```

### security.violation

Logged when a security policy is violated.

```json
{
  "event": "security.violation",
  "path": "/usr/bin/mytool",
  "success": false,
  "error": "shimming requires explicit confirmation",
  "details": {
    "original_path": "/usr/bin/mytool",
    "violation_type": "system_directory"
  }
}
```

Violation types:
- `path_traversal` - `..` sequence detected
- `system_directory` - Path in a system directory without confirmation
- `symlink_escape` - Symlink points to disallowed location
- `chain_depth_exceeded` - Symlink chain too deep

### privileged.operation

Logged when running as root.

```json
{
  "event": "privileged.operation",
  "binary": "/usr/local/bin/tsc",
  "elevated": true
}
```

### config.load

Logged when configuration is loaded.

```json
{
  "event": "config.load",
  "path": "/project/ribbin.jsonc",
  "success": true
}
```

### registry.update

Logged when the global registry is modified.

```json
{
  "event": "registry.update",
  "path": "~/.config/ribbin/registry.json",
  "success": true,
  "details": {
    "action": "add",
    "binary": "/usr/local/bin/tsc"
  }
}
```

## Details Field

The `details` object varies by event type:

| Event | Details Fields |
|-------|----------------|
| `bypass.used` | `pid` |
| `security.violation` | `original_path`, `violation_type` |
| `registry.update` | `action`, `binary` |

## Querying Examples

**jq - Count by event type:**
```bash
cat ~/.local/state/ribbin/audit.log | jq -r '.event' | sort | uniq -c
```

**jq - Failed operations:**
```bash
cat ~/.local/state/ribbin/audit.log | jq 'select(.success == false)'
```

**jq - Elevated operations:**
```bash
cat ~/.local/state/ribbin/audit.log | jq 'select(.elevated == true)'
```

**jq - Events after timestamp:**
```bash
cat ~/.local/state/ribbin/audit.log | \
  jq --arg since "2026-01-15T00:00:00Z" 'select(.timestamp > $since)'
```

**grep - Security violations:**
```bash
grep 'security.violation' ~/.local/state/ribbin/audit.log
```

## See Also

- [View Audit Logs](../how-to/view-audit-logs.md) - How to query logs
- [Rotate Logs](../how-to/rotate-logs.md) - Manage log size
- [Security Features](security-features.md) - What triggers logging
