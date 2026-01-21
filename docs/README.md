# Ribbin Documentation

Comprehensive documentation for Ribbin, the command wrapping tool.

## Getting Started

- [Main README](../README.md) - Quick start guide and basic usage
- [Installation Guide](../README.md#installation) - How to install Ribbin

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

**Conditional Passthrough:**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "passthrough": {
        // Allow when called from pnpm scripts
        "invocation": ["pnpm run typecheck", "pnpm run build"],
        // Or use regex for flexible matching
        "invocationRegexp": ["pnpm (typecheck|build)"]
      }
    }
  }
}
```

The `passthrough` option allows commands through when the parent process matches specified patterns. See [AI Coding Agents Guide](agent-integration.md#approach-b-keep-codebase-unchanged-passthrough) for detailed examples.

### Scopes (Monorepo Support)

Scopes let you define different wrapper rules for different directories—ideal for monorepos where different apps have different requirements.

```jsonc
{
  // Root-level wrappers apply everywhere unless overridden
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "Use pnpm instead"
    },
    "rm": {
      "action": "warn",
      "message": "Be careful with rm"
    }
  },

  "scopes": {
    // Frontend: stricter rules
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root"],
      "wrappers": {
        "yarn": {
          "action": "block",
          "message": "Use pnpm in frontend"
        },
        // Override rm to be stricter
        "rm": {
          "action": "block",
          "message": "Use trash-cli in frontend"
        }
      }
    },

    // Backend: allow npm for legacy reasons
    "backend": {
      "path": "apps/backend",
      "extends": ["root"],
      "wrappers": {
        "npm": {
          "action": "passthrough"
        }
      }
    }
  }
}
```

**How scopes work:**
- `path` - Directory this scope applies to (relative to config file)
- `extends` - Inherit wrappers from other sources (see below)
- `wrappers` - Additional or overriding wrappers for this scope

When a command runs, Ribbin checks the current working directory and applies the most specific matching scope.

**Check effective config per directory:**
```bash
ribbin config show                    # From project root
cd apps/frontend && ribbin config show  # See frontend rules
cd apps/backend && ribbin config show   # See backend rules
```

### Config Inheritance (extends)

The `extends` field lets scopes inherit wrappers from multiple sources:

**1. Root wrappers:**
```jsonc
{
  "wrappers": { /* root-level */ },
  "scopes": {
    "myapp": {
      "path": "apps/myapp",
      "extends": ["root"],  // Inherit root wrappers
      "wrappers": { /* additions/overrides */ }
    }
  }
}
```

**2. Other scopes (mixins):**
```jsonc
{
  "scopes": {
    // Mixin: no path, can only be extended
    "hardened": {
      "wrappers": {
        "rm": { "action": "block", "message": "Use trash" },
        "curl": { "action": "warn", "message": "Use httpie" }
      }
    },

    "production": {
      "path": "apps/prod",
      "extends": ["root", "root.hardened"],  // Inherit from mixin
      "wrappers": { }
    }
  }
}
```

**3. External files:**
```jsonc
{
  "scopes": {
    "myapp": {
      "path": "apps/myapp",
      "extends": [
        "./team-configs/security-baseline.jsonc",  // External file
        "./team-configs/production.jsonc"
      ]
    }
  }
}
```

**Inheritance order:** Later entries in `extends` override earlier ones. Local `wrappers` override everything.

**Mixins vs Scopes:**
- **Scope**: Has a `path` → applies when working in that directory
- **Mixin**: No `path` → can only be referenced via `extends`

### User-Local Config Override

Create `ribbin.local.jsonc` for personal overrides that aren't committed:

```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"],  // Inherit shared config
      "wrappers": {
        // Your personal overrides
      }
    }
  }
}
```

When present, `ribbin.local.jsonc` is loaded instead of `ribbin.jsonc`. Add it to `.gitignore`.

See [Configuration Options](../README.md#configuration) for full details.

## Commands Reference

| Command | Description | Documentation |
|---------|-------------|---------------|
| `ribbin init` | Initialize ribbin.jsonc | [README](../README.md#quick-start) |
| `ribbin wrap` | Install wrappers | [README](../README.md#quick-start) |
| `ribbin unwrap` | Remove wrappers | [README](../README.md#commands) |
| `ribbin activate` | Activate Ribbin (config, shell, or global) | [README](../README.md#activation-modes) |
| `ribbin deactivate` | Deactivate Ribbin | [README](../README.md#commands) |
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

Then in `package.json`, bypass Ribbin for the actual script:
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
3. When invoked, Ribbin checks configuration
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

1. Check if Ribbin is active:
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
