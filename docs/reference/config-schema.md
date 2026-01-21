# Configuration Schema Reference

Complete reference for `ribbin.jsonc` configuration.

## File Format

Ribbin uses JSONC (JSON with Comments). Comments start with `//`.

```jsonc
{
  // This is a comment
  "wrappers": {}
}
```

## Schema

A JSON Schema is available at `ribbin.schema.json` for editor autocompletion.

```jsonc
{
  "$schema": "https://github.com/happycollision/ribbin/ribbin.schema.json"
}
```

## Top-Level Properties

| Property | Type | Description |
|----------|------|-------------|
| `$schema` | string | Optional schema URL for editor support |
| `wrappers` | object | Command wrapper definitions |
| `scopes` | object | Directory-specific configurations |

## Wrapper Definition

Each wrapper is keyed by command name:

```jsonc
{
  "wrappers": {
    "command-name": {
      "action": "block",
      "message": "...",
      "paths": [],
      "redirect": "",
      "passthrough": {}
    }
  }
}
```

### action (required)

| Value | Behavior |
|-------|----------|
| `block` | Show error message and exit with code 1 |
| `warn` | Show warning message, then run original command |
| `redirect` | Execute redirect script instead |
| `passthrough` | Always allow (useful for scope overrides) |

### message

Error or warning message to display. Supports `\n` for line breaks.

```jsonc
{
  "message": "Use 'pnpm run typecheck' instead.\n\nThis ensures correct tsconfig."
}
```

### paths

Array of specific binary paths to wrap.

- If omitted, Ribbin searches the system PATH for the command
- **Required for project-local tools** (e.g., `./node_modules/.bin/tsc`) since they're typically not in the system PATH
- Supports relative paths (relative to config file) or absolute paths

```jsonc
{
  // Project-local tool - paths required
  "tsc": {
    "action": "block",
    "message": "Use 'pnpm run typecheck'",
    "paths": ["./node_modules/.bin/tsc"]
  },
  // Global tool - paths optional (found via PATH)
  "npm": {
    "action": "block",
    "message": "This project uses pnpm"
  },
  // Multiple specific paths
  "curl": {
    "action": "block",
    "paths": ["/usr/bin/curl", "/usr/local/bin/curl"]
  }
}
```

### redirect

Path to script for `action: "redirect"`. Relative to config file or absolute.

```jsonc
{
  "action": "redirect",
  "redirect": "./scripts/wrapper.sh"
}
```

### passthrough

Allow command when parent process matches patterns.

```jsonc
{
  "passthrough": {
    "invocation": ["pnpm run"],
    "invocationRegexp": ["make (test|build)"]
  }
}
```

| Property | Type | Description |
|----------|------|-------------|
| `invocation` | string[] | Substrings to match in parent command |
| `invocationRegexp` | string[] | Regex patterns to match parent command |

## Scope Definition

Scopes define directory-specific rules:

```jsonc
{
  "scopes": {
    "scope-name": {
      "path": "relative/path",
      "extends": [],
      "wrappers": {}
    }
  }
}
```

### path

Directory this scope applies to, relative to config file. Omit for mixins.

```jsonc
{
  "path": "apps/frontend"
}
```

### extends

Array of sources to inherit wrappers from:

| Value | Description |
|-------|-------------|
| `"root"` | Top-level wrappers |
| `"root.scopeName"` | Another scope (mixin) |
| `"./path/to/file.jsonc"` | External config file |

```jsonc
{
  "extends": ["root", "root.hardened", "./team/base.jsonc"]
}
```

### wrappers

Scope-specific wrapper definitions. Override inherited wrappers.

## Complete Example

```jsonc
{
  "$schema": "https://github.com/happycollision/ribbin/ribbin.schema.json",

  "wrappers": {
    // Global tools - found via PATH
    "npm": {
      "action": "block",
      "message": "This project uses pnpm"
    },
    // Project-local tools - paths required
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck'",
      "paths": ["./node_modules/.bin/tsc"],
      "passthrough": {
        "invocation": ["pnpm run typecheck"]
      }
    }
  },

  "scopes": {
    // Mixin (no path)
    "hardened": {
      "wrappers": {
        "rm": {
          "action": "block",
          "message": "Use trash for safe deletion"
        }
      }
    },

    // Scope with path
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root", "root.hardened"],
      "wrappers": {
        "yarn": {
          "action": "block",
          "message": "Use pnpm in frontend"
        }
      }
    },

    // Scope that allows npm (override)
    "legacy": {
      "path": "apps/legacy",
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

## Local Override File

`ribbin.local.jsonc` is loaded instead of `ribbin.jsonc` when present:

```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"],
      "wrappers": {
        // Personal overrides
      }
    }
  }
}
```

## See Also

- [CLI Commands](cli-commands.md) - Command reference
- [How to Configure Scopes](../how-to/monorepo-scopes.md) - Scope guide
- [How to Use Inheritance](../how-to/config-inheritance.md) - Extends guide
