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

Install wrappers for commands defined in config. By default, uses the nearest `ribbin.jsonc` or `ribbin.local.jsonc`. You can optionally specify config files explicitly.

```bash
ribbin wrap [config-files...] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--confirm-system-dir` | Allow wrapping in system directories (`/usr/bin`, etc.) |
| `--dry-run` | Show what would be wrapped without making changes |

**Example:**
```bash
ribbin wrap                           # Use nearest config
ribbin wrap ./ribbin.jsonc            # Use specific config
ribbin wrap ./a.jsonc ./b.jsonc       # Use multiple configs
ribbin wrap --dry-run
sudo ribbin wrap --confirm-system-dir
```

## ribbin unwrap

Remove wrappers and restore original binaries. By default, uses the nearest config. You can optionally specify config files explicitly.

```bash
ribbin unwrap [config-files...] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--all` | Unwrap all registered wrappers |
| `--dry-run` | Show what would be unwrapped without making changes |

**Example:**
```bash
ribbin unwrap                         # Use nearest config
ribbin unwrap ./ribbin.jsonc          # Use specific config
ribbin unwrap --all                   # Unwrap everything
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

Disable Ribbin wrappers. You can optionally specify config files for config-scoped deactivation.

```bash
ribbin deactivate [config-files...] [flags]
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
ribbin deactivate ./ribbin.jsonc      # Config-scoped deactivation
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

Add a wrapper to a config file. By default, uses the nearest config.

```bash
ribbin config add <command> [flags]
ribbin config add <config-path> <command> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--action` | Action type: `block`, `warn`, `redirect`, `passthrough` (required) |
| `--message` | Message to display |
| `--redirect` | Script path (for redirect action) |

**Example:**
```bash
ribbin config add tsc --action=block --message="Use pnpm run typecheck"
ribbin config add ./ribbin.jsonc tsc --action=block   # Add to specific config
ribbin config add npm --action=block --message="Use pnpm"
```

## ribbin config edit

Edit an existing wrapper in a config file. By default, uses the nearest config.

```bash
ribbin config edit <command> [flags]
ribbin config edit <config-path> <command> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--action` | Change action type: `block`, `redirect` |
| `--message` | Update message |
| `--redirect` | Update redirect script path |
| `--paths` | Update binary path restrictions |
| `--clear-message` | Clear the message field |
| `--clear-paths` | Clear the paths restrictions |
| `--clear-redirect` | Clear the redirect field |

**Example:**
```bash
ribbin config edit tsc --message="Use 'bun run typecheck'"
ribbin config edit ./ribbin.jsonc tsc --message="..."   # Edit in specific config
ribbin config edit npm --action=redirect --redirect=/usr/local/bin/pnpm
```

## ribbin config remove

Remove a wrapper from a config file. By default, uses the nearest config.

```bash
ribbin config remove <command> [flags]
ribbin config remove <config-path> <command> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |

**Example:**
```bash
ribbin config remove tsc
ribbin config remove ./ribbin.jsonc tsc --force   # Remove from specific config
```

## ribbin config list

List all configured wrappers. By default, uses the nearest config.

```bash
ribbin config list [config-path] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |
| `--command` | Filter to specific command |

**Example:**
```bash
ribbin config list                    # Use nearest config
ribbin config list ./ribbin.jsonc     # Use specific config
ribbin config list --json
```

## ribbin config show

Show effective config for current directory. By default, uses the nearest config.

```bash
ribbin config show [config-path] [flags]
```

Shows merged config after applying scopes and inheritance.

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |
| `--command` | Filter to specific command |

**Example:**
```bash
ribbin config show                    # Use nearest config
ribbin config show ./ribbin.jsonc     # Use specific config
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
