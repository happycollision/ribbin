# How to View and Query Audit Logs

Monitor what Ribbin is blocking, allowing, and logging.

## View Recent Events

```bash
ribbin audit show
```

Shows the last 50 events from the last 24 hours.

## Filter by Time

```bash
# Last 7 days
ribbin audit show --since 7d

# Last 30 days
ribbin audit show --since 30d

# Last 2 hours
ribbin audit show --since 2h
```

## Filter by Event Type

```bash
# Only bypass events
ribbin audit show --type bypass.used

# Only security violations
ribbin audit show --type security.violation

# Only wrapper installations
ribbin audit show --type wrap.install
```

Available event types:
- `wrap.install` - Wrapper installed
- `wrap.uninstall` - Wrapper removed
- `bypass.used` - `RIBBIN_BYPASS=1` used
- `security.violation` - Security policy violation
- `privileged.operation` - Operation run as root
- `config.load` - Configuration loaded
- `registry.update` - Registry modified

## Limit Results

```bash
ribbin audit show --limit 100
```

## Combine Filters

```bash
ribbin audit show --since 30d --type security.violation --limit 20
```

## View Summary Statistics

```bash
ribbin audit summary
```

Shows:
- Total events
- Successful/failed operations
- Elevated (root) operations
- Security violations
- Bypass usage count

## Query with jq

The log is JSONL format at `~/.local/state/ribbin/audit.log`:

```bash
# Count events by type
cat ~/.local/state/ribbin/audit.log | jq -r '.event' | sort | uniq -c

# Find all failed operations
cat ~/.local/state/ribbin/audit.log | jq 'select(.success == false)'

# Find all elevated operations
cat ~/.local/state/ribbin/audit.log | jq 'select(.elevated == true)'

# Events for specific binary
cat ~/.local/state/ribbin/audit.log | jq 'select(.binary | contains("tsc"))'
```

## Query with grep

```bash
# All security violations
grep 'security.violation' ~/.local/state/ribbin/audit.log

# All bypass usage
grep 'bypass.used' ~/.local/state/ribbin/audit.log

# Events for specific binary
grep '/usr/local/bin/tsc' ~/.local/state/ribbin/audit.log
```

## Interpret Results

**wrap.install success:**
```
[2026-01-18 15:30:45] ✓ wrap.install: /usr/local/bin/tsc
```
Wrapper installed successfully.

**security.violation:**
```
[2026-01-18 15:31:10] ✗ security.violation: /etc/passwd
    Error: path outside allowed directories
```
Someone tried to wrap a forbidden path.

**bypass.used:**
```
[2026-01-18 15:32:00] ✓ bypass.used: /usr/local/bin/tsc.ribbin-original
    Details: pid=12345
```
Command ran with `RIBBIN_BYPASS=1`.

## See Also

- [Rotate Logs](rotate-logs.md) - Manage log file size
- [Audit Log Format](../reference/audit-log-format.md) - Event structure reference
- [Security Features](../reference/security-features.md) - What gets logged
