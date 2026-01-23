# How to Allow Commands from Approved Scripts

Use passthrough matching to block direct invocations while allowing commands when run from approved ancestor processes.

## The Problem

You want to block `tsc` when run directly, but allow it when run through `pnpm run typecheck`. Without passthrough, you'd have to modify `package.json` to add `RIBBIN_BYPASS=1`.

## The Solution

Use the `passthrough` option:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "paths": ["./node_modules/.bin/tsc"],
      "passthrough": {
        "invocation": ["pnpm run typecheck", "pnpm run build"]
      }
    }
  }
}
```

> **Note:** For project-local tools like `tsc`, you must specify `paths` since they're not in the system PATH.

Now:
- Direct `tsc` calls are blocked
- `pnpm run typecheck` works (parent process matches)
- No changes to `package.json` needed

## Matching Options

### Substring Matching

`invocation` checks if any substring appears in any ancestor command:

```jsonc
{
  "passthrough": {
    "invocation": ["pnpm run"]
  }
}
```

This allows `tsc` when any ancestor command contains `pnpm run` anywhere.

### Regex Matching

`invocationRegexp` uses [Go regular expressions](https://pkg.go.dev/regexp/syntax):

```jsonc
{
  "passthrough": {
    "invocationRegexp": ["pnpm (run )?(typecheck|build)"]
  }
}
```

This matches:
- `pnpm run typecheck`
- `pnpm typecheck`
- `pnpm run build`
- `pnpm build`

### Combine Both

Use both for different patterns:

```jsonc
{
  "passthrough": {
    "invocation": ["make test"],
    "invocationRegexp": ["poetry run pytest"]
  }
}
```

## Examples

### TypeScript Project

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' or 'pnpm run build'",
      "paths": ["./node_modules/.bin/tsc"],
      "passthrough": {
        "invocationRegexp": ["pnpm (run )?(typecheck|build)"]
      }
    }
  }
}
```

### Python with Poetry

```jsonc
{
  "wrappers": {
    "pytest": {
      "action": "block",
      "message": "Use 'poetry run pytest' or 'make test'",
      "passthrough": {
        "invocation": ["make test"],
        "invocationRegexp": ["poetry run pytest"]
      }
    }
  }
}
```

### ESLint

```jsonc
{
  "wrappers": {
    "eslint": {
      "action": "block",
      "message": "Use 'pnpm run lint'",
      "paths": ["./node_modules/.bin/eslint"],
      "passthrough": {
        "invocationRegexp": ["pnpm (run )?lint"]
      }
    }
  }
}
```

## Ancestor Matching

By default, passthrough checks **all ancestor processes** in the process tree, not just the immediate parent. This handles task runners like nx, turborepo, and make that spawn intermediate processes.

### Example: nx monorepo

When you run `pnpm nx typecheck`, the process tree looks like:

```
pnpm nx typecheck → nx → node worker → tsc
```

With this config, tsc will pass through because "pnpm nx" is found in an ancestor:

```jsonc
{
  "passthrough": {
    "invocation": ["pnpm nx"]
  }
}
```

### Limiting Search Depth

Use `depth` to limit how far up the tree to search:

```jsonc
{
  "passthrough": {
    "invocation": ["pnpm run"],
    "depth": 1
  }
}
```

| depth | behavior |
|-------|----------|
| 0 or omitted | unlimited (check all ancestors) |
| 1 | immediate parent only |
| 2 | parent + grandparent |
| N | up to N ancestors |

## How It Works

When Ribbin intercepts a command:

1. Check if `RIBBIN_BYPASS=1` is set → allow
2. Get ancestor process command lines (up to `depth` limit)
3. Check if any `invocation` substring matches any ancestor → allow
4. Check if any `invocationRegexp` pattern matches any ancestor → allow
5. Apply the configured action (block/warn/redirect)

## Passthrough vs RIBBIN_BYPASS

| Approach | Best For |
|----------|----------|
| `passthrough` | You don't want to modify shared files like package.json |
| `RIBBIN_BYPASS=1` | You control the scripts and prefer explicit bypass |

Both can be used together. `RIBBIN_BYPASS` is checked first.

## Debugging

Check if your pattern matches by running the parent command and checking the audit log:

```bash
pnpm run typecheck
ribbin audit show
```

Look for `bypass.used` or `security.violation` events.

## Troubleshooting

### Task Runner Caching (nx, turborepo, etc.)

**Problem:** You've configured passthrough correctly, but the command is still blocked.

**Cause:** Task runners like nx and turborepo cache command outputs, including failures. If a command was blocked *before* you configured passthrough, the cached failure might be replayed if your runner doesn't know better. This is especially likely if your ribbin config file is not housed in your project, but in a parent directory.

**Solution:** Clear your task runner's cache:

```bash
# nx
pnpm nx reset

# turborepo
pnpm turbo daemon clean
# or delete .turbo directory
```

**How to verify it's a caching issue:**

1. Look for `[local cache]` or similar in the task runner output
2. The block message appears instantly without actually running the command
3. Running with `--skip-cache` works correctly

**Prevention:** When testing ribbin configuration changes, always clear your task runner cache or use `--skip-cache` to ensure you're seeing fresh results.

## See Also

- [Block Commands](block-commands.md) - Basic blocking
- [Integrate with AI Agents](integrate-ai-agents.md) - Full setup guide
- [Configuration Reference](../reference/config-schema.md) - All options
