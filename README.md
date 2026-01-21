# Ribbin

Wrap up commands for agents: Block direct tool calls and redirect to project-specific alternatives.

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

This keeps `package.json` unchanged—no `RIBBIN_BYPASS` needed. See the [AI Coding Agents Guide](docs/how-to/integrate-ai-agents.md) for details.

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

See the [Configuration Schema](docs/reference/config-schema.md) for scopes, inheritance, and local overrides.

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
| `ribbin unwrap --all --find` | Find and remove all wrappers including orphaned ones |
| `ribbin activate --global` | Enable wrappers globally |
| `ribbin deactivate --global` | Disable wrappers globally |
| `ribbin status` | Show current activation status |
| `ribbin find` | Find orphaned sidecars and config files |
| `ribbin recover` | Restore orphaned wrapped binaries |
| `ribbin config show` | Show effective config for current directory |

Run `ribbin --help` for all commands and options.

## Documentation

- **[Documentation Index](docs/index.md)** - Full documentation organized by topic
- **[Getting Started](docs/tutorials/getting-started.md)** - Installation and first steps
- **[AI Coding Agents Guide](docs/how-to/integrate-ai-agents.md)** - Passthrough, bypass patterns, real-world examples
- **[CLI Reference](docs/reference/cli-commands.md)** - All commands and options
- **[Security Features](docs/reference/security-features.md)** - Path validation, audit logging, protections

## How It Works

1. `ribbin wrap` renames binaries (e.g., `tsc` → `tsc.ribbin-original`)
2. A symlink to Ribbin takes its place
3. When invoked, Ribbin checks the config and applies the action
4. If no rule matches, the original runs transparently

## What Ribbin Is Not

**Not an AI sandbox or containment system.** Ribbin provides helpful guardrails, not security boundaries. It reminds agents (and humans) to use the right commands—it doesn't prevent a determined actor from bypassing it.

**Not a security hardening tool.** Ribbin is about workflow enforcement and real-time feedback, not preventing malicious behavior. The bypass mechanism (`RIBBIN_BYPASS=1`) exists by design.

**Not a version manager.** Tools like mise, nvm, and asdf manage multiple versions of the same tool. Ribbin intercepts commands to provide feedback and guide you toward project-specific alternatives—it doesn't manage tool versions.

**Not an alias manager.** Shell aliases only work in interactive shells. Ribbin intercepts actual binary execution via PATH manipulation, so it works even when scripts or other tools invoke commands directly.

**Not a permissions system.** Ribbin doesn't control *who* can run commands, just *how* commands are invoked within a project context.

**Not a container or VM.** Commands still run directly on your system with full access. Ribbin adds a decision layer, not isolation.

## Development

```bash
make test              # Run unit tests
make test-integration  # Run integration tests
make scenario          # Interactive Docker testing
```

## License

MIT
