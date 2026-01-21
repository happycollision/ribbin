#!/bin/bash
# Description: Test finding orphaned sidecars and unwrap --search

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up find-orphaned scenario..."
echo ""

# Create local bin directory and add to PATH (at the front!)
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create test binaries that we'll wrap
cat > "$LOCAL_BIN/tool-one" << 'EOF'
#!/bin/bash
echo "I am tool-one (original)"
EOF
chmod +x "$LOCAL_BIN/tool-one"

cat > "$LOCAL_BIN/tool-two" << 'EOF'
#!/bin/bash
echo "I am tool-two (original)"
EOF
chmod +x "$LOCAL_BIN/tool-two"

cat > "$LOCAL_BIN/tool-three" << 'EOF'
#!/bin/bash
echo "I am tool-three (original)"
EOF
chmod +x "$LOCAL_BIN/tool-three"

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
    "tool-one": {
      "action": "block",
      "message": "Tool one is blocked",
      "paths": ["$LOCAL_BIN/tool-one"]
    },
    "tool-two": {
      "action": "block",
      "message": "Tool two is blocked",
      "paths": ["$LOCAL_BIN/tool-two"]
    },
    "tool-three": {
      "action": "block",
      "message": "Tool three is blocked",
      "paths": ["$LOCAL_BIN/tool-three"]
    }
  }
}
EOF

cat > README.md << 'EOF'
# Find Orphaned Sidecars Scenario

Test the `ribbin find` command and `ribbin unwrap --search` functionality.

## Test Steps

### 1. Setup: Wrap all tools
```bash
ribbin wrap
ls -la ~/.local/bin/*.ribbin-original
# Should see: tool-one.ribbin-original, tool-two.ribbin-original, tool-three.ribbin-original
```

### 2. Find command should show all known wrappers
```bash
ribbin find
# Should show all three tools as "Known Wrapped Binaries"
```

### 3. Create orphaned sidecar by manually deleting registry entry
```bash
# Manually corrupt the registry to simulate an orphaned sidecar
# (In real scenarios, this happens from interrupted operations)
# Edit ~/.config/ribbin/registry.json and remove the "tool-two" entry

# Or simulate it by creating a new wrapped binary that's not in registry:
cp ~/.local/bin/tool-one.ribbin-original ~/.local/bin/orphan-tool.ribbin-original
cp ~/.local/bin/tool-one.ribbin-meta ~/.local/bin/orphan-tool.ribbin-meta
cat > ~/.local/bin/orphan-tool << 'EOF2'
#!/bin/bash
echo "Symlink wrapper (won't work but demonstrates orphaning)"
EOF2
chmod +x ~/.local/bin/orphan-tool
```

### 4. Find command should now show orphaned wrapper
```bash
ribbin find ~/.local/bin
# Should show "orphan-tool" as an Unknown/Orphaned Wrapped Binary
```

### 5. Test unwrap --all --find to remove orphaned wrappers
```bash
ribbin unwrap --all --find
# Should find and offer to restore the orphaned sidecar
```

### 6. Verify cleanup
```bash
ribbin find ~/.local/bin
# Orphaned entries should be gone or restored
```

## Testing Different Search Scopes

```bash
# Search current directory only
ribbin find

# Search specific directory
ribbin find ~/.local/bin

# Search entire system (slow, for demonstration)
# ribbin find --all
```

## Manual Orphan Cleanup

Without ribbin:
```bash
# Find all sidecars
find ~/.local/bin -name "*.ribbin-original"

# Restore each manually
for sidecar in ~/.local/bin/*.ribbin-original; do
  original="${sidecar%.ribbin-original}"
  rm -f "$original"
  mv "$sidecar" "$original"
  rm -f "${original}.ribbin-meta"
done
```
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Find Orphaned Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo "Wrapper directory: $LOCAL_BIN"
echo ""
echo "Test tools:"
echo "  tool-one     - first test tool"
echo "  tool-two     - second test tool"
echo "  tool-three   - third test tool"
echo ""
echo "Quick test sequence:"
echo "  1. ribbin wrap                     # Wrap all tools"
echo "  2. ribbin find                     # Show known wrappers"
echo "  3. [Manually create orphan]        # See README.md"
echo "  4. ribbin find ~/.local/bin        # Show orphaned wrapper"
echo "  5. ribbin unwrap --all --find     # Clean up orphans"
echo ""
echo "See README.md for detailed test steps."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
