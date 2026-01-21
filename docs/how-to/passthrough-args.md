# How to Allow Commands from Approved Scripts

Use passthrough matching to block direct invocations while allowing commands when run from approved parent processes.

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
      "passthrough": {
        "invocation": ["pnpm run typecheck", "pnpm run build"]
      }
    }
  }
}
```

Now:
- Direct `tsc` calls are blocked
- `pnpm run typecheck` works (parent process matches)
- No changes to `package.json` needed

## Matching Options

### Substring Matching

`invocation` checks if any substring appears in the parent command:

```jsonc
{
  "passthrough": {
    "invocation": ["pnpm run"]
  }
}
```

This allows `tsc` when the parent command contains `pnpm run` anywhere.

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
      "passthrough": {
        "invocationRegexp": ["pnpm (run )?lint"]
      }
    }
  }
}
```

## How It Works

When Ribbin intercepts a command:

1. Check if `RIBBIN_BYPASS=1` is set → allow
2. Get the parent process command line
3. Check if any `invocation` substring matches → allow
4. Check if any `invocationRegexp` pattern matches → allow
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

## See Also

- [Block Commands](block-commands.md) - Basic blocking
- [Integrate with AI Agents](integrate-ai-agents.md) - Full setup guide
- [Configuration Reference](../reference/config-schema.md) - All options
