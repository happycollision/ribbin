# How to Create Personal Config Overrides

Use `ribbin.local.jsonc` for personal overrides that aren't committed to the repository.

## Create Local Config

Create `ribbin.local.jsonc` in the same directory as `ribbin.jsonc`:

```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"],
      "wrappers": {
        // Your personal overrides
      }
    }
  }
}
```

## How It Works

When `ribbin.local.jsonc` exists, Ribbin loads it **instead of** `ribbin.jsonc`. Use `extends` to inherit the shared config and add your overrides.

## Add to .gitignore

```bash
echo "ribbin.local.jsonc" >> .gitignore
```

## Example: Disable a Wrapper Locally

Team config blocks `npm`, but you need it for a personal workflow:

**ribbin.local.jsonc:**
```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"],
      "wrappers": {
        "npm": { "action": "passthrough" }
      }
    }
  }
}
```

## Example: Add Personal Wrappers

Add wrappers just for yourself:

**ribbin.local.jsonc:**
```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"],
      "wrappers": {
        // Personal guardrails
        "rm": {
          "action": "warn",
          "message": "Are you sure? Consider using trash."
        }
      }
    }
  }
}
```

## Example: Different Behavior in Subdirectory

Override only for a specific path:

**ribbin.local.jsonc:**
```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"]
    },

    "experiments": {
      "path": "experiments",
      "extends": ["local"],
      "wrappers": {
        // Allow everything in experiments folder
        "npm": { "action": "passthrough" },
        "yarn": { "action": "passthrough" }
      }
    }
  }
}
```

## Verify Your Config

Check what's active:

```bash
ribbin config show
```

## See Also

- [Config Inheritance](config-inheritance.md) - Using extends
- [Monorepo Scopes](monorepo-scopes.md) - Per-directory rules
- [Configuration Reference](../reference/config-schema.md) - All options
