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
3. **Redirect** `cat` to `bat` for better syntax highlighting

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

# Redirect cat to bat for syntax highlighting
[shims.cat]
action = "redirect"
redirect = "bat"
message = "Using bat for syntax highlighting"
paths = ["/bin/cat", "/usr/bin/cat"]
```

## Step 2: Set Up the Bypass in package.json

The key insight: your `typecheck` script needs to bypass ribbin so it can actually run `tsc`:

```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit",
    "typecheck:watch": "RIBBIN_BYPASS=1 tsc --noEmit --watch",
    "build": "RIBBIN_BYPASS=1 tsc"
  }
}
```

The `RIBBIN_BYPASS=1` prefix tells ribbin to let the command through.

## Step 3: Install and Activate

```bash
# Install shims for commands in ribbin.toml
ribbin shim

# Activate for your shell session
eval "$(ribbin activate)"
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

### `cat` redirects to `bat`:

```
$ cat src/index.ts
# Actually runs: bat src/index.ts
# Shows syntax-highlighted output
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

# Better alternatives
[shims.cat]
action = "redirect"
redirect = "bat"
paths = ["/bin/cat", "/usr/bin/cat"]

# Safety
[shims.rm]
action = "block"
message = "Use 'trash' for safe deletion, or 'pnpm run clean' for build artifacts"
paths = ["/bin/rm", "/usr/bin/rm"]
```

With corresponding `package.json`:

```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit",
    "build": "RIBBIN_BYPASS=1 tsc",
    "lint": "RIBBIN_BYPASS=1 eslint src/",
    "format": "RIBBIN_BYPASS=1 prettier --write src/",
    "clean": "RIBBIN_BYPASS=1 rm -rf dist/"
  }
}
```

### Python Project with Poetry

```toml
[shims.pip]
action = "block"
message = "Use 'poetry add <package>' to manage dependencies"

[shims.python]
action = "redirect"
redirect = "poetry run python"
message = "Running Python through Poetry's virtualenv"

[shims.pytest]
action = "block"
message = "Use 'poetry run pytest' or 'make test'"
```

### Go Project

```toml
[shims.go]
action = "block"
message = "Use 'make build', 'make test', or 'make run'"

# Allow go through make
# In Makefile: RIBBIN_BYPASS=1 go build ./...
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

## See Also

- [Security Overview](security.md) - Security features and protections
- [Audit Logging](audit-logging.md) - Monitoring blocked commands
- [Main README](../README.md) - Installation and basic usage
