# How Ribbin Works

Understanding Ribbin's architecture and the mechanics of command wrapping.

## The Sidecar Approach

Ribbin uses a "sidecar" pattern where it sits alongside the original binary:

```
Before wrapping:
/usr/local/bin/tsc  →  TypeScript compiler

After wrapping:
/usr/local/bin/tsc                →  Symlink to Ribbin
/usr/local/bin/tsc.ribbin-original  →  TypeScript compiler
```

When you run `tsc`, you're actually running Ribbin. Ribbin decides what to do based on your configuration.

## Execution Flow

```
1. User runs: tsc --noEmit
           ↓
2. Shell resolves: /usr/local/bin/tsc
           ↓
3. This is a symlink to Ribbin
           ↓
4. Ribbin starts, determines it was invoked as "tsc"
           ↓
5. Ribbin checks:
   - Is RIBBIN_BYPASS=1 set? → Run original
   - Is activation enabled? → Check config
   - Does passthrough match? → Run original
           ↓
6. Look up "tsc" in ribbin.jsonc
           ↓
7. Apply action:
   - block: Show error, exit 1
   - warn: Show warning, run original
   - redirect: Run redirect script
   - passthrough: Run original
```

## How Wrapping Works

The `ribbin wrap` command:

1. **Finds binaries** - Locates all binaries matching your config
2. **Renames originals** - `tsc` → `tsc.ribbin-original`
3. **Creates symlinks** - `tsc` → path to Ribbin binary
4. **Updates registry** - Records what was wrapped in `~/.config/ribbin/registry.json`

```bash
# Before
$ ls -la /usr/local/bin/tsc
-rwxr-xr-x  1 root  admin  1234  Jan 1 12:00  tsc

# After ribbin wrap
$ ls -la /usr/local/bin/tsc*
lrwxr-xr-x  1 root  admin    42  Jan 18 15:00  tsc -> /usr/local/bin/ribbin
-rwxr-xr-x  1 root  admin  1234  Jan 1 12:00  tsc.ribbin-original
```

## Activation System

Ribbin uses a three-tier activation system:

### Config-Scoped

```bash
ribbin activate ./ribbin.jsonc
```

Only activates wrappers defined in the specified config file.

### Shell-Scoped

```bash
ribbin activate --shell
```

Activates for the current shell session and its children. Sets environment variables that child processes inherit.

### Global

```bash
ribbin activate --global
```

Activates system-wide. Persists across shell sessions. Stored in the registry.

## Registry

The registry (`~/.config/ribbin/registry.json`) tracks:

- Which binaries are wrapped
- Where the originals were renamed
- Activation state

This allows Ribbin to:
- Know what to unwrap
- Recover from errors
- Persist state across sessions

## Config Resolution

When Ribbin intercepts a command, it resolves the effective config:

### Config Discovery Algorithm

1. **Start at current directory** - Begin at the process's working directory
2. **Check for local override first** - Look for `ribbin.local.jsonc`
3. **Fall back to standard config** - If no local file, look for `ribbin.jsonc`
4. **Stop at first match** - Return immediately when a config file is found
5. **Walk up to parent** - If neither exists, move to parent directory
6. **Repeat until root** - Continue until filesystem root is reached

**Key point:** `ribbin.local.jsonc` always takes priority over `ribbin.jsonc` in the same directory. This allows personal overrides without modifying the shared config.

```
/project/
├── ribbin.jsonc          # Shared team config
├── ribbin.local.jsonc    # Personal overrides (gitignored) ← Used if present
└── apps/
    └── frontend/
        └── ribbin.jsonc  # App-specific config ← Used for commands run here
```

### After Config Discovery

1. **Determine scope** - Match current working directory against scope paths
2. **Apply inheritance** - Process `extends` to build merged wrappers
3. **Look up command** - Find the wrapper definition for the invoked command

```
/project/apps/frontend/src/index.ts
                    ↓
        Look for config (local first, then standard)
                    ↓
/project/apps/frontend/ribbin.local.jsonc? No
/project/apps/frontend/ribbin.jsonc? No
/project/apps/ribbin.local.jsonc? No
/project/apps/ribbin.jsonc? No
/project/ribbin.local.jsonc? No
/project/ribbin.jsonc? Yes!
                    ↓
        Current dir matches "frontend" scope?
                    ↓
        Apply scope inheritance
                    ↓
        Look up "tsc" in merged wrappers
```

## Why Symlinks?

Ribbin uses symlinks rather than shell aliases or PATH manipulation because:

1. **Universal** - Works regardless of shell (bash, zsh, fish)
2. **Transparent** - No shell configuration changes needed
3. **Inherited** - Child processes see the same wrappers
4. **AI-compatible** - Works with AI agents that spawn processes directly

## Bypass Mechanism

The `RIBBIN_BYPASS=1` environment variable provides an escape hatch:

```bash
RIBBIN_BYPASS=1 tsc --version
```

This is checked early in the execution flow. When set, Ribbin immediately execs the original binary without checking any configuration.

Use cases:
- Package scripts that need the real binary
- Debugging
- One-off commands where you know what you're doing

## Performance

Ribbin adds minimal overhead:
- ~1ms on Linux
- ~5ms on macOS

The overhead comes from:
1. Loading the registry
2. Finding and parsing config
3. Looking up the command
4. Decision logic

For performance-critical scripts, use `RIBBIN_BYPASS=1`.

## See Also

- [Security Model](security-model.md) - How security is implemented
- [Local Dev Mode](local-dev-mode.md) - Repository-scoped protection
- [Performance](performance.md) - Detailed benchmarks
