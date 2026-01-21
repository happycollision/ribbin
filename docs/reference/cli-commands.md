# CLI Commands Reference

Complete reference for all Ribbin commands.

## ribbin init

Create a `ribbin.jsonc` configuration file.

```bash
ribbin init [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing config file |

**Example:**
```bash
ribbin init
ribbin init --force
```

## ribbin wrap

Install wrappers for commands defined in config.

```bash
ribbin wrap [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--confirm-system-dir` | Allow wrapping in system directories (`/usr/bin`, etc.) |
| `--dry-run` | Show what would be wrapped without making changes |

**Example:**
```bash
ribbin wrap
ribbin wrap --dry-run
sudo ribbin wrap --confirm-system-dir
```

## ribbin unwrap

Remove wrappers and restore original binaries.

```bash
ribbin unwrap [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--all` | Unwrap all registered wrappers |
| `--dry-run` | Show what would be unwrapped without making changes |

**Example:**
```bash
ribbin unwrap
ribbin unwrap --all
```

## ribbin activate

Enable Ribbin wrappers.

```bash
ribbin activate [config-path] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `config-path` | Path to specific config file (config-scoped activation) |

**Flags:**
| Flag | Description |
|------|-------------|
| `--global` | Activate system-wide |
| `--shell` | Activate for current shell only |

**Example:**
```bash
ribbin activate --global
ribbin activate --shell
ribbin activate ./ribbin.jsonc
```

## ribbin deactivate

Disable Ribbin wrappers.

```bash
ribbin deactivate [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--global` | Deactivate system-wide |
| `--shell` | Deactivate for current shell only |
| `--all` | Deactivate all activation modes |

**Example:**
```bash
ribbin deactivate --global
ribbin deactivate --shell
ribbin deactivate --all
```

## ribbin status

Show current activation status.

```bash
ribbin status [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

**Example:**
```bash
ribbin status
ribbin status --json
```

## ribbin recover

Restore orphaned wrapped binaries.

```bash
ribbin recover [flags]
```

Use when wrappers exist but registry is corrupted or missing.

**Flags:**
| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be recovered |

**Example:**
```bash
ribbin recover
ribbin recover --dry-run
```

## ribbin config add

Add a wrapper to the config file.

```bash
ribbin config add <command> [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `command` | Command name to wrap |

**Flags:**
| Flag | Description |
|------|-------------|
| `--action` | Action type: `block`, `warn`, `redirect`, `passthrough` |
| `--message` | Message to display |
| `--redirect` | Script path (for redirect action) |

**Example:**
```bash
ribbin config add tsc --action=block --message="Use pnpm run typecheck"
ribbin config add npm --action=block --message="Use pnpm"
```

## ribbin config remove

Remove a wrapper from the config file.

```bash
ribbin config remove <command>
```

**Example:**
```bash
ribbin config remove tsc
```

## ribbin config list

List all configured wrappers.

```bash
ribbin config list [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

**Example:**
```bash
ribbin config list
ribbin config list --json
```

## ribbin config show

Show effective config for current directory.

```bash
ribbin config show [flags]
```

Shows merged config after applying scopes and inheritance.

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

**Example:**
```bash
ribbin config show
cd apps/frontend && ribbin config show
```

## ribbin audit show

View audit log events.

```bash
ribbin audit show [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--since` | Time range (e.g., `24h`, `7d`, `30d`) |
| `--type` | Filter by event type |
| `--limit` | Maximum events to show |
| `--failed` | Show only failed operations |

**Example:**
```bash
ribbin audit show
ribbin audit show --since 7d
ribbin audit show --type security.violation
ribbin audit show --since 30d --type bypass.used --limit 20
```

## ribbin audit summary

View audit log statistics.

```bash
ribbin audit summary [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--since` | Time range (default: 30d) |

**Example:**
```bash
ribbin audit summary
ribbin audit summary --since 7d
```

## Global Flags

Available on all commands:

| Flag | Description |
|------|-------------|
| `--help` | Show help for command |
| `--version` | Show Ribbin version |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `RIBBIN_BYPASS` | Set to `1` to bypass wrappers |
| `XDG_CONFIG_HOME` | Override config directory (default: `~/.config`) |
| `XDG_STATE_HOME` | Override state directory (default: `~/.local/state`) |

## See Also

- [Configuration Schema](config-schema.md) - Config file format
- [Environment Variables](environment-vars.md) - All environment variables
