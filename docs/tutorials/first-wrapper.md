# Tutorial: Create Your First Wrapper

This tutorial walks you through creating a wrapper that blocks direct `npm` usage and guides users to use `pnpm` instead.

## What You'll Build

A wrapper that:
- Blocks direct `npm` commands
- Shows a helpful message explaining what to use instead
- Still allows `npm` when explicitly needed

## Prerequisites

- Ribbin installed ([Getting Started](getting-started.md))
- A project directory to work in

## Step 1: Create a Test Project

Create a new directory for this tutorial:

```bash
mkdir ribbin-tutorial
cd ribbin-tutorial
```

## Step 2: Initialize Ribbin

```bash
ribbin init
```

This creates `ribbin.jsonc` with a basic template.

## Step 3: Add Your Wrapper

Open `ribbin.jsonc` and replace its contents with:

```jsonc
{
  "wrappers": {
    "npm": {
      "action": "block",
      "message": "This project uses pnpm, not npm.\n\nUse 'pnpm install' or 'pnpm add <package>' instead."
    }
  }
}
```

Let's break this down:

- `"npm"` - The command name to wrap
- `"action": "block"` - Stop the command and show an error
- `"message"` - The helpful text to display

## Step 4: Find npm's Location

Before wrapping, check where `npm` is installed:

```bash
which npm
```

You'll see something like `/usr/local/bin/npm` or `~/.local/bin/npm`.

## Step 5: Install the Wrapper

```bash
ribbin wrap
```

Ribbin will:
1. Rename `npm` to `npm.ribbin-original`
2. Create a symlink `npm` pointing to Ribbin

You should see:
```
Wrapped: /usr/local/bin/npm
```

## Step 6: Activate Ribbin

```bash
ribbin activate --global
```

## Step 7: Test the Wrapper

Try running npm:

```bash
npm --version
```

You should see:
```
ERROR: Direct use of 'npm' is blocked.

This project uses pnpm, not npm.

Use 'pnpm install' or 'pnpm add <package>' instead.

Bypass: RIBBIN_BYPASS=1 npm ...
```

## Step 8: Test the Bypass

When you legitimately need npm:

```bash
RIBBIN_BYPASS=1 npm --version
```

This runs the original npm command.

## Step 9: Check the Audit Log

See what Ribbin logged:

```bash
ribbin audit show
```

You'll see entries for:
- The wrapper installation
- The blocked `npm --version` attempt
- The bypassed `npm --version` call

## Step 10: Clean Up

Remove the wrapper when you're done:

```bash
ribbin unwrap
ribbin deactivate --global
```

## Summary

You learned to:
1. Create a `ribbin.jsonc` configuration
2. Define a wrapper with `action: "block"`
3. Install wrappers with `ribbin wrap`
4. Activate Ribbin globally
5. Use `RIBBIN_BYPASS=1` for legitimate access
6. Check the audit log

## Next Steps

- [Redirect commands](../how-to/redirect-commands.md) - Run a different command instead of blocking
- [Passthrough matching](../how-to/passthrough-args.md) - Allow commands from specific parent processes
- [Configuration reference](../reference/config-schema.md) - All configuration options
