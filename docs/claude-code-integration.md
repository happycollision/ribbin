# Practical Guide: Using Ribbin with AI Coding Assistants

This guide demonstrates ribbin's key features through a practical example: enforcing TypeScript project conventions when working with Claude Code or other AI assistants.

## The Problem

You have a TypeScript project where:
- `tsc` should use your project's `tsconfig.json` via `pnpm run typecheck`
- Developers (and AI assistants) sometimes run `tsc` directly, missing project settings
- You want to guide toward the correct workflow, not just block

## Solution Overview

We'll set up ribbin to:
1. **Block** direct `tsc` calls with a helpful message
2. **Allow** `tsc` when called from `pnpm run typecheck` (via bypass)

## Step 1: Create the Configuration

Create `ribbin.toml` in your project root:

```toml
# Block direct tsc usage - guide to project script
[shims.tsc]
action = "block"
message = """TypeScript should be run through the project script:

    pnpm run typecheck

This ensures tsconfig.json settings are used correctly.
"""
```

## Step 2: Set Up the Bypass

Your `typecheck` script needs to bypass ribbin so it can actually run `tsc`. There are two approaches:

### Option A: Environment Variable Bypass

The simplest approach - prefix with `RIBBIN_BYPASS=1`:

```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit",
    "typecheck:watch": "RIBBIN_BYPASS=1 tsc --noEmit --watch",
    "build": "RIBBIN_BYPASS=1 tsc"
  }
}
```

### Option B: Redirect to Wrapper Script

For more control, use `redirect` to a script that checks how it was invoked:

```toml
# ribbin.toml
[shims.tsc]
action = "redirect"
redirect = "./scripts/tsc-wrapper.sh"
```

```bash
#!/bin/bash
# scripts/tsc-wrapper.sh

# Get the parent process name
PARENT=$(ps -o comm= $PPID 2>/dev/null)

# Allow if run through npm/pnpm/yarn
case "$PARENT" in
  npm|pnpm|yarn|node)
    # Running through package manager - call original via RIBBIN_ORIGINAL
    exec "$RIBBIN_ORIGINAL" "$@"
    ;;
  *)
    # Direct invocation - show guidance
    echo "Use 'pnpm run typecheck' instead of running tsc directly"
    exit 1
    ;;
esac
```

Then in `package.json` (no bypass needed - the wrapper handles it):
```json
{
  "scripts": {
    "typecheck": "tsc --noEmit"
  }
}
```

This approach:
- Uses `RIBBIN_ORIGINAL` environment variable (set by ribbin for redirect scripts)
- Checks the parent process to determine if run via package manager
- No `RIBBIN_BYPASS` needed in package.json - the logic is in the wrapper

## Step 3: Activate Ribbin

### For AI Coding Sessions

If your AI assistant doesn't have a persistent shell (like Claude Code), use global activation:

```bash
ribbin on
```

This enables ribbin system-wide until you run `ribbin off`.

### For Persistent Shell Sessions

If your agent has a persistent shell, you can activate per-session:

```bash
ribbin activate
```

This sets up the current shell so ribbin is active. For human developers, add to your shell profile (`.bashrc`, `.zshrc`):

```bash
# Activate ribbin if available
command -v ribbin >/dev/null && ribbin activate
```

### Installing the Shims

Before activation works, you need to install the shims:

```bash
# Install shims for commands in ribbin.toml
ribbin shim
```

## What Happens Now

### Direct `tsc` is blocked:

```
$ tsc --noEmit
✗ tsc is blocked by ribbin

TypeScript should be run through the project script:

    pnpm run typecheck

This ensures tsconfig.json settings are used correctly.
```

### Project script works (bypass):

```
$ pnpm run typecheck
> RIBBIN_BYPASS=1 tsc --noEmit

# tsc runs normally with project settings
```

## How the Bypass Works

When ribbin intercepts a command, it checks:

1. Is `RIBBIN_BYPASS=1` set in the environment?
2. If yes, execute the original command
3. If no, apply the configured action (block/redirect)

This allows your npm scripts to use the actual tools while direct invocation is still controlled.

```
Developer runs: tsc           → BLOCKED (no bypass)
pnpm runs:      tsc           → ALLOWED (RIBBIN_BYPASS=1 set by script)
```

## Real-World Configuration Examples

### Full TypeScript Project

```toml
# TypeScript - use project scripts
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' or 'pnpm run build'"

[shims.eslint]
action = "block"
message = "Use 'pnpm run lint' - includes project plugins"

[shims.prettier]
action = "block"
message = "Use 'pnpm run format' - uses project config"

# Package manager - this project uses pnpm
[shims.npm]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' instead."

[shims.yarn]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' instead."
```

With corresponding `package.json`:

```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit",
    "build": "RIBBIN_BYPASS=1 tsc",
    "lint": "RIBBIN_BYPASS=1 eslint src/",
    "format": "RIBBIN_BYPASS=1 prettier --write src/"
  }
}
```

### Python Project with Poetry

```toml
[shims.pip]
action = "block"
message = "Use 'poetry add <package>' to manage dependencies"

[shims.pytest]
action = "block"
message = "Use 'poetry run pytest' or 'make test'"
```

### Go Project

```toml
[shims.go]
action = "block"
message = "Use 'make build', 'make test', or 'make run'"
```

In your Makefile:
```makefile
build:
	RIBBIN_BYPASS=1 go build ./...

test:
	RIBBIN_BYPASS=1 go test ./...
```

## Using with Claude Code

When Claude Code encounters a blocked command, it sees the error message and can adapt. Add guidance to your `CLAUDE.md`:

```markdown
## Build Commands

This project uses ribbin to enforce conventions. If a command is blocked:
- Follow the suggestion in the error message
- Use the project scripts in package.json

Key commands:
- Type checking: `pnpm run typecheck`
- Building: `pnpm run build`
- Linting: `pnpm run lint`
- Formatting: `pnpm run format`
```

## Checking What's Blocked

View the audit log to see blocked commands:

```bash
# Recent events
ribbin audit show

# Just failures
ribbin audit show --failed

# Summary statistics
ribbin audit summary
```

## Quick Reference

| Action | Effect |
|--------|--------|
| `block` | Show error message, don't run command |
| `redirect` | Run a different command instead |
| `RIBBIN_BYPASS=1` | Skip ribbin, run original command |
| `cmd.ribbin-original` | Call the original binary directly |

## See Also

- [Security Overview](security.md) - Security features and protections
- [Audit Logging](audit-logging.md) - Monitoring blocked commands
- [Main README](../README.md) - Installation and basic usage
