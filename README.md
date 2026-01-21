# Ribbin

Block direct tool calls and redirect to project-specific alternatives.

## The Problem

AI agents sometimes forget or ignore project instructions and call tools directly instead of using project-configured wrappers. This leads to misconfigured tool runs, confusion, rabbit holes, and repeated mistakes.

| Ecosystem | Agent runs... | But should run... |
|-----------|---------------|-------------------|
| **JavaScript** | `tsc`, `jest`, `eslint` | `pnpm run typecheck`, `pnpm test`, `pnpm run lint` |
| **Python** | `pytest`, `pip install` | `poetry run pytest`, `poetry add` |
| **Go** | `go test`, `go build` | `make test`, `make build` |
| **Rust** | `cargo test`, `cargo clippy` | `make test`, `make lint` |
| **Ruby** | `rspec`, `rubocop` | `bundle exec rspec`, `rake lint` |

## The Solution

Ribbin intercepts calls to specified commands and blocks them with helpful error messages explaining what to do instead.

```
┌─────────────────────────────────────────────────────┐
│  ERROR: Direct use of 'tsc' is blocked.             │
│                                                     │
│  Use 'pnpm run typecheck' instead                   │
│                                                     │
│  Bypass: RIBBIN_BYPASS=1 tsc ...                    │
└─────────────────────────────────────────────────────┘
```

## Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/happycollision/ribbin/main/install.sh | bash
```

### From Source

```bash
go install github.com/happycollision/ribbin/cmd/ribbin@latest
```

### Manual Download

Download the latest release from [GitHub Releases](https://github.com/happycollision/ribbin/releases).

## Quick Start

1. Initialize Ribbin in your project:

```bash
ribbin init
```

2. Edit the generated `ribbin.jsonc` to add your wrappers:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead"
    },
    "npm": {
      "action": "block",
      "message": "This project uses pnpm"
    }
  }
}
```

3. Install the wrappers:

```bash
ribbin wrap
```

4. Activate Ribbin globally:

```bash
ribbin activate --global
```

Now when you (or an AI agent) runs `tsc` in this project, they'll see the helpful error instead.

## Configuration

### Actions

| Action | Behavior |
|--------|----------|
| `block` | Show error message and exit |
| `warn` | Show warning, then run original command |
| `redirect` | Execute a custom script instead |
| `passthrough` | Always allow (useful for overrides in scopes) |

### Basic Example

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

### Redirect Example

Execute a wrapper script instead of blocking:

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

### Passthrough Example

Block direct calls but allow when run from approved scripts:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "passthrough": {
        "invocation": ["pnpm run typecheck", "pnpm run build"]
      }
    }
  }
}
```

This keeps `package.json` unchanged—no `RIBBIN_BYPASS` needed. See the [AI Coding Agents Guide](docs/agent-integration.md) for details.

### Monorepo Scopes

Different rules for different directories:

```jsonc
{
  "wrappers": {
    "npm": { "action": "block", "message": "Use pnpm" }
  },
  "scopes": {
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root"],
      "wrappers": {
        "yarn": { "action": "block", "message": "Use pnpm" }
      }
    }
  }
}
```

See [Configuration Guide](docs/README.md#configuration) for scopes, inheritance, and local overrides.

## Common Use Cases

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
    "npm": { "action": "block", "message": "This project uses pnpm" },
    "yarn": { "action": "block", "message": "This project uses pnpm" }
  }
}
```

### AI Agent Guardrails

```jsonc
{
  "wrappers": {
    "rm": {
      "action": "block",
      "message": "Use 'trash' for safe deletion"
    }
  }
}
```

## Bypass

When you legitimately need to run the original command:

```bash
RIBBIN_BYPASS=1 tsc --version
```

## Commands

| Command | Description |
|---------|-------------|
| `ribbin init` | Create a `ribbin.jsonc` in the current directory |
| `ribbin wrap` | Install wrappers for commands in config |
| `ribbin unwrap` | Remove wrappers and restore originals |
| `ribbin activate --global` | Enable wrappers globally |
| `ribbin deactivate --global` | Disable wrappers globally |
| `ribbin status` | Show current activation status |
| `ribbin recover` | Restore orphaned wrapped binaries |
| `ribbin config show` | Show effective config for current directory |

Run `ribbin --help` for all commands and options.

## Documentation

- **[Full Documentation](docs/README.md)** - Complete configuration reference
- **[AI Coding Agents Guide](docs/agent-integration.md)** - Passthrough, bypass patterns, real-world examples
- **[Security Features](docs/security.md)** - Path validation, audit logging, protections
- **[Audit Logging](docs/audit-logging.md)** - Security event tracking

## How It Works

1. `ribbin wrap` renames binaries (e.g., `tsc` → `tsc.ribbin-original`)
2. A symlink to Ribbin takes its place
3. When invoked, Ribbin checks the config and applies the action
4. If no rule matches, the original runs transparently

## Development

```bash
make test              # Run unit tests
make test-integration  # Run integration tests
make scenario          # Interactive Docker testing
```

## License

MIT
