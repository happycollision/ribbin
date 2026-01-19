# Using Ribbin with Claude Code

Ribbin can be used to enforce project conventions when working with Claude Code (or other AI coding assistants). By shimming commands, you can redirect Claude away from direct tool usage toward project-specific scripts and conventions.

## Why Use Ribbin with Claude Code?

Claude Code has access to shell commands through its Bash tool. While powerful, this can lead to:

- **Inconsistent tool usage**: Claude might use `cat` instead of the Read tool
- **Bypassing project conventions**: Direct `npm` when the project uses `pnpm`
- **Missing project-specific configuration**: Running `tsc` without your tsconfig
- **Unsafe operations**: Using `rm` instead of safer alternatives

Ribbin intercepts these commands and provides helpful guidance, teaching Claude (and human developers) the project's preferred workflows.

## Quick Setup

### 1. Install Ribbin

```bash
go install github.com/happycollision/ribbin/cmd/ribbin@latest
```

### 2. Create Project Configuration

Create `ribbin.toml` in your project root:

```toml
# Encourage using Claude Code's native tools
[shims.cat]
action = "block"
message = "Use Claude Code's Read tool instead of cat for better context handling"
paths = ["/bin/cat", "/usr/bin/cat"]

[shims.grep]
action = "block"
message = "Use Claude Code's Grep tool instead - it has better output formatting"
paths = ["/bin/grep", "/usr/bin/grep"]

[shims.find]
action = "block"
message = "Use Claude Code's Glob tool instead of find for file discovery"
paths = ["/usr/bin/find"]

# Enforce project conventions
[shims.npm]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' or 'pnpm run <script>'"

[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' to use the project's tsconfig settings"

# Safety guardrails
[shims.rm]
action = "block"
message = "Use 'trash' for safe deletion, or be explicit about what you're removing"
paths = ["/bin/rm", "/usr/bin/rm"]
```

### 3. Install Shims

```bash
# Install shims for commands in your PATH
ribbin shim

# For system directories, confirm you understand the implications
sudo ribbin shim --confirm-system-dir
```

### 4. Activate Ribbin

```bash
# For your current shell session
eval "$(ribbin activate)"

# Or enable globally
ribbin on
```

## Common Configurations

### TypeScript/JavaScript Projects

```toml
# Package manager enforcement
[shims.npm]
action = "block"
message = "This project uses pnpm. Use 'pnpm install' or 'pnpm run <script>'"

[shims.yarn]
action = "block"
message = "This project uses pnpm. Use 'pnpm install' or 'pnpm run <script>'"

# TypeScript tooling
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - it uses the project's tsconfig.json"

[shims.eslint]
action = "block"
message = "Use 'pnpm run lint' - it includes project-specific rules"

[shims.prettier]
action = "block"
message = "Use 'pnpm run format' - it uses the project's prettier config"
```

### Python Projects

```toml
# Virtual environment enforcement
[shims.pip]
action = "block"
message = "Use 'poetry add <package>' or 'uv pip install <package>' within the venv"

[shims.python]
action = "redirect"
redirect = ".venv/bin/python"
message = "Redirecting to project's virtual environment Python"

# Testing
[shims.pytest]
action = "block"
message = "Use 'poetry run pytest' or 'make test' to use project settings"
```

### Rust Projects

```toml
[shims.rustc]
action = "block"
message = "Use 'cargo build' instead of calling rustc directly"

[shims.rustfmt]
action = "block"
message = "Use 'cargo fmt' to format with project settings"

[shims.clippy]
action = "block"
message = "Use 'cargo clippy' with project's lint configuration"
```

### Go Projects

```toml
[shims.go]
action = "redirect"
redirect = "./scripts/go-wrapper.sh"
message = "Using project's Go wrapper for consistent environment"

[shims.gofmt]
action = "block"
message = "Use 'make fmt' to format with project conventions"
```

## Redirecting to Claude Code Tools

For commands where Claude Code has a native tool, you can guide it to use the better option:

```toml
# File reading - Claude's Read tool handles context better
[shims.cat]
action = "block"
message = """Use Claude Code's Read tool instead:
- Better token handling for large files
- Automatic line numbers
- Supports images and PDFs
"""

# File searching - Grep tool has structured output
[shims.grep]
action = "block"
message = """Use Claude Code's Grep tool instead:
- Structured output with file paths
- Better for codebase exploration
- Supports glob patterns for filtering
"""

# File finding - Glob tool is purpose-built
[shims.find]
action = "block"
message = """Use Claude Code's Glob tool instead:
- Faster for large codebases
- Results sorted by modification time
- Native glob pattern support
"""

# Text editing - Edit tool is atomic
[shims.sed]
action = "block"
message = """Use Claude Code's Edit tool instead:
- Atomic, verifiable changes
- Automatic backup
- Better error handling
"""
```

## Safety Guardrails

Prevent potentially dangerous operations:

```toml
# Safer file deletion
[shims.rm]
action = "block"
message = """Dangerous: rm can permanently delete files.
- For safe deletion: use 'trash' command
- For cleanup: use 'make clean' if available
- If you must delete: be explicit about the target
"""

# Prevent accidental git operations
[shims.git]
action = "redirect"
redirect = "./scripts/git-wrapper.sh"
message = "Using project's git wrapper for safety checks"

# Block direct database access
[shims.psql]
action = "block"
message = "Use 'make db-shell' for database access with proper credentials"

[shims.mysql]
action = "block"
message = "Use './scripts/db-connect.sh' for safe database access"
```

## CLAUDE.md Integration

Add ribbin instructions to your project's `CLAUDE.md`:

```markdown
# Project Instructions

## Command Conventions

This project uses ribbin to enforce conventions. If a command is blocked,
follow the suggested alternative in the error message.

### Key Conventions

- **Package Manager**: Use `pnpm`, not npm or yarn
- **Type Checking**: Use `pnpm run typecheck`, not direct `tsc`
- **File Reading**: Prefer the Read tool over `cat`
- **Searching**: Use Grep/Glob tools over shell commands
- **Deletion**: Use `trash` or `make clean`, never raw `rm`

### If a Command is Blocked

When ribbin blocks a command, it will show a message explaining the
preferred alternative. Follow that guidance.

### Bypass (Emergency Only)

If you absolutely must bypass a shim:
```bash
RIBBIN_BYPASS=1 command args
```

Use this sparingly and document why the bypass was necessary.
```

## Monitoring Claude's Tool Usage

Use ribbin's audit log to see what commands are being blocked:

```bash
# View recent blocked commands
ribbin audit show --failed

# Summary of security events
ribbin audit summary

# Filter by event type
ribbin audit show --event-type security.violation
```

This helps you:
1. Identify patterns in Claude's tool usage
2. Add new shims for commands you want to redirect
3. Verify that safety guardrails are working

## Troubleshooting

### Claude Keeps Using Blocked Commands

If Claude repeatedly tries blocked commands:

1. Add clearer instructions to `CLAUDE.md`
2. Make the block message more specific
3. Provide a concrete alternative command

### Shims Not Active in Claude's Environment

Ensure ribbin is activated in the shell Claude uses:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
eval "$(ribbin activate)"

# Or enable globally
ribbin on
```

### Claude Uses RIBBIN_BYPASS

If Claude is using the bypass:

1. This indicates the shim messages may be unclear
2. Review and improve the guidance in block messages
3. Consider whether the shim is too restrictive

## Best Practices

1. **Start with guidance, not blocking**: Use clear messages that explain *why* and *what instead*

2. **Be specific in messages**: Instead of "don't use npm", say "use 'pnpm install' or 'pnpm run build'"

3. **Provide alternatives**: Every blocked command should have a clear alternative

4. **Monitor and iterate**: Use the audit log to see what's being blocked and refine your configuration

5. **Document in CLAUDE.md**: Make sure Claude knows about the conventions before it starts working

6. **Allow escape hatches**: The bypass mechanism exists for legitimate edge cases

## Example: Full Project Configuration

Here's a complete `ribbin.toml` for a TypeScript/Node.js project:

```toml
# =============================================================================
# Claude Code Tool Guidance
# =============================================================================

[shims.cat]
action = "block"
message = "Use Claude Code's Read tool - it handles large files and provides line numbers"
paths = ["/bin/cat", "/usr/bin/cat"]

[shims.grep]
action = "block"
message = "Use Claude Code's Grep tool - structured output, better for code exploration"
paths = ["/bin/grep", "/usr/bin/grep"]

[shims.find]
action = "block"
message = "Use Claude Code's Glob tool - faster, sorted by modification time"
paths = ["/usr/bin/find"]

# =============================================================================
# Package Manager
# =============================================================================

[shims.npm]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' or 'pnpm run <script>'"

[shims.yarn]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' or 'pnpm run <script>'"

# =============================================================================
# Build Tools
# =============================================================================

[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' - uses project's tsconfig.json"

[shims.eslint]
action = "block"
message = "Use 'pnpm run lint' - includes project-specific rules and plugins"

[shims.prettier]
action = "block"
message = "Use 'pnpm run format' - uses project's prettier configuration"

# =============================================================================
# Safety
# =============================================================================

[shims.rm]
action = "block"
message = "Use 'trash' for safe deletion, or 'pnpm run clean' for build artifacts"
paths = ["/bin/rm", "/usr/bin/rm"]
```

## See Also

- [Security Overview](security.md) - Ribbin's security features
- [Audit Logging](audit-logging.md) - Monitoring command usage
- [Main README](../README.md) - General ribbin documentation
