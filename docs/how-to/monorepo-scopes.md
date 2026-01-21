# How to Configure Per-Directory Rules

Use scopes to define different wrapper rules for different directories—ideal for monorepos.

## Basic Scope

```jsonc
{
  // Root-level wrappers apply everywhere
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "Use pnpm instead"
    }
  },

  "scopes": {
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root"],
      "wrappers": {
        "yarn": {
          "action": "block",
          "message": "Use pnpm in frontend"
        }
      }
    }
  }
}
```

When working in `apps/frontend`, both `npm` and `yarn` are blocked.

## Scope Properties

| Property | Description |
|----------|-------------|
| `path` | Directory this scope applies to (relative to config file) |
| `extends` | Inherit wrappers from other sources |
| `wrappers` | Wrappers specific to this scope |

## Override Root Wrappers

Scopes can override root-level wrappers:

```jsonc
{
  "wrappers": {
    "rm": {
      "action": "warn",
      "message": "Be careful with rm"
    }
  },

  "scopes": {
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root"],
      "wrappers": {
        // Stricter in frontend
        "rm": {
          "action": "block",
          "message": "Use trash-cli in frontend"
        }
      }
    },

    "backend": {
      "path": "apps/backend",
      "extends": ["root"],
      "wrappers": {
        // Allow npm in backend (legacy)
        "npm": {
          "action": "passthrough"
        }
      }
    }
  }
}
```

## Check Effective Config

See what rules apply in each directory:

```bash
# From project root
ribbin config show

# In frontend
cd apps/frontend && ribbin config show

# In backend
cd apps/backend && ribbin config show
```

## Scope Matching

Ribbin checks the current working directory and applies the most specific matching scope:

```
project/
├── ribbin.jsonc
├── apps/
│   ├── frontend/     → "frontend" scope
│   │   └── src/      → "frontend" scope (inherited)
│   └── backend/      → "backend" scope
└── packages/         → root wrappers only
```

## Multiple Scopes

```jsonc
{
  "wrappers": {
    "npm": { "action": "block", "message": "Use pnpm" }
  },

  "scopes": {
    "web": {
      "path": "apps/web",
      "extends": ["root"],
      "wrappers": {
        "tsc": { "action": "block", "message": "Use 'pnpm run typecheck'" }
      }
    },

    "api": {
      "path": "apps/api",
      "extends": ["root"],
      "wrappers": {
        "go": { "action": "block", "message": "Use 'make build'" }
      }
    },

    "docs": {
      "path": "docs",
      "extends": ["root"],
      "wrappers": {
        // Allow npm for docs site
        "npm": { "action": "passthrough" }
      }
    }
  }
}
```

## See Also

- [Config Inheritance](config-inheritance.md) - Extend from files and mixins
- [Local Overrides](local-overrides.md) - Personal config overrides
- [Configuration Reference](../reference/config-schema.md) - All options
