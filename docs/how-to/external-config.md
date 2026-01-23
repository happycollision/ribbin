# How to Keep Config Outside the Project

Store ribbin config in a parent directory to keep project directories clean or manage multiple projects from one config.

## Use Case

You want ribbin rules but don't want `ribbin.jsonc` in your source-controlled project:

```
~/projects/
├── ribbin.jsonc        ← Config lives here
└── my-project/         ← Source-controlled project
    ├── .git/
    └── src/
```

## Setup

1. Create `ribbin.jsonc` in the parent directory:

```jsonc
{
  "scopes": {
    "my-project": {
      "path": "my-project",
      "wrappers": {
        "npm": {
          "action": "block",
          "message": "This project uses pnpm"
        }
      }
    }
  }
}
```

2. Run commands from the project directory:

```bash
cd ~/projects/my-project
ribbin wrap     # Finds ../ribbin.jsonc automatically
ribbin activate
```

Ribbin searches parent directories for config files.

## Path Resolution

All paths in `ribbin.jsonc` are relative to the config file, not your working directory.

| Property | Relative To |
|----------|-------------|
| `path` in scopes | Config file |
| `extends` references | Config file |
| `paths` for binaries | Config file |
| `redirect` scripts | Config file |

Example with multiple path types:

```jsonc
{
  "scopes": {
    "my-project": {
      "path": "my-project",              // → ~/projects/my-project
      "extends": ["./shared/base.jsonc"], // → ~/projects/shared/base.jsonc
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use 'pnpm run typecheck'",
          "paths": ["./my-project/node_modules/.bin/tsc"]  // → ~/projects/my-project/node_modules/.bin/tsc
        },
        "node": {
          "action": "redirect",
          "redirect": "./my-project/scripts/node-wrapper.sh"  // → ~/projects/my-project/scripts/node-wrapper.sh
        }
      }
    }
  }
}
```

## Multiple Projects

Manage several projects from one config:

```jsonc
{
  "wrappers": {
    // Shared rules for all projects
    "npm": {
      "action": "block",
      "message": "Use pnpm for all projects"
    }
  },

  "scopes": {
    "web-app": {
      "path": "web-app",
      "extends": ["root"],
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use 'pnpm run typecheck'",
          "paths": ["./web-app/node_modules/.bin/tsc"]
        }
      }
    },

    "api": {
      "path": "api",
      "extends": ["root"],
      "wrappers": {
        "go": {
          "action": "block",
          "message": "Use 'make build'"
        }
      }
    },

    "docs": {
      "path": "docs",
      "wrappers": {
        // No extends - doesn't inherit npm block
      }
    }
  }
}
```

Directory structure:

```
~/projects/
├── ribbin.jsonc
├── web-app/
│   ├── .git/
│   └── node_modules/
├── api/
│   └── .git/
└── docs/
    └── .git/
```

## Verify Config

Check that ribbin finds your config:

```bash
cd ~/projects/my-project
ribbin config show
```

This shows which config file is loaded and what rules apply.

## See Also

- [Per-Directory Rules](monorepo-scopes.md) - Scopes for monorepos
- [Config Inheritance](config-inheritance.md) - Extend from files
- [Configuration Reference](../reference/config-schema.md) - All options
