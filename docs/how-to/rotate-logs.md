# How to Rotate Audit Logs

The audit log doesn't automatically rotate. Here's how to manage its size.

## Check Log Size

```bash
du -h ~/.local/state/ribbin/audit.log
```

## Manual Rotation

Archive and clear:

```bash
# Simple archive
cp ~/.local/state/ribbin/audit.log ~/.local/state/ribbin/audit.log.old
> ~/.local/state/ribbin/audit.log

# With date and compression
gzip -c ~/.local/state/ribbin/audit.log > ~/ribbin-audit-$(date +%Y%m%d).log.gz
> ~/.local/state/ribbin/audit.log
```

## Automated Rotation with logrotate

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

This keeps 4 weeks of compressed logs.

## Rotation Options

| Option | Effect |
|--------|--------|
| `weekly` | Rotate weekly |
| `daily` | Rotate daily |
| `monthly` | Rotate monthly |
| `rotate 4` | Keep 4 old logs |
| `compress` | gzip old logs |
| `missingok` | Don't error if log missing |
| `notifempty` | Don't rotate if empty |
| `create 0600` | Set permissions on new file |

## Log Location

Default: `~/.local/state/ribbin/audit.log`

Or if `XDG_STATE_HOME` is set: `$XDG_STATE_HOME/ribbin/audit.log`

## See Also

- [View Audit Logs](view-audit-logs.md) - Query and analyze logs
- [Audit Log Format](../reference/audit-log-format.md) - Event structure reference
