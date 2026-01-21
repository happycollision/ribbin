# Environment Variables Reference

All environment variables that affect Ribbin's behavior.

## RIBBIN_BYPASS

Skip wrapper logic and run the original command.

```bash
RIBBIN_BYPASS=1 tsc --version
```

| Value | Effect |
|-------|--------|
| `1` | Bypass wrappers |
| Any other value | Normal wrapper behavior |
| Unset | Normal wrapper behavior |

**Logged:** Yes, as `bypass.used` event.

## XDG_CONFIG_HOME

Override the configuration directory.

**Default:** `~/.config`

**Used for:**
- Registry: `$XDG_CONFIG_HOME/ribbin/registry.json`

```bash
export XDG_CONFIG_HOME=/custom/config
# Registry at /custom/config/ribbin/registry.json
```

## XDG_STATE_HOME

Override the state directory.

**Default:** `~/.local/state`

**Used for:**
- Audit log: `$XDG_STATE_HOME/ribbin/audit.log`

```bash
export XDG_STATE_HOME=/custom/state
# Audit log at /custom/state/ribbin/audit.log
```

## HOME

User's home directory. Used for path expansion (`~`).

**Used for:**
- Resolving `~/.local/bin`, `~/bin`
- Default XDG paths when XDG variables unset

## Redirect Script Environment

When using `action: "redirect"`, the redirect script receives these variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `RIBBIN_ORIGINAL_BIN` | Path to original binary | `/usr/local/bin/tsc.ribbin-original` |
| `RIBBIN_COMMAND` | Command name | `tsc` |
| `RIBBIN_CONFIG` | Path to ribbin.jsonc | `/project/ribbin.jsonc` |
| `RIBBIN_ACTION` | Always `redirect` | `redirect` |

**Example redirect script:**
```bash
#!/bin/bash
echo "Command: $RIBBIN_COMMAND"
echo "Original: $RIBBIN_ORIGINAL_BIN"
echo "Config: $RIBBIN_CONFIG"
exec "$RIBBIN_ORIGINAL_BIN" "$@"
```

## File Locations Summary

| Purpose | Default | Override Variable |
|---------|---------|-------------------|
| Config directory | `~/.config/ribbin/` | `XDG_CONFIG_HOME` |
| State directory | `~/.local/state/ribbin/` | `XDG_STATE_HOME` |
| Registry | `~/.config/ribbin/registry.json` | `XDG_CONFIG_HOME` |
| Audit log | `~/.local/state/ribbin/audit.log` | `XDG_STATE_HOME` |

## See Also

- [CLI Commands](cli-commands.md) - Command reference
- [Configuration Schema](config-schema.md) - Config file format
- [Redirect Commands](../how-to/redirect-commands.md) - Using redirect scripts
