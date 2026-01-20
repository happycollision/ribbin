#!/bin/bash
# Description: Mixed permission levels (allowed, confirmation-required)

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up mixed-permissions scenario..."
echo ""

# Create local bin directory
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create a local tool (allowed - no confirmation needed)
cat > "$LOCAL_BIN/my-tool" << 'EOF'
#!/bin/bash
echo "my-tool: running locally"
EOF
chmod +x "$LOCAL_BIN/my-tool"

# Create scenario directory structure
mkdir -p "$SCENARIO_DIR"/scripts
cd "$SCENARIO_DIR"

# Initialize git repo
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Create a ribbin.toml with mixed permission levels
cat > ribbin.toml << EOF
# Mixed permissions scenario
# This config has wrappers targeting different permission levels

# ALLOWED: ~/.local/bin - no confirmation needed
[wrappers.my-tool]
action = "block"
message = "Use 'npm run tool' instead"
paths = ["$LOCAL_BIN/my-tool"]

# ALLOWED: /usr/local/bin - no confirmation needed
[wrappers.jq]
action = "warn"
message = "Consider using gojq for better performance"
# Will resolve from PATH to /usr/local/bin/jq (allowed)

# REQUIRES CONFIRMATION: /bin, /usr/bin - needs --confirm-system-dir
[wrappers.cat]
action = "block"
message = "Use bat instead"
# Will resolve from PATH to /bin/cat (requires confirmation)

[wrappers.ls]
action = "warn"
message = "Use exa for better output"
# Will resolve from PATH to /bin/ls (requires confirmation)
EOF

cat > README.md << 'EOF'
# Mixed Permissions Scenario

This scenario demonstrates ribbin's permission levels:

## Permission Levels

1. **ALLOWED** - No confirmation needed
   - `~/.local/bin/*`
   - `~/go/bin/*`
   - `/usr/local/bin/*`
   - `/opt/homebrew/bin/*`
   - `./node_modules/.bin/*`

2. **REQUIRES CONFIRMATION** - Needs `--confirm-system-dir` flag
   - `/bin/*`
   - `/usr/bin/*`
   - `/sbin/*`

3. **ALWAYS BLOCKED** - Critical binaries blocked by name
   - `bash`, `sh`, `zsh`, `fish` (shells)
   - `sudo`, `su`, `doas` (privilege escalation)
   - `ssh`, `sshd` (remote access)

## In this scenario:

- `my-tool` → `~/.local/bin/my-tool` → **ALLOWED**
- `jq` → `/usr/local/bin/jq` → **ALLOWED**
- `cat` → `/bin/cat` → **REQUIRES CONFIRMATION**
- `ls` → `/bin/ls` → **REQUIRES CONFIRMATION**

## Try these commands:

1. Run without confirmation flag:
   ```
   ribbin wrap
   ```
   Only `my-tool` and `jq` should succeed. `cat` and `ls` will fail.

2. Run with confirmation flag:
   ```
   ribbin wrap --confirm-system-dir
   ```
   Now `cat` and `ls` will also be wrapped (we're root in Docker).

3. Check what got wrapped:
   ```
   ls -la ~/.local/bin/
   ls -la /bin/cat /bin/ls
   ```

4. Unwrap everything:
   ```
   ribbin unwrap
   ```

## Key insight:

ribbin allows wrapping system directories with explicit confirmation.
Critical binaries (bash, sudo, ssh, etc.) are always blocked by name,
regardless of the --confirm-system-dir flag.
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Mixed Permissions Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo ""
echo "Permission levels in this config:"
echo "  ALLOWED:              my-tool -> ~/.local/bin/my-tool"
echo "  ALLOWED:              jq      -> /usr/local/bin/jq"
echo "  REQUIRES CONFIRMATION: cat     -> /bin/cat"
echo "  REQUIRES CONFIRMATION: ls      -> /bin/ls"
echo ""
echo "Try: ribbin wrap"
echo "  - my-tool and jq will succeed (allowed directories)"
echo "  - cat and ls will fail (need --confirm-system-dir)"
echo ""
echo "Then try: ribbin wrap --confirm-system-dir"
echo "  - All four will succeed"
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
