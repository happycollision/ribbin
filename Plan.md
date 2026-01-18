# ribbin

**Like husky, but for shimming/blocking commands with project-specific guidance.**

## Problem Statement

AI agents (and humans) sometimes ignore project-specific instructions and call tools directly (like `tsc`) instead of using the project-configured wrapper (like `bun run typecheck`). This leads to:
- False positives/negatives from misconfigured tool runs
- Confusion about why things don't work
- Repeated mistakes despite documentation

## Solution

A universal tool that intercepts calls to specified commands and either:
1. **Blocks** with a helpful error message explaining what to do instead
2. **Passes through** if bypassed intentionally

## User Experience

```bash
# Install ribbin
curl -fsSL https://raw.githubusercontent.com/happycollision/ribbin/main/install.sh | bash

# In any project
ribbin init                  # Creates ribbin.toml
# Edit ribbin.toml to add your shims
ribbin shim                  # Installs the shims
ribbin activate              # Activates for current shell
ribbin on                    # Or enable globally

# Now in that project directory:
tsc                          # ERROR: Use 'bun run typecheck' instead
RIBBIN_BYPASS=1 tsc          # Runs real tsc (escape hatch)
bun run typecheck            # Works normally
```

## Config Format

```toml
# ribbin.toml
[shims.tsc]
action = "block"
message = "Use 'bun run typecheck' - direct tsc misses project config"

[shims.npm]
action = "block"
message = "This project uses pnpm"

[shims.cat]
action = "block"
message = "Use 'bat' for syntax highlighting"
paths = ["/usr/bin/cat", "/bin/cat"]  # Optional: restrict to specific paths
```

## Architecture

### Core Technique: Busybox-style Universal Shim

A single binary that:
1. Gets invoked as `tsc`, `npm`, etc. (via symlinks replacing the original)
2. Checks `argv[0]` to know what command was called
3. Looks for `ribbin.toml` in cwd (walking up to find it)
4. Either blocks with message, or execs the real command (stored as `.ribbin-original` sidecar)

### Sidecar Approach

When `ribbin shim` is run:
1. Original binary is renamed: `/usr/local/bin/cat` → `/usr/local/bin/cat.ribbin-original`
2. Symlink to ribbin takes its place: `/usr/local/bin/cat` → `ribbin`
3. When invoked, ribbin checks config and either blocks or execs the sidecar

### Activation Modes

- **Shell-scoped** (`ribbin activate`): Only affects the current shell and its children via process ancestry checking
- **Global** (`ribbin on`/`off`): Affects all shells

## Current Implementation

```
ribbin/
├── cmd/ribbin/             # CLI entry point
│   └── main.go
├── internal/
│   ├── cli/                # CLI commands (Cobra)
│   │   ├── cli.go          # Root command
│   │   ├── init.go         # Create ribbin.toml
│   │   ├── shim.go         # Install shims
│   │   ├── unshim.go       # Remove shims
│   │   ├── activate.go     # Shell-scoped activation
│   │   ├── on.go           # Global enable
│   │   └── off.go          # Global disable
│   ├── config/             # Config file parsing
│   │   ├── project.go      # ribbin.toml parsing (TOML)
│   │   └── registry.go     # Global state (~/.config/ribbin/registry.json)
│   ├── shim/               # Shim logic
│   │   ├── installer.go    # Install/uninstall shims
│   │   ├── resolver.go     # Find commands in PATH
│   │   └── runner.go       # Shim execution logic
│   └── process/            # PID ancestry checking
│       ├── ancestry_darwin.go
│       └── ancestry_linux.go
└── testdata/               # Test fixtures
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `ribbin init` | Create `ribbin.toml` in current directory |
| `ribbin shim` | Install shims for all commands in `ribbin.toml` |
| `ribbin unshim` | Remove shims for commands in `ribbin.toml` |
| `ribbin unshim --all` | Remove all shims from registry |
| `ribbin unshim --all --search` | Search and remove all shims |
| `ribbin activate` | Activate for current shell session |
| `ribbin on` | Enable shims globally |
| `ribbin off` | Disable shims globally |

## Bypass Mechanisms

1. **Environment variable**: `RIBBIN_BYPASS=1 tsc`
2. **Direct sidecar**: `/usr/local/bin/tsc.ribbin-original`

## Open Questions / Future Work

### Config Inheritance
Should `ribbin.toml` in parent directories apply?
- Pro: Monorepo support
- Con: Complexity, surprising behavior

### Per-command Bypass
Allow some commands to have bypass disabled?
```toml
[shims.rm]
action = "block"
message = "Use trash-cli instead"
allow_bypass = false
```

### Additional Actions
- `warn`: Print warning but allow execution
- `redirect`: Automatically run suggested command instead

## Related/Prior Art

- **busybox** - Single binary, many commands via argv[0] (technique inspiration)
- **direnv** - Per-directory environment, shell hooks (integration pattern)
- **husky** - Git hooks made easy (UX inspiration)
- **asdf/mise** - Version managers with shims (similar shim technique, different purpose)
