# Practical Guide: Using Ribbin with AI Coding Agents

This guide demonstrates Ribbin's key features through a practical example: enforcing TypeScript project conventions when working with AI coding agents.

## The Problem

You have a TypeScript project where:
- `tsc` should use your project's `tsconfig.json` via `pnpm run typecheck`
- AI agents sometimes run `tsc` directly, missing project settings
- You want to guide toward the correct workflow, not just block

## Two Approaches

There are two ways to set this up:

### Approach A: Modify Package Scripts (Simpler)

Use `action = "block"` with `RIBBIN_BYPASS` in package.json.

**ribbin.jsonc:**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "TypeScript should be run through the project script:\n\n    pnpm run typecheck\n\nThis ensures tsconfig.json settings are used correctly."
    }
  }
}
```

**package.json:**
```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit",
    "typecheck:watch": "RIBBIN_BYPASS=1 tsc --noEmit --watch",
    "build": "RIBBIN_BYPASS=1 tsc"
  }
}
```

Direct `tsc` calls are blocked. The `RIBBIN_BYPASS=1` prefix in package.json lets the scripts through.

### Approach B: Keep Codebase Unchanged (Passthrough)

If you don't want to modify shared files like package.json, use the `passthrough` option. This blocks direct invocations but allows the command when called from approved parent processes (like pnpm scripts).

**ribbin.jsonc** (can be in parent directory):
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead of running tsc directly",
      "passthrough": {
        "invocation": ["pnpm run typecheck", "pnpm run build"],
        "invocationRegexp": ["pnpm (typecheck|build)"]
      }
    }
  }
}
```

**package.json** (unchanged):
```json
{
  "scripts": {
    "typecheck": "tsc --noEmit",
    "build": "tsc"
  }
}
```

This approach:
- Keeps the codebase unchanged—no `RIBBIN_BYPASS` needed in scripts
- Blocks direct `tsc` calls from agents or the command line
- Allows `tsc` when the parent process matches the passthrough rules
- No wrapper scripts needed—everything is declarative in the config

#### Passthrough Matching

The `passthrough` option checks the parent process command line:

- **`invocation`**: Array of exact substrings to match. If any substring is found in the parent command, the call passes through.
- **`invocationRegexp`**: Array of [Go regular expressions](https://pkg.go.dev/regexp/syntax). If any pattern matches the parent command, the call passes through.

Both arrays are optional—use one or both depending on your needs.

**Examples:**

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use project scripts",
      "passthrough": {
        // Simple substring matching
        "invocation": ["pnpm run"]
      }
    },
    "eslint": {
      "action": "block",
      "message": "Use 'pnpm run lint'",
      "passthrough": {
        // Regex for flexible matching
        "invocationRegexp": ["pnpm (run )?lint"]
      }
    },
    "pytest": {
      "action": "block",
      "message": "Use 'make test' or 'poetry run pytest'",
      "passthrough": {
        // Multiple patterns
        "invocation": ["make test"],
        "invocationRegexp": ["poetry run pytest", "make (test|check)"]
      }
    }
  }
}
```

#### When to Use Passthrough vs RIBBIN_BYPASS

| Approach | Best For |
|----------|----------|
| `RIBBIN_BYPASS=1` | You control the scripts calling the command |
| `passthrough` | You can't or don't want to modify the calling scripts |

Both can be combined—`RIBBIN_BYPASS` is checked first, then passthrough rules.

### Approach C: Redirect Script (Advanced)

For complex logic that can't be expressed declaratively, use `action = "redirect"` to a wrapper script.

**ribbin.jsonc:**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "redirect",
      "redirect": "./scripts/tsc-wrapper.sh"
    }
  }
}
```

**scripts/tsc-wrapper.sh:**
```bash
#!/bin/bash

# Check the full command line of parent process
PARENT_CMD=$(ps -o args= $PPID 2>/dev/null)

# Only allow specific sanctioned commands
case "$PARENT_CMD" in
  *"pnpm run typecheck"*|*"pnpm typecheck"*|*"pnpm run build"*|*"pnpm build"*)
    # Running through approved package.json script - allow
    exec "$RIBBIN_ORIGINAL" "$@"
    ;;
  *)
    # Direct invocation or unapproved command
    echo "Use 'pnpm run typecheck' instead of running tsc directly"
    exit 1
    ;;
esac
```

Use redirect scripts when you need:
- Custom logic beyond pattern matching
- Argument inspection or modification
- Logging or metrics collection
- Different behavior based on arguments

## Activating Ribbin

### For AI Coding Sessions

If your AI assistant doesn't have a persistent shell (like Claude Code), use global activation:

```bash
ribbin activate --global
```

This enables Ribbin system-wide until you run `ribbin deactivate --global`.

### For Persistent Shell Sessions

If your agent has a persistent shell, you can activate per-session:

```bash
ribbin activate --shell
```

This sets up the current shell so Ribbin is active.

### Installing the Wrappers

Before activation works, you need to install the wrappers:

```bash
# Install wrappers for commands in ribbin.jsonc
ribbin wrap
```

## What Happens Now

### Direct `tsc` is blocked:

```
$ tsc --noEmit
✗ tsc is blocked by Ribbin

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

When Ribbin intercepts a command, it checks:

1. Is `RIBBIN_BYPASS=1` set in the environment?
2. If yes, execute the original command
3. If no, apply the configured action (block/warn/redirect)

This allows your npm scripts to use the actual tools while direct invocation is still controlled.

```
Agent runs: tsc           → BLOCKED (no bypass)
pnpm runs:  tsc           → ALLOWED (RIBBIN_BYPASS=1 set by script)
```

## Real-World Configuration Examples

### Full TypeScript Project

```jsonc
{
  "wrappers": {
    // TypeScript - use project scripts
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' or 'pnpm run build'"
    },
    "eslint": {
      "action": "block",
      "message": "Use 'pnpm run lint' - includes project plugins"
    },
    "prettier": {
      "action": "block",
      "message": "Use 'pnpm run format' - uses project config"
    },
    // Package manager - this project uses pnpm
    "npm": {
      "action": "block",
      "message": "This project uses pnpm. Run 'pnpm install' instead."
    },
    "yarn": {
      "action": "block",
      "message": "This project uses pnpm. Run 'pnpm install' instead."
    }
  }
}
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

```jsonc
{
  "wrappers": {
    "pip": {
      "action": "block",
      "message": "Use 'poetry add <package>' to manage dependencies"
    },
    "pytest": {
      "action": "block",
      "message": "Use 'poetry run pytest' or 'make test'"
    }
  }
}
```

### Go Project

```jsonc
{
  "wrappers": {
    "go": {
      "action": "block",
      "message": "Use 'make build', 'make test', or 'make run'"
    }
  }
}
```

In your Makefile:
```makefile
build:
	RIBBIN_BYPASS=1 go build ./...

test:
	RIBBIN_BYPASS=1 go test ./...
```

## Using with Agents

When an AI coding agent encounters a blocked command, it sees the error message and can adapt. No special instructions in your `CLAUDE.md` or agent configuration are needed—the block message itself teaches the agent what to do instead.

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
| `warn` | Show warning message, then run command |
| `redirect` | Run a different command instead |
| `passthrough` | Explicit pass-through action (always allow) |
| `RIBBIN_BYPASS=1` | Skip Ribbin, run original command |
| `cmd.ribbin-original` | Call the original binary directly |

| Passthrough Option | Effect |
|--------------------|--------|
| `passthrough.invocation` | Allow if parent command contains any of these substrings |
| `passthrough.invocationRegexp` | Allow if parent command matches any of these regexes |

## See Also

- [Security Overview](security.md) - Security features and protections
- [Audit Logging](audit-logging.md) - Monitoring blocked commands
- [Main README](../README.md) - Installation and basic usage
