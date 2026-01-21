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

Create a `ribbin.jsonc` file in your project root:

```jsonc
{
  "wrappers": {
    // Block direct tsc usage
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' - direct tsc misses project config"
    },
    // Block npm - this project uses pnpm
    "npm": {
      "action": "block",
      "message": "This project uses pnpm. Run 'pnpm install' or 'pnpm add <pkg>' instead.",
      "paths": ["/usr/local/bin/npm", "/usr/bin/npm"]  // Optional: restrict to specific paths
    }
  }
}
```

### Configuration Options

| Field | Description |
|-------|-------------|
| `action` | `"block"` - Display error message and exit, `"warn"` - Show warning but continue, or `"redirect"` - Execute custom script |
| `message` | Custom message explaining what to do instead |
| `paths` | (Optional) Restrict wrapper to specific binary paths |
| `redirect` | (For redirect action) Path to script to execute instead of original command |

### Redirect Action

Execute a custom script instead of the original command:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "redirect",
      "redirect": "./scripts/typecheck-wrapper.sh",
      "message": "Using project's TypeScript configuration"
    }
  }
}
```

Your redirect script receives:
- All original arguments as script arguments
- Environment variables:
  - `RIBBIN_ORIGINAL_BIN` - Path to original binary (e.g., `/usr/local/bin/tsc.ribbin-original`)
  - `RIBBIN_COMMAND` - Command name (e.g., `tsc`)
  - `RIBBIN_CONFIG` - Path to ribbin.jsonc
  - `RIBBIN_ACTION` - Always `redirect`

**Example redirect script:**

```bash
#!/bin/bash
# scripts/typecheck-wrapper.sh

# Call original with modified behavior
exec "$RIBBIN_ORIGINAL_BIN" --project tsconfig.json "$@"
```

**Path Resolution:**
- Relative paths (e.g., `./scripts/foo.sh`) resolve relative to `ribbin.jsonc` directory
- Absolute paths (e.g., `/usr/local/bin/custom`) used as-is
- Script must be executable (`chmod +x script.sh`)

**Common Use Cases:**
- Enforce specific flags or configuration files
- Redirect to alternative tools (npm → pnpm)
- Add environment setup before running tools
- Log command usage for auditing

### Redirect vs Block

**Use redirect when you want to:**
- Automatically fix the command (add flags, change tool)
- Allow the operation to proceed with modifications
- Wrap commands with logging or environment setup

**Use block when you want to:**
- Prevent the operation entirely
- Ensure AI agents learn the correct command via the error message
- Avoid hiding what's actually being executed

Example: For TypeScript, you might redirect `tsc` to automatically add `--project tsconfig.json`, or block it to ensure agents use `pnpm run typecheck` which may do additional steps like linting.

## Commands

| Command | Description |
|---------|-------------|
| `ribbin init` | Create a `ribbin.jsonc` in the current directory |
| `ribbin wrap` | Install wrappers for commands in `ribbin.jsonc` |
| `ribbin unwrap` | Remove wrappers and restore original commands |
| `ribbin unwrap --global` | Remove all wrappers tracked in the registry |
| `ribbin activate --shell` | Activate Ribbin for the current shell session |
| `ribbin activate --global` | Enable wrappers globally (all shells) |
| `ribbin deactivate --global` | Disable wrappers globally |
| `ribbin deactivate --everything` | Clear all activation state |
| `ribbin status` | Show current activation status |
| `ribbin recover` | Find and restore orphaned wrapped binaries |
| `ribbin audit show` | View recent security audit events |
| `ribbin audit summary` | View audit log statistics |
| `ribbin config add` | Add a wrapper configuration to ribbin.jsonc |
| `ribbin config remove` | Remove a wrapper configuration |
| `ribbin config list` | List all configured wrappers |
| `ribbin config show` | Show effective configuration for current directory |
| `ribbin config edit` | Edit the config file |

Run `ribbin --help` or `ribbin <command> --help` for detailed usage information.

## How It Works

Ribbin uses a sidecar approach:

1. When you run `ribbin wrap`, it renames the original binary (e.g., `/usr/local/bin/cat` → `/usr/local/bin/cat.ribbin-original`)
2. A symlink to Ribbin takes its place at the original path
3. When the command is invoked, Ribbin checks:
   - Is Ribbin active? (via `activate --shell` or `activate --global`)
   - Is there a `ribbin.jsonc` in this directory (or any parent)?
   - Is this command configured to be blocked/warned/redirected?
4. If blocked: show the error message and exit
5. If warned: show the warning and continue to original
6. If redirected: execute the redirect script
7. Otherwise: transparently exec the original binary

## Bypass

When you legitimately need to run the original command:

```bash
RIBBIN_BYPASS=1 cat file.txt
```

Or use the full path to the original:

```bash
/usr/bin/cat.ribbin-original file.txt
```

## Activation Modes

Ribbin uses a three-tier activation system:

1. **Config-scoped** (`ribbin activate ./ribbin.jsonc`): Only activates wrappers from specific config files. Most precise control.
2. **Shell-scoped** (`ribbin activate --shell`): Activates all wrappers for the current shell and its children. Useful for development sessions.
3. **Global** (`ribbin activate --global`): Affects all shells system-wide. Useful when you always want protection.

Use `ribbin status` to see current activation state and `ribbin deactivate` with corresponding flags to turn off.

## Recovery

If Ribbin was uninstalled before running `ribbin unwrap`, your original binaries are still safe! The originals are stored alongside their paths with a `.ribbin-original` suffix.

### Using Ribbin recover

If Ribbin is still installed:

```bash
ribbin recover
```

This searches common binary directories for wrapped binaries and restores them.

### If Ribbin is not installed

The easiest approach is to reinstall Ribbin temporarily:

```bash
go install github.com/happycollision/ribbin/cmd/ribbin@latest
ribbin recover
# Then uninstall if desired
```

### Manual Recovery

You can restore binaries manually. This is essentially what `ribbin recover` does (the real code has additional safety checks):

```bash
# Find wrapped binaries
ls /usr/local/bin/*.ribbin-original ~/.local/bin/*.ribbin-original

# For each one found, restore it:
rm /usr/local/bin/tsc                                    # Remove symlink
mv /usr/local/bin/tsc.ribbin-original /usr/local/bin/tsc # Restore original
rm -f /usr/local/bin/tsc.ribbin-meta                     # Clean up metadata
```

## Audit Logging

Ribbin includes comprehensive security audit logging that tracks:

- Shim installations and uninstallations
- Bypass usage (`RIBBIN_BYPASS=1`)
- Security violations (path traversal, forbidden directories)
- Privileged operations (commands run as root)

**View recent events:**
```bash
ribbin audit show
ribbin audit show --since 7d --type security.violation
```

**View summary statistics:**
```bash
ribbin audit summary
```

The audit log is stored at `~/.local/state/ribbin/audit.log` in JSONL format for easy parsing.

For detailed documentation, see [docs/audit-logging.md](docs/audit-logging.md).

## Development

### Testing

```bash
make test              # Run unit tests in Docker
make test-integration  # Run integration tests
make scenario          # Interactive scenario testing (see below)
```

### Interactive Scenario Testing

Test Ribbin in isolated Docker environments without affecting your host system:

```bash
make scenario                           # Show menu to pick a scenario
make scenario SCENARIO=basic            # Run specific scenario directly
```

**Available scenarios:**

| Scenario | Description |
|----------|-------------|
| `basic` | Block and redirect actions with local wrapper commands |
| `extends` | Config inheritance from mixins and external files |
| `local-dev-mode` | Simulates Ribbin in node_modules/.bin - tests repo-only shimming |
| `mixed-permissions` | Demonstrates allowed vs forbidden directory security |
| `recovery` | Test recovery command |
| `scopes` | Directory-based configs (monorepo style) |

Inside the scenario shell, Ribbin is pre-installed and you can test commands interactively. Type `exit` to leave.

## Use Cases

### TypeScript Projects

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' - it uses the project's tsconfig"
    },
    "node": {
      "action": "block",
      "message": "Use 'pnpm run' or 'pnpm exec' for consistent environment"
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
    },
    "yarn": {
      "action": "block",
      "message": "This project uses pnpm"
    }
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
    },
    "curl": {
      "action": "block",
      "message": "Use the project's API client at ./scripts/api.sh or the built-in fetch tools"
    }
  }
}
```

## Contributing

### Releasing

To create a new release:

```bash
make release VERSION=0.1.0
```

This will:
1. Validate the version format (semver)
2. Update CHANGELOG.md (move Unreleased content to new version)
3. Commit the changelog update
4. Create and push the git tag
5. Trigger GitHub Actions to build binaries and publish the release

**Prerequisites:**
- No uncommitted changes
- Content in the `[Unreleased]` section of CHANGELOG.md
- Valid semver version (e.g., `1.0.0`, `0.1.0-alpha.1`)

## License

MIT
