# How to Block Commands

Block specific tools with helpful error messages that guide users to the correct alternative.

## Basic Blocking

Add a wrapper with `action: "block"`:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead",
      "paths": ["./node_modules/.bin/tsc"]
    }
  }
}
```

> **Note:** For project-local tools like `tsc` (in `node_modules/.bin/`), you must specify `paths` since they're not in the system PATH. Global tools like `npm` are found automatically.

When someone runs `tsc`, they see:

```
ERROR: Direct use of 'tsc' is blocked.

Use 'pnpm run typecheck' instead

Bypass: RIBBIN_BYPASS=1 tsc ...
```

## Block Multiple Commands

```jsonc
{
  "wrappers": {
    // Global tools - no paths needed
    "npm": {
      "action": "block",
      "message": "This project uses pnpm"
    },
    "yarn": {
      "action": "block",
      "message": "This project uses pnpm"
    },
    // Project-local tools - paths required
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck'",
      "paths": ["./node_modules/.bin/tsc"]
    }
  }
}
```

## Block Specific Paths

Only block binaries at specific locations:

```jsonc
{
  "wrappers": {
    "curl": {
      "action": "block",
      "message": "Use the project's API client instead",
      "paths": ["/usr/bin/curl", "/usr/local/bin/curl"]
    }
  }
}
```

This leaves other `curl` installations (e.g., in a virtual environment) unaffected.

## Multi-Line Messages

Use `\n` for line breaks in your message:

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

## Install and Activate

After editing `ribbin.jsonc`:

```bash
ribbin wrap
ribbin activate --global
```

## Allow Legitimate Usage

Your project scripts need to run the blocked commands. Two approaches:

**Option 1: Use RIBBIN_BYPASS in scripts**

```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit"
  }
}
```

**Option 2: Use passthrough matching**

See [Passthrough Arguments](passthrough-args.md) to allow commands from approved parent processes without modifying scripts.

## See Also

- [Redirect Commands](redirect-commands.md) - Run a different command instead
- [Passthrough Arguments](passthrough-args.md) - Allow from specific contexts
- [Configuration Reference](../reference/config-schema.md) - All options
