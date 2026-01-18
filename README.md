# ribbin

Block direct tool calls and redirect to project-specific alternatives.

## The Problem

AI agents (and humans) sometimes ignore project instructions and call tools directly (`tsc`, `npm`, `cat`) instead of using project-configured wrappers (`pnpm run typecheck`, `bat`). This leads to misconfigured tool runs, confusion, and repeated mistakes.

## The Solution

ribbin intercepts calls to specified commands and blocks them with helpful error messages explaining what to do instead.

```
┌─────────────────────────────────────────────────────┐
│  ERROR: Direct use of 'tsc' is blocked.            │
│                                                     │
│  Use 'pnpm run typecheck' instead                  │
│                                                     │
│  Bypass: RIBBIN_BYPASS=1 tsc ...                   │
└─────────────────────────────────────────────────────┘
```

## Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/happycollision/ribbin/main/install.sh | bash
```

### From Source

```bash
go install github.com/happycollision/ribbin/cmd/ribbin@latest
```

### Manual Download

Download the latest release from [GitHub Releases](https://github.com/happycollision/ribbin/releases).

## Quick Start

1. Initialize ribbin in your project:

```bash
ribbin init
```

2. Edit the generated `ribbin.toml` to add your shims:

```toml
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' instead"

[shims.npm]
action = "block"
message = "This project uses pnpm"
```

3. Install the shims:

```bash
ribbin shim
```

4. Activate ribbin for your shell:

```bash
ribbin activate
```

5. Enable shims globally:

```bash
ribbin on
```

Now when you (or an AI agent) runs `tsc` in this project, they'll see the helpful error instead.

## Configuration

Create a `ribbin.toml` file in your project root:

```toml
# Block direct tsc usage
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - direct tsc misses project config"

# Block cat, suggest bat
[shims.cat]
action = "block"
message = "Use 'bat' for syntax highlighting"
paths = ["/usr/bin/cat", "/bin/cat"]  # Optional: restrict to specific paths

# Block npm in pnpm projects
[shims.npm]
action = "block"
message = "This project uses pnpm"
```

### Configuration Options

| Field | Description |
|-------|-------------|
| `action` | `"block"` - Display error message and exit |
| `message` | Custom message explaining what to do instead |
| `paths` | (Optional) Restrict shim to specific binary paths |

## Commands

| Command | Description |
|---------|-------------|
| `ribbin init` | Create a `ribbin.toml` in the current directory |
| `ribbin shim` | Install shims for all commands in `ribbin.toml` |
| `ribbin unshim` | Remove shims for commands in `ribbin.toml` |
| `ribbin activate` | Activate ribbin for the current shell session |
| `ribbin on` | Enable shims globally |
| `ribbin off` | Disable shims globally |

## How It Works

ribbin uses a "sidecar" approach:

1. When you run `ribbin shim`, it renames the original binary (e.g., `/usr/local/bin/cat` → `/usr/local/bin/cat.ribbin-original`)
2. A symlink to ribbin takes its place at the original path
3. When the command is invoked, ribbin checks:
   - Is ribbin active? (via `activate` or `on`)
   - Is there a `ribbin.toml` in this directory (or any parent)?
   - Is this command configured to be blocked?
4. If blocked: show the error message and exit
5. Otherwise: transparently exec the original binary

## Bypass

When you legitimately need to run the original command:

```bash
RIBBIN_BYPASS=1 cat file.txt
```

Or use the full path to the original:

```bash
/usr/bin/cat.ribbin-original file.txt
```

## Activation Modes

- **Shell-scoped** (`ribbin activate`): Only affects the current shell and its children. Useful for development sessions.
- **Global** (`ribbin on`/`off`): Affects all shells. Useful when you always want protection.

## Use Cases

### TypeScript Projects

```toml
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - it uses the project's tsconfig"

[shims.node]
action = "block"
message = "Use 'pnpm run' or 'pnpm exec' for consistent environment"
```

### Package Manager Enforcement

```toml
[shims.npm]
action = "block"
message = "This project uses pnpm"

[shims.yarn]
action = "block"
message = "This project uses pnpm"
```

### AI Agent Guardrails

```toml
[shims.rm]
action = "block"
message = "Use 'trash' for safe deletion"

[shims.cat]
action = "block"
message = "Use 'bat' or the Read tool for file contents"
```

## License

MIT
