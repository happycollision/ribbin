# Per-Directory Command Shims with mise

Override or block specific commands on a per-project basis using mise and simple shell scripts.

## Why?

Sometimes you need to:
- Block direct use of a command (e.g., `tsc`, `npm`) and show an error with the correct alternative
- Wrap a command with project-specific setup or validation
- Provide project-local scripts that shadow system commands

This pattern uses mise's `_.path` feature to prepend directories to PATH, combined with executable scripts that intercept commands before they reach the real binary.

## Directory Structure

```
myproject/
├── .claude/
│   └── settings.json        # Claude Code hook for mise activation
├── bin-overrides/           # Committed to git, shared with team
│   ├── tsc
│   └── npm
├── bin-overrides.local/     # Gitignored, personal overrides
│   └── kubectl
├── mise.toml
└── .gitignore
```

## Setup

### 1. Create the directories

```bash
mkdir -p bin-overrides bin-overrides.local
```

### 2. Configure mise.toml

```toml
[env]
_.path = ["./bin-overrides.local", "./bin-overrides"]
```

Order matters: `bin-overrides.local` is listed first, so personal overrides take precedence over shared ones.

### 3. Add to .gitignore

```gitignore
bin-overrides.local/
```

### 4. Create your shims

Example blocker (`bin-overrides/tsc`):

```bash
#!/usr/bin/env bash
cat >&2 <<'EOF'
┌─────────────────────────────────────────────────────────┐
│  ERROR: Direct use of tsc is not allowed.               │
│                                                         │
│  Use instead:                                           │
│    pnpm run build       # for production build          │
│    pnpm run typecheck   # for type checking only        │
└─────────────────────────────────────────────────────────┘
EOF
exit 1
```

Example wrapper that adds behavior then calls the real command (`bin-overrides/npm`):

```bash
#!/usr/bin/env bash
echo "WARNING: This project uses pnpm. Proceeding with npm anyway..." >&2

# Remove this script's directory from PATH and call the real npm
PATH="${PATH//:\.\/bin-overrides/}"
PATH="${PATH//:\.\/bin-overrides.local/}"
exec npm "$@"
```

Example passthrough that just logs usage (`bin-overrides.local/kubectl`):

```bash
#!/usr/bin/env bash
echo "[$(date -Iseconds)] kubectl $*" >> ~/.kubectl-audit.log
exec /usr/local/bin/kubectl "$@"
```

### 5. Make scripts executable

```bash
chmod +x bin-overrides/*
chmod +x bin-overrides.local/* 2>/dev/null || true
```

## Agent Setup

For shims to work in non-interactive contexts (CI, agents, IDEs), the shell must activate mise before running commands.

### Claude Code

Create `.claude/settings.json` in your project:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'eval \"$(mise activate bash)\"' >> \"$CLAUDE_ENV_FILE\""
          }
        ]
      }
    ]
  }
}
```

This hook runs at session start and appends mise activation to Claude Code's environment file, which is sourced before every Bash command (including in sub-agents).

### Other Agents

The pattern is the same—find the agent's pre-command hook or shell init mechanism and add:

```bash
eval "$(mise activate bash)"
```

Common options:
- **Cursor**: Uses integrated terminal; add mise activate to your shell rc file
- **VS Code tasks**: Add mise activation to the task's shell command
- **CI/CD**: Run `eval "$(mise activate bash)"` at the start of your script, or use `mise exec --`

### Interactive Shells

Add to `~/.zshrc` or `~/.bashrc`:

```bash
eval "$(mise activate zsh)"  # or bash
```

This is standard mise setup. The prompt hook re-evaluates on directory change.

## Verification

After setup, test that it works:

```bash
cd myproject
which tsc
# Should show: /path/to/myproject/bin-overrides/tsc

tsc
# Should show your error message
```

In Claude Code (start a new session after adding the hook):

```
> Run `which tsc` and then `tsc --version`
```

Should show your shim path and error message.

## Tips

### Calling the real binary from a wrapper

If your shim needs to call the original command, you have options:

```bash
# Option 1: Use full path
/usr/local/bin/npm "$@"

# Option 2: Use mise which
"$(mise which npm)" "$@"

# Option 3: Remove shim dirs from PATH temporarily
PATH="${PATH//:*bin-overrides*/}"
exec npm "$@"
```

### Making shims team-wide vs personal

- `bin-overrides/` — Commit these. Good for enforcing team standards.
- `bin-overrides.local/` — Gitignore these. Good for personal workflow customization.

### Conditional behavior

```bash
#!/usr/bin/env bash
# bin-overrides/npm

if [[ "$1" == "install" ]] && [[ ! -f "package-lock.json" ]]; then
  echo "ERROR: This project uses pnpm. Run 'pnpm install' instead." >&2
  exit 1
fi

# Allow other npm commands to pass through
exec "$(mise which npm)" "$@"
```

## Troubleshooting

**Shim not being picked up:**
- Run `mise doctor` — check that mise is activated
- Run `echo $PATH` — verify `bin-overrides` appears before the real binary's location
- Run `mise env` — check that `_.path` is being applied

**Works interactively but not in agent:**
- The agent isn't activating mise. Check the agent's shell init configuration.
- For Claude Code: verify `.claude/settings.json` has the SessionStart hook and start a new session

**"Permission denied" when running shim:**
- Run `chmod +x bin-overrides/your-script`

**Changes to mise.toml not taking effect:**
- In interactive shell: `cd` out and back in, or run `mise activate`
- In agent: start a new session
