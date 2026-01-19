# Ribbin Documentation

Comprehensive documentation for ribbin, the command shimming tool.

## Getting Started

- [Main README](../README.md) - Quick start guide and basic usage
- [Installation Guide](../README.md#installation) - How to install ribbin

## Features

### Core Features
- **Command Shimming** - Intercept and block/redirect commands
- **Project Configuration** - Per-project `ribbin.toml` files
- **Activation Modes** - Shell-scoped or global activation

### Security Features
- [Security Overview](security.md) - Comprehensive security features
- [Audit Logging](audit-logging.md) - Security event logging and monitoring

### Performance
- [Performance](performance.md) - Overhead measurements and benchmarks

## Configuration

### Basic Configuration

Create a `ribbin.toml` file in your project:

```toml
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' instead"
```

### Advanced Configuration

**Redirect Actions:**
```toml
[shims.tsc]
action = "redirect"
redirect = "./scripts/typecheck-wrapper.sh"
```

**Path Restrictions:**
```toml
[shims.cat]
action = "block"
message = "Use 'bat' instead"
paths = ["/usr/bin/cat", "/bin/cat"]
```

See [Configuration Options](../README.md#configuration) for full details.

## Commands Reference

| Command | Description | Documentation |
|---------|-------------|---------------|
| `ribbin init` | Initialize ribbin.toml | [README](../README.md#quick-start) |
| `ribbin shim` | Install shims | [README](../README.md#quick-start) |
| `ribbin unshim` | Remove shims | [README](../README.md#commands) |
| `ribbin activate` | Shell-scoped activation | [README](../README.md#activation-modes) |
| `ribbin on/off` | Global activation | [README](../README.md#activation-modes) |
| `ribbin audit show` | View audit log | [Audit Logging](audit-logging.md) |
| `ribbin audit summary` | View audit statistics | [Audit Logging](audit-logging.md) |
| `ribbin config add` | Add shim config | Run `ribbin config add --help` |
| `ribbin config remove` | Remove shim config | Run `ribbin config remove --help` |
| `ribbin config list` | List shim configs | Run `ribbin config list --help` |

## Security

Ribbin includes comprehensive security hardening:

- **[Security Overview](security.md)** - All security features
  - Path sanitization and validation
  - Directory allowlist
  - File locking (TOCTOU prevention)
  - Symlink attack prevention
  - Atomic operations
  - Privilege warnings

- **[Audit Logging](audit-logging.md)** - Security event tracking
  - Event types and structure
  - CLI commands
  - Querying and analysis
  - Integration with monitoring tools

## Use Cases

### TypeScript Projects
```toml
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - it uses the project's tsconfig"
```

### Package Manager Enforcement
```toml
[shims.npm]
action = "block"
message = "This project uses pnpm"
```

### AI Agent Guardrails
```toml
[shims.rm]
action = "block"
message = "Use 'trash' for safe deletion"

[shims.cat]
action = "block"
message = "Use 'bat' or the Read tool"
```

See [Use Cases](../README.md#use-cases) for more examples.

## Architecture

### How It Works

Ribbin uses a "sidecar" approach:

1. Original binary renamed: `cat` → `cat.ribbin-original`
2. Symlink created: `cat` → `ribbin`
3. When invoked, ribbin checks configuration
4. If blocked: show error message
5. Otherwise: exec original binary

See [How It Works](../README.md#how-it-works) for details.

### Directory Structure

```
~/.local/bin/ribbin              # Ribbin binary
~/.config/ribbin/registry.json   # Global registry
~/.local/state/ribbin/audit.log  # Audit log
project/ribbin.toml              # Project config
```

### File Locations

- **Binary**: Installed to `$GOPATH/bin` or `/usr/local/bin`
- **Registry**: `$XDG_CONFIG_HOME/ribbin/registry.json` (default: `~/.config/ribbin/`)
- **Audit Log**: `$XDG_STATE_HOME/ribbin/audit.log` (default: `~/.local/state/ribbin/`)
- **Config**: `ribbin.toml` in project root

## Development

### Building from Source

```bash
git clone https://github.com/happycollision/ribbin
cd ribbin
make build
```

### Running Tests

```bash
make test
make test-integration
make test-coverage
```

### Project Structure

```
cmd/ribbin/              # CLI entry point
internal/
  cli/                   # CLI commands (Cobra)
  config/                # Config file parsing (TOML)
  shim/                  # Shim logic
  security/              # Security features
    paths.go             # Path validation
    allowlist.go         # Directory allowlist
    filelock.go          # File locking
    symlinks.go          # Symlink validation
    audit.go             # Audit logging
  process/               # PID ancestry checking
docs/                    # Documentation
```

## Troubleshooting

### Shim Not Working

1. Check if ribbin is active:
   ```bash
   ribbin on  # or ribbin activate
   ```

2. Verify shim is installed:
   ```bash
   ls -la $(which command)
   ```

3. Check for ribbin.toml:
   ```bash
   ls ribbin.toml
   ```

### Permission Denied

Use sudo for system directories:
```bash
sudo ribbin shim cat --confirm-system-dir
```

Or use user-local directories:
```bash
mkdir -p ~/.local/bin
# Add ~/.local/bin to PATH
ribbin shim ~/.local/bin/mycommand
```

### Bypass Not Working

Ensure exact syntax:
```bash
RIBBIN_BYPASS=1 command args
```

Or use original path:
```bash
/usr/bin/command.ribbin-original args
```

### Audit Log Issues

Check audit log location:
```bash
ls -la ~/.local/state/ribbin/audit.log
```

View recent events:
```bash
ribbin audit show
```

## Contributing

Contributions are welcome! Please:

1. Read the [security documentation](security.md)
2. Follow Go coding standards
3. Add tests for new features
4. Update documentation

## License

MIT License - See [LICENSE](../LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/happycollision/ribbin/issues)
- **Discussions**: [GitHub Discussions](https://github.com/happycollision/ribbin/discussions)
- **Security**: See [Security Overview](security.md#reporting-security-issues)
