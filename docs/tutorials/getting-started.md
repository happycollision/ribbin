# Getting Started with Ribbin

This tutorial walks you through setting up Ribbin to block direct tool calls and redirect to your project's preferred commands.

## What You'll Learn

By the end of this tutorial, you will:
- Install Ribbin on your machine
- Create a configuration file for your project
- Install wrappers for specific commands
- Activate Ribbin so wrappers take effect

## Prerequisites

- A Unix-like system (Linux or macOS)
- A project where you want to enforce tool usage patterns

## Step 1: Install Ribbin

Choose one of these installation methods:

**Quick Install (Linux/macOS):**
```bash
curl -fsSL https://raw.githubusercontent.com/happycollision/ribbin/main/install.sh | bash
```

**From Source (requires Go):**
```bash
go install github.com/happycollision/ribbin/cmd/ribbin@latest
```

**Manual Download:**
Download the latest release from [GitHub Releases](https://github.com/happycollision/ribbin/releases).

Verify the installation:
```bash
ribbin --version
```

## Step 2: Initialize Your Project

Navigate to your project directory and run:

```bash
cd /path/to/your/project
ribbin init
```

This creates a `ribbin.jsonc` file in your project root.

## Step 3: Configure Your Wrappers

Open `ribbin.jsonc` and add the commands you want to control. Here's a simple example that blocks direct `tsc` usage:

```jsonc
{
  "wrappers": {
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead"
    }
  }
}
```

Save the file.

## Step 4: Install the Wrappers

Run the wrap command to install your configured wrappers:

```bash
ribbin wrap
```

Ribbin will:
1. Find the `tsc` binary on your system
2. Rename it to `tsc.ribbin-original`
3. Create a symlink `tsc` that points to Ribbin

## Step 5: Activate Ribbin

For wrappers to take effect, you need to activate Ribbin:

```bash
ribbin activate --global
```

This enables Ribbin system-wide. Now when anyone (including AI agents) runs `tsc`, they'll see your helpful message instead.

## Step 6: Test It

Try running the blocked command:

```bash
tsc --version
```

You should see:
```
ERROR: Direct use of 'tsc' is blocked.

Use 'pnpm run typecheck' instead

Bypass: RIBBIN_BYPASS=1 tsc ...
```

## Step 7: Allow Legitimate Usage

Your project scripts still need to run `tsc`. Update your `package.json` to bypass Ribbin:

```json
{
  "scripts": {
    "typecheck": "RIBBIN_BYPASS=1 tsc --noEmit"
  }
}
```

Now `pnpm run typecheck` works normally while direct `tsc` calls are blocked.

## What's Next?

You've successfully set up Ribbin. Here's where to go next:

- [Create your first wrapper](first-wrapper.md) - A step-by-step guide to creating wrappers
- [Block commands](../how-to/block-commands.md) - Block specific tools with helpful messages
- [Set up passthrough](../how-to/passthrough-args.md) - Allow commands from approved scripts without modifying them
- [CLI reference](../reference/cli-commands.md) - All available commands and options

## Quick Reference

| Command | Description |
|---------|-------------|
| `ribbin init` | Create a `ribbin.jsonc` in the current directory |
| `ribbin wrap` | Install wrappers for commands in config |
| `ribbin unwrap` | Remove wrappers and restore originals |
| `ribbin activate --global` | Enable wrappers globally |
| `ribbin deactivate --global` | Disable wrappers globally |
| `ribbin status` | Show current activation status |
