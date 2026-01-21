# How to Set Up Ribbin for AI Coding Agents

Configure Ribbin to guide AI agents toward your project's preferred workflows.

## The Problem

AI coding agents sometimes:
- Run `tsc` directly instead of `pnpm run typecheck`
- Use `npm` when your project uses `pnpm`
- Miss project-specific configurations

## Quick Setup

1. **Create config:**

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "paths": ["./node_modules/.bin/tsc"]
    },
    "npm": {
      "action": "block",
      "message": "This project uses pnpm"
    }
  }
}
```

> **Note:** For project-local tools like `tsc` (installed via npm/pnpm), you must specify `paths` since they're not in the system PATH. Global tools like `npm` are found automatically.

2. **Install and activate:**

```bash
ribbin wrap
ribbin activate --global
```

3. **Allow legitimate usage** (choose one approach):

**Option A: Modify package.json**
```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit"
  }
}
```

**Option B: Use passthrough matching**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "paths": ["./node_modules/.bin/tsc"],
      "passthrough": {
        "invocation": ["pnpm run typecheck"]
      }
    }
  }
}
```

## What the Agent Sees

When the agent runs a blocked command:

```
ERROR: Direct use of 'tsc' is blocked.

Use 'pnpm run typecheck' instead

Bypass: RIBBIN_BYPASS=1 tsc ...
```

The agent learns the correct workflow from the message.

## Approach A: RIBBIN_BYPASS in Scripts

Best when you control the scripts and prefer explicit bypass.

**ribbin.jsonc:**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' or 'pnpm run build'"
    }
  }
}
```

**package.json:**
```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit",
    "build": "RIBBIN_BYPASS=1 tsc"
  }
}
```

## Approach B: Passthrough Matching

Best when you don't want to modify shared files.

**ribbin.jsonc:**
```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "paths": ["./node_modules/.bin/tsc"],
      "passthrough": {
        "invocationRegexp": ["pnpm (run )?(typecheck|build)"]
      }
    }
  }
}
```

**package.json (unchanged):**
```json
{
  "scripts": {
    "typecheck": "tsc --noEmit",
    "build": "tsc"
  }
}
```

## Full TypeScript Project Example

```jsonc
{
  "wrappers": {
    // Project-local tools need explicit paths
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' or 'pnpm run build'",
      "paths": ["./node_modules/.bin/tsc"]
    },
    "eslint": {
      "action": "block",
      "message": "Use 'pnpm run lint' - includes project plugins",
      "paths": ["./node_modules/.bin/eslint"]
    },
    "prettier": {
      "action": "block",
      "message": "Use 'pnpm run format' - uses project config",
      "paths": ["./node_modules/.bin/prettier"]
    },
    // Global tools are found automatically
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

## Python Project Example

```jsonc
{
  "wrappers": {
    "pip": {
      "action": "block",
      "message": "Use 'poetry add <package>' to manage dependencies"
    },
    "pytest": {
      "action": "block",
      "message": "Use 'poetry run pytest' or 'make test'",
      "passthrough": {
        "invocation": ["make test"],
        "invocationRegexp": ["poetry run"]
      }
    }
  }
}
```

## Go Project Example

```jsonc
{
  "wrappers": {
    "go": {
      "action": "block",
      "message": "Use 'make build', 'make test', or 'make run'",
      "passthrough": {
        "invocationRegexp": ["make (build|test|run)"]
      }
    }
  }
}
```

## Activation for AI Agents

For AI assistants that don't have persistent shells, use global activation:

```bash
ribbin activate --global
```

This stays active until `ribbin deactivate --global`.

## Monitor Blocked Commands

Check what's being blocked:

```bash
ribbin audit show
ribbin audit summary
```

## See Also

- [Passthrough Arguments](passthrough-args.md) - Detailed passthrough guide
- [Block Commands](block-commands.md) - Blocking options
- [View Audit Logs](view-audit-logs.md) - Monitor activity
