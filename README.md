# ribbin

Block direct tool calls and redirect to project-specific alternatives.

## The Problem

AI agents sometimes ignore project instructions and call tools directly (`tsc`, `npm`, `cat`) instead of using project-configured wrappers (`pnpm run typecheck`, `bat`). This leads to misconfigured tool runs, confusion, and repeated mistakes.

## The Solution

ribbin intercepts calls to specified commands and blocks them with helpful error messages explaining what to do instead.

```
┌─────────────────────────────────────────────────────┐
│  ERROR: Direct use of 'tsc' is blocked.            │
│                                                     │
│  Use 'pnpm run typecheck' instead                  │
│                                                     │
│  Bypass: RIBBIN_BYPASS=1 tsc ...                   │
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

1. Initialize ribbin in your project:

```bash
ribbin init
```

2. Edit the generated `ribbin.toml` to add your shims:

```toml
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' instead"

[shims.npm]
action = "block"
message = "This project uses pnpm"
```

3. Install the shims:

```bash
ribbin shim
```

4. Activate ribbin for your shell:

```bash
ribbin activate
```

5. Enable shims globally:

```bash
ribbin on
```

Now when you (or an AI agent) runs `tsc` in this project, they'll see the helpful error instead.

## Configuration

Create a `ribbin.toml` file in your project root:

```toml
# Block direct tsc usage
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - direct tsc misses project config"

# Block cat, suggest bat
[shims.cat]
action = "block"
message = "Use 'bat' for syntax highlighting"
paths = ["/usr/bin/cat", "/bin/cat"]  # Optional: restrict to specific paths

# Block npm in pnpm projects
[shims.npm]
action = "block"
message = "This project uses pnpm"
```

### Configuration Options

| Field | Description |
|-------|-------------|
| `action` | `"block"` - Display error message and exit, or `"redirect"` - Execute custom script |
| `message` | Custom message explaining what to do instead |
| `paths` | (Optional) Restrict shim to specific binary paths |
| `redirect` | (For redirect action) Path to script to execute instead of original command |

### Redirect Action

Execute a custom script instead of the original command:

```toml
[shims.tsc]
action = "redirect"
redirect = "./scripts/typecheck-wrapper.sh"
message = "Using project's TypeScript configuration"
```

Your redirect script receives:
- All original arguments as script arguments
- Environment variables:
  - `RIBBIN_ORIGINAL_BIN` - Path to original binary (e.g., `/usr/local/bin/tsc.ribbin-original`)
  - `RIBBIN_COMMAND` - Command name (e.g., `tsc`)
  - `RIBBIN_CONFIG` - Path to ribbin.toml
  - `RIBBIN_ACTION` - Always `redirect`

**Example redirect script:**

```bash
#!/bin/bash
# scripts/typecheck-wrapper.sh

# Call original with modified behavior
exec "$RIBBIN_ORIGINAL_BIN" --project tsconfig.json "$@"
```

**Path Resolution:**
- Relative paths (e.g., `./scripts/foo.sh`) resolve relative to `ribbin.toml` directory
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
| `ribbin init` | Create a `ribbin.toml` in the current directory |
| `ribbin shim` | Install shims for commands in `ribbin.toml` |
| `ribbin unshim` | Remove shims and restore original commands |
| `ribbin unshim --all` | Remove all shims tracked in the registry |
| `ribbin activate` | Activate ribbin for the current shell session |
| `ribbin on` | Enable shims globally (all shells) |
| `ribbin off` | Disable shims globally (all shells) |
| `ribbin audit show` | View recent security audit events |
| `ribbin audit summary` | View audit log statistics |
| `ribbin config add` | Add a shim configuration to ribbin.toml |
| `ribbin config remove` | Remove a shim configuration |
| `ribbin config list` | List all configured shims |
| `ribbin config edit` | Edit the config file |

Run `ribbin --help` or `ribbin <command> --help` for detailed usage information.

## How It Works

ribbin uses a "sidecar" approach:

1. When you run `ribbin shim`, it renames the original binary (e.g., `/usr/local/bin/cat` → `/usr/local/bin/cat.ribbin-original`)
2. A symlink to ribbin takes its place at the original path
3. When the command is invoked, ribbin checks:
   - Is ribbin active? (via `activate` or `on`)
   - Is there a `ribbin.toml` in this directory (or any parent)?
   - Is this command configured to be blocked?
4. If blocked: show the error message and exit
5. Otherwise: transparently exec the original binary

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

- **Shell-scoped** (`ribbin activate`): Only affects the current shell and its children. Useful for development sessions.
- **Global** (`ribbin on`/`off`): Affects all shells. Useful when you always want protection.

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

Test ribbin in isolated Docker environments without affecting your host system:

```bash
make scenario                           # Show menu to pick a scenario
make scenario SCENARIO=basic            # Run specific scenario directly
```

**Available scenarios:**

| Scenario | Description |
|----------|-------------|
| `basic` | Block and redirect actions with local wrapper commands |
| `local-dev-mode` | Simulates ribbin in node_modules/.bin - tests repo-only shimming |
| `mixed-permissions` | Demonstrates allowed vs forbidden directory security |
| `scopes` | Directory-based configs (monorepo style) |
| `extends` | Config inheritance from mixins and external files |

Inside the scenario shell, ribbin is pre-installed and you can test commands interactively. Type `exit` to leave.

## Use Cases

### TypeScript Projects

```toml
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - it uses the project's tsconfig"

[shims.node]
action = "block"
message = "Use 'pnpm run' or 'pnpm exec' for consistent environment"
```

### Package Manager Enforcement

```toml
[shims.npm]
action = "block"
message = "This project uses pnpm"

[shims.yarn]
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
message = "Use 'bat' or the Read tool for file contents"
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
