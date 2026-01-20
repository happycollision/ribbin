#!/bin/bash
# Description: Mixed permission levels (allowed, confirmation-required, forbidden)

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"
SYSTEM_BIN="/usr/local/bin"

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
# This config has shims targeting different permission levels

# ALLOWED: ~/.local/bin - no confirmation needed
[shims.my-tool]
action = "block"
message = "Use 'npm run tool' instead"
paths = ["$LOCAL_BIN/my-tool"]

# REQUIRES CONFIRMATION: /usr/local/bin - needs --confirm-system-dir
# (In Docker, we have write access to /usr/local/bin as root built it)
[shims.curl]
action = "warn"
message = "Consider using httpie for better output"
# Will try to resolve from PATH, landing in /usr/bin (forbidden) or /usr/local/bin (confirmation)

# FORBIDDEN: /bin, /usr/bin - never allowed
[shims.cat]
action = "block"
message = "Use bat instead"
# Will try to resolve from PATH, landing in /bin/cat (forbidden)

[shims.ls]
action = "warn"
message = "Use exa for better output"
# Will also be in /bin (forbidden)
EOF

cat > README.md << 'EOF'
# Mixed Permissions Scenario

This scenario demonstrates ribbin's three permission levels:

## Permission Levels

1. **ALLOWED** - No confirmation needed
   - `~/.local/bin/*`
   - `~/go/bin/*`
   - `./node_modules/.bin/*`

2. **REQUIRES CONFIRMATION** - Needs `--confirm-system-dir` flag
   - `/usr/local/bin/*`
   - `/opt/homebrew/bin/*`

3. **FORBIDDEN** - Never allowed (system directories)
   - `/bin/*`
   - `/usr/bin/*`
   - `/sbin/*`

## In this scenario:

- `my-tool` → `~/.local/bin/my-tool` → **ALLOWED**
- `curl` → `/usr/bin/curl` → **FORBIDDEN** (system)
- `cat` → `/bin/cat` → **FORBIDDEN** (system)
- `ls` → `/bin/ls` → **FORBIDDEN** (system)

## Try these commands:

1. Run without confirmation flag:
   ```
   ribbin shim
   ```
   Only `my-tool` should succeed. Others should fail with security errors.

2. Try with confirmation flag (won't help forbidden dirs):
   ```
   ribbin shim --confirm-system-dir
   ```
   Still can't shim /bin/* binaries - they're in forbidden directories.

3. Check what got shimmed:
   ```
   ls -la ~/.local/bin/
   ```

## Key insight:

ribbin protects system binaries by refusing to shim them, even if you have
write permission. This prevents accidental or malicious modification of
critical system tools.
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
echo "  ALLOWED:    my-tool  -> ~/.local/bin/my-tool"
echo "  FORBIDDEN:  cat      -> /bin/cat"
echo "  FORBIDDEN:  ls       -> /bin/ls"
echo "  FORBIDDEN:  curl     -> /usr/bin/curl"
echo ""
echo "Try: ribbin shim"
echo "  - my-tool will succeed (allowed directory)"
echo "  - cat/ls/curl will fail (system directories)"
echo ""
echo "Even --confirm-system-dir won't help with /bin/*"
echo "Those are FORBIDDEN, not just confirmation-required."
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
