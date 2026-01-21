# How to Use Config Inheritance

Use `extends` to inherit wrappers from other sources, reducing duplication across scopes.

## Extend Root Wrappers

```jsonc
{
  "wrappers": {
    "npm": { "action": "block", "message": "Use pnpm" }
  },

  "scopes": {
    "frontend": {
      "path": "apps/frontend",
      "extends": ["root"],  // Inherit root wrappers
      "wrappers": {
        // Additions here
      }
    }
  }
}
```

The `"root"` keyword refers to the top-level `wrappers` object.

## Create Mixins

Mixins are scopes without a `path`—they can only be extended:

```jsonc
{
  "scopes": {
    // Mixin: no path
    "hardened": {
      "wrappers": {
        "rm": { "action": "block", "message": "Use trash" },
        "curl": { "action": "warn", "message": "Use httpie" }
      }
    },

    // Scope that extends the mixin
    "production": {
      "path": "apps/prod",
      "extends": ["root", "root.hardened"],
      "wrappers": { }
    }
  }
}
```

Reference mixins with `root.mixinName`.

## Extend External Files

```jsonc
{
  "scopes": {
    "myapp": {
      "path": "apps/myapp",
      "extends": [
        "./team-configs/security-baseline.jsonc",
        "./team-configs/typescript.jsonc"
      ]
    }
  }
}
```

External files can be relative (to the config file) or absolute paths.

## Inheritance Order

Later entries in `extends` override earlier ones. Local `wrappers` override everything:

```jsonc
{
  "wrappers": {
    "rm": { "action": "warn", "message": "Root warning" }
  },

  "scopes": {
    "hardened": {
      "wrappers": {
        "rm": { "action": "block", "message": "Mixin block" }
      }
    },

    "myapp": {
      "path": "apps/myapp",
      "extends": ["root", "root.hardened"],
      "wrappers": {
        "rm": { "action": "passthrough" }  // Wins
      }
    }
  }
}
```

Order: `root` → `hardened` → local `wrappers`

## Example: Shared Security Baseline

**team-configs/security-baseline.jsonc:**
```jsonc
{
  "wrappers": {
    "rm": { "action": "block", "message": "Use trash for safe deletion" },
    "curl": { "action": "warn", "message": "Prefer httpie or project API client" }
  }
}
```

**ribbin.jsonc:**
```jsonc
{
  "wrappers": {
    "npm": { "action": "block", "message": "Use pnpm" }
  },

  "scopes": {
    "app": {
      "path": "apps/main",
      "extends": [
        "root",
        "./team-configs/security-baseline.jsonc"
      ],
      "wrappers": {
        "tsc": { "action": "block", "message": "Use pnpm run typecheck" }
      }
    }
  }
}
```

The `app` scope inherits:
- `npm` block from root
- `rm` block and `curl` warn from security baseline
- Plus its own `tsc` block

## Mixin vs Scope

| | Has `path` | Can be extended | Applies to directories |
|---|---|---|---|
| **Scope** | Yes | Yes | Yes |
| **Mixin** | No | Yes | No (only via extends) |

## See Also

- [Monorepo Scopes](monorepo-scopes.md) - Per-directory rules
- [Local Overrides](local-overrides.md) - Personal config overrides
- [Configuration Reference](../reference/config-schema.md) - All options
