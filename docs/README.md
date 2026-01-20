# Ribbin Documentation

Comprehensive documentation for ribbin, the command wrapping tool.

## Getting Started

- [Main README](../README.md) - Quick start guide and basic usage
- [Installation Guide](../README.md#installation) - How to install ribbin

## Features

### Core Features
- **Command Wrapping** - Intercept and block/warn/redirect commands
- **Project Configuration** - Per-project `ribbin.jsonc` files
- **Activation Modes** - Config-scoped, shell-scoped, or global activation

### Security Features
- [Security Overview](security.md) - Comprehensive security features
- [Audit Logging](audit-logging.md) - Security event logging and monitoring

### Integrations
- [AI Coding Agents](agent-integration.md) - Practical guide with bypass examples

### Performance
- [Performance](performance.md) - Overhead measurements and benchmarks

## Configuration

### Basic Configuration

Create a `ribbin.jsonc` file in your project:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead"
    }
  }
}
```

### Advanced Configuration

**Redirect Actions:**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "redirect",
      "redirect": "./scripts/typecheck-wrapper.sh"
    }
  }
}
```

**Path Restrictions:**
```jsonc
{
  "wrappers": {
    "curl": {
      "action": "block",
      "message": "Use the project's API client instead",
      "paths": ["/usr/bin/curl", "/bin/curl"]
    }
  }
}
```

See [Configuration Options](../README.md#configuration) for full details.

## Commands Reference

| Command | Description | Documentation |
|---------|-------------|---------------|
| `ribbin init` | Initialize ribbin.jsonc | [README](../README.md#quick-start) |
| `ribbin wrap` | Install wrappers | [README](../README.md#quick-start) |
| `ribbin unwrap` | Remove wrappers | [README](../README.md#commands) |
| `ribbin activate` | Activate ribbin (config, shell, or global) | [README](../README.md#activation-modes) |
| `ribbin deactivate` | Deactivate ribbin | [README](../README.md#commands) |
| `ribbin status` | Show activation status | Run `ribbin status --help` |
| `ribbin recover` | Recover orphaned wrappers | Run `ribbin recover --help` |
| `ribbin audit show` | View audit log | [Audit Logging](audit-logging.md) |
| `ribbin audit summary` | View audit statistics | [Audit Logging](audit-logging.md) |
| `ribbin config add` | Add wrapper config | Run `ribbin config add --help` |
| `ribbin config remove` | Remove wrapper config | Run `ribbin config remove --help` |
| `ribbin config list` | List wrapper configs | Run `ribbin config list --help` |
| `ribbin config show` | Show effective config | Run `ribbin config show --help` |

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
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' - it uses the project's tsconfig"
    }
  }
}
```

### Package Manager Enforcement
```jsonc
{
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "This project uses pnpm"
    }
  }
}
```

### AI Agent Guardrails

See the full [AI Coding Agents Guide](agent-integration.md) for setup with bypass examples.

```jsonc
{
  "wrappers": {
    // Block direct tsc - guide to project script
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' to use project's tsconfig"
    },
    // Enforce package manager
    "npm": {
      "action": "block",
      "message": "This project uses pnpm"
    }
  }
}
```

Then in `package.json`, bypass ribbin for the actual script:
```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit"
  }
}
```

See [Use Cases](../README.md#use-cases) for more examples.

## Architecture

### How It Works

Ribbin uses a "sidecar" approach:

1. Original binary renamed: `npm` → `npm.ribbin-original`
2. Symlink created: `npm` → `ribbin`
3. When invoked, ribbin checks configuration
4. If blocked: show error message
5. If warned: show warning, then run original
6. If redirected: run redirect script
7. Otherwise: exec original binary

See [How It Works](../README.md#how-it-works) for details.

### Directory Structure

```
~/.local/bin/ribbin              # Ribbin binary
~/.config/ribbin/registry.json   # Global registry
~/.local/state/ribbin/audit.log  # Audit log
project/ribbin.jsonc             # Project config
```

### File Locations

- **Binary**: Installed to `$GOPATH/bin` or `/usr/local/bin`
- **Registry**: `$XDG_CONFIG_HOME/ribbin/registry.json` (default: `~/.config/ribbin/`)
- **Audit Log**: `$XDG_STATE_HOME/ribbin/audit.log` (default: `~/.local/state/ribbin/`)
- **Config**: `ribbin.jsonc` in project root

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
  config/                # Config file parsing (JSONC)
  wrap/                  # Wrapper logic
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

### Wrapper Not Working

1. Check if ribbin is active:
   ```bash
   ribbin status
   ribbin activate --global  # or --shell
   ```

2. Verify wrapper is installed:
   ```bash
   ls -la $(which command)
   ```

3. Check for ribbin.jsonc:
   ```bash
   ls ribbin.jsonc
   ```

### Permission Denied

Use sudo for system directories:
```bash
sudo ribbin wrap --confirm-system-dir
```

Or use user-local directories:
```bash
mkdir -p ~/.local/bin
# Add ~/.local/bin to PATH
ribbin wrap
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
