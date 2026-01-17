# spry-shim

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
# Install globally
bun install -g spry-shim

# In any project
shim init                    # Creates .shimrc.json
shim add tsc "Use 'bun run typecheck' instead"
shim add npm "This project uses bun"
shim install                 # Hooks into shell

# Now in that project directory:
tsc                          # ERROR: Use 'bun run typecheck' instead
SHIM_BYPASS=1 tsc            # Runs real tsc (escape hatch)
bun run typecheck            # Works normally
```

## Config Format

```json
// .shimrc.json
{
  "shims": {
    "tsc": {
      "message": "Use 'bun run typecheck' - direct tsc misses project config",
      "suggest": "bun run typecheck"
    },
    "npm": {
      "message": "This project uses bun",
      "suggest": "bun"
    }
  }
}
```

## Architecture

### Core Technique: Busybox-style Universal Shim

A single binary that:
1. Gets invoked as `tsc`, `npm`, etc. (via symlinks or PATH manipulation)
2. Checks `argv[0]` to know what command was called
3. Looks for `.shimrc.json` in cwd (walking up to find it)
4. Either blocks with message, or execs the real command

### Shell Integration

Two approaches to consider:

**Option A: Global shims directory (always in PATH)**
- `~/.shim/bin/` contains symlinks to the universal shim
- This directory is prepended to PATH via shell hook
- Universal shim checks cwd for `.shimrc.json` on every invocation

**Option B: direnv-style per-directory PATH**
- `shim install` adds a line to shell rc: `eval "$(shim shell-hook)"`
- Shell hook modifies PATH when entering directories with `.shimrc.json`
- Cleaner but requires shell hook to run on every `cd`

### Proposed File Structure

```
spry-shim/
├── src/
│   ├── cli.ts              # Main CLI (init, add, remove, install, list)
│   ├── commands/
│   │   ├── init.ts         # Create .shimrc.json
│   │   ├── add.ts          # Add a shim rule
│   │   ├── remove.ts       # Remove a shim rule
│   │   ├── install.ts      # Set up shell integration
│   │   └── list.ts         # Show active shims
│   ├── shell-hook.ts       # Generates shell integration code
│   ├── shim-runner.ts      # The universal shim logic
│   └── config.ts           # Reads/writes .shimrc.json
├── bin/
│   └── shim                # CLI entry point
└── templates/
    └── shell-hook.sh       # Shell hook template
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `shim init` | Create `.shimrc.json` in current directory |
| `shim add <cmd> "<message>"` | Add a command to block |
| `shim add <cmd> "<message>" --suggest "<alt>"` | Add with suggested alternative |
| `shim remove <cmd>` | Remove a blocked command |
| `shim list` | Show all shims in current project |
| `shim install` | Set up shell integration (add to .bashrc/.zshrc) |
| `shim uninstall` | Remove shell integration |
| `shim shell-hook` | Output shell hook code (for eval) |

## Bypass Mechanisms

Multiple ways to bypass for legitimate use:

1. **Environment variable**: `SHIM_BYPASS=1 tsc`
2. **Full path**: `/usr/local/bin/tsc` (bypasses PATH-based shim)
3. **Command flag** (optional): `tsc --shim-bypass`

## Open Questions

### Naming
- `shim` - simple, describes the technique
- `block` - describes the primary use case
- `guard` - implies protection
- `intercept` - technical but clear

### Shell Support
- bash + zsh (priority)
- fish (nice to have)
- PowerShell (low priority)

### Interactive Mode
Should blocked commands offer to run the suggested command?
```
ERROR: Don't use 'tsc' directly.
Suggestion: bun run typecheck

Run suggested command? [Y/n]
```
Probably not for v1 - keep it simple.

### Config Inheritance
Should `.shimrc.json` in parent directories apply?
- Pro: Monorepo support
- Con: Complexity, surprising behavior

### Per-command Bypass
Allow some commands to have bypass disabled?
```json
{
  "shims": {
    "rm": {
      "message": "Use trash-cli instead",
      "allowBypass": false
    }
  }
}
```

## Implementation Phases

### Phase 1: Core MVP
- [ ] Config file reading/writing
- [ ] Universal shim binary (checks argv[0], reads config, blocks or passes through)
- [ ] Basic CLI: init, add, remove, list
- [ ] Manual PATH setup instructions

### Phase 2: Shell Integration
- [ ] Shell hook generation (bash, zsh)
- [ ] `shim install` / `shim uninstall`
- [ ] Automatic PATH management

### Phase 3: Polish
- [ ] Config validation
- [ ] Better error messages with colors
- [ ] `--suggest` flag with optional "run instead?" prompt
- [ ] fish shell support

### Phase 4: Advanced
- [ ] Config inheritance (parent directories)
- [ ] Glob patterns for commands
- [ ] Conditional shims (only block in certain directories)

## Related/Prior Art

- **busybox** - Single binary, many commands via argv[0] (technique inspiration)
- **direnv** - Per-directory environment, shell hooks (integration pattern)
- **husky** - Git hooks made easy (UX inspiration)
- **asdf/mise** - Version managers with shims (similar shim technique, different purpose)

## Notes

- The universal shim needs to find the "real" command (the one it's shadowing)
- Need to handle the case where the shim is the only thing in PATH
- Consider caching config lookups for performance
- Shell hook should be fast (runs on every prompt/cd)
