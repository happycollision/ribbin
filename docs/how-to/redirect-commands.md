# How to Redirect Commands

Execute a custom script instead of the original command.

## Basic Redirect

Use `action: "redirect"` with a script path:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "redirect",
      "redirect": "./scripts/typecheck-wrapper.sh",
      "message": "Using project's TypeScript configuration"
    }
  }
}
```

## Create the Redirect Script

**scripts/typecheck-wrapper.sh:**
```bash
#!/bin/bash
exec "$RIBBIN_ORIGINAL_BIN" --project tsconfig.json "$@"
```

Make it executable:
```bash
chmod +x scripts/typecheck-wrapper.sh
```

## Environment Variables

Your redirect script receives:

| Variable | Description | Example |
|----------|-------------|---------|
| `RIBBIN_ORIGINAL_BIN` | Path to original binary | `/usr/local/bin/tsc.ribbin-original` |
| `RIBBIN_COMMAND` | Command name | `tsc` |
| `RIBBIN_CONFIG` | Path to ribbin.jsonc | `/project/ribbin.jsonc` |
| `RIBBIN_ACTION` | Always `redirect` | `redirect` |

All original arguments are passed as `$@`.

## Path Resolution

- **Relative paths** (e.g., `./scripts/foo.sh`) resolve relative to `ribbin.jsonc`
- **Absolute paths** used as-is

## Example: Add Default Flags

Force specific flags for every invocation:

```bash
#!/bin/bash
# Always use strict mode
exec "$RIBBIN_ORIGINAL_BIN" --strict "$@"
```

## Example: Conditional Logic

Route based on arguments:

```bash
#!/bin/bash
if [[ "$1" == "--watch" ]]; then
    exec pnpm run typecheck:watch
else
    exec "$RIBBIN_ORIGINAL_BIN" --project tsconfig.json "$@"
fi
```

## Example: Add Logging

```bash
#!/bin/bash
echo "[ribbin] Running tsc with project config" >&2
exec "$RIBBIN_ORIGINAL_BIN" --project tsconfig.json "$@"
```

## Example: Check Parent Process

Allow or modify behavior based on caller:

```bash
#!/bin/bash
PARENT_CMD=$(ps -o args= $PPID 2>/dev/null)

case "$PARENT_CMD" in
    *"pnpm run build"*)
        # Production build - add optimizations
        exec "$RIBBIN_ORIGINAL_BIN" --declaration --declarationMap "$@"
        ;;
    *)
        # Default behavior
        exec "$RIBBIN_ORIGINAL_BIN" "$@"
        ;;
esac
```

## Install and Activate

```bash
ribbin wrap
ribbin activate --global
```

## When to Use Redirect vs Block

| Use Case | Action |
|----------|--------|
| Guide to different command | `block` |
| Modify arguments or behavior | `redirect` |
| Complex conditional logic | `redirect` |
| Simple "use X instead" | `block` |

## See Also

- [Block Commands](block-commands.md) - Show error instead of running
- [Passthrough Arguments](passthrough-args.md) - Allow from specific contexts
- [Configuration Reference](../reference/config-schema.md) - All options
