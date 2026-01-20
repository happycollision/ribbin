#!/bin/bash
# Description: Test recovery command

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up recovery scenario..."
echo ""

# Create local bin directory and add to PATH (at the front!)
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create test binaries that we'll wrap and then recover
cat > "$LOCAL_BIN/tool-alpha" << 'EOF'
#!/bin/bash
echo "I am tool-alpha (original)"
EOF
chmod +x "$LOCAL_BIN/tool-alpha"

cat > "$LOCAL_BIN/tool-beta" << 'EOF'
#!/bin/bash
echo "I am tool-beta (original)"
EOF
chmod +x "$LOCAL_BIN/tool-beta"

# Create scenario directory structure
mkdir -p "$SCENARIO_DIR"
cd "$SCENARIO_DIR"

# Initialize git repo (ribbin uses git root for config discovery)
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Create ribbin config
cat > ribbin.jsonc << EOF
{
  "wrappers": {
    "tool-alpha": {
      "action": "block",
      "message": "Tool alpha is blocked for testing",
      "paths": ["$LOCAL_BIN/tool-alpha"]
    },
    "tool-beta": {
      "action": "block",
      "message": "Tool beta is blocked for testing",
      "paths": ["$LOCAL_BIN/tool-beta"]
    }
  }
}
EOF

cat > README.md << 'EOF'
# Recovery Scenario

Test the `ribbin recover` command.

## Quick Test

```bash
# 1. Verify tools work
tool-alpha
tool-beta

# 2. Wrap and activate
ribbin wrap
ribbin activate --global

# 3. Verify they're blocked
tool-alpha  # Should be blocked

# 4. Recover
ribbin recover

# 5. Verify restoration
tool-alpha  # Should work again
```

## Manual Recovery

If ribbin weren't installed, you'd do this manually:

```bash
# Find wrapped binaries
ls ~/.local/bin/*.ribbin-original

# Restore each one
rm ~/.local/bin/tool-alpha
mv ~/.local/bin/tool-alpha.ribbin-original ~/.local/bin/tool-alpha
rm -f ~/.local/bin/tool-alpha.ribbin-meta
```
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Recovery Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo "Wrapper directory: $LOCAL_BIN"
echo ""
echo "Test tools:"
echo "  tool-alpha   - first test tool"
echo "  tool-beta    - second test tool"
echo ""
echo "Quick start:"
echo "  1. tool-alpha               # Works normally"
echo "  2. ribbin wrap              # Install wrappers"
echo "  3. ribbin activate --global # Activate"
echo "  4. tool-alpha               # Now blocked!"
echo "  5. ribbin recover           # Restore all tools"
echo "  6. tool-alpha               # Works again!"
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
