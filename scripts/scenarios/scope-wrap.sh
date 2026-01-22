#!/bin/bash
# Description: Scope Wrappers - Test that `ribbin wrap` installs wrappers defined in scopes

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up scope-wrap scenario..."
echo ""

# Create local bin directory
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create mock binaries in local bin that we can wrap
cat > "$LOCAL_BIN/tsc" << 'EOF'
#!/bin/bash
echo "tsc: TypeScript compiler executed with args: $*"
EOF
chmod +x "$LOCAL_BIN/tsc"

cat > "$LOCAL_BIN/eslint" << 'EOF'
#!/bin/bash
echo "eslint: Linter executed with args: $*"
EOF
chmod +x "$LOCAL_BIN/eslint"

cat > "$LOCAL_BIN/jest" << 'EOF'
#!/bin/bash
echo "jest: Test runner executed with args: $*"
EOF
chmod +x "$LOCAL_BIN/jest"

# Create scenario directory structure (monorepo with frontend and backend)
mkdir -p "$SCENARIO_DIR"/{frontend,backend}
cd "$SCENARIO_DIR"

# Initialize git repo
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Create ribbin.jsonc with wrappers ONLY in scopes (not at root level)
cat > ribbin.jsonc << EOF
{
  // Test that wrappers defined ONLY in scopes get installed
  // (no root-level wrappers defined)

  "scopes": {
    "frontend": {
      "path": "frontend",
      "wrappers": {
        "tsc": {
          "action": "block",
          "message": "Use 'pnpm nx type-check' instead",
          "paths": ["$LOCAL_BIN/tsc"]
        },
        "eslint": {
          "action": "block",
          "message": "Use 'pnpm nx lint' instead",
          "paths": ["$LOCAL_BIN/eslint"]
        }
      }
    },
    "backend": {
      "path": "backend",
      "wrappers": {
        "jest": {
          "action": "block",
          "message": "Use 'pnpm nx test' instead",
          "paths": ["$LOCAL_BIN/jest"]
        }
      }
    }
  }
}
EOF

# Create README
cat > README.md << 'EOF'
# Scope Wrappers Scenario

This scenario tests that `ribbin wrap` correctly installs wrappers that are
defined ONLY in scopes (not at the root level).

## Configuration

- NO root-level wrappers defined
- Frontend scope wraps: tsc, eslint
- Backend scope wraps: jest

## Test Steps

1. Install wrappers:
   ```
   ribbin wrap
   ```
   Expected: Should wrap tsc, eslint, and jest (3 wrapped)

2. Verify wrappers were installed:
   ```
   ls -la ~/.local/bin/*.ribbin-original
   ```
   Expected: Should see tsc.ribbin-original, eslint.ribbin-original, jest.ribbin-original

3. Test wrapped commands (after activate):
   ```
   ribbin activate --global
   tsc        # Should be blocked
   eslint     # Should be blocked
   jest       # Should be blocked
   ```

4. Clean up:
   ```
   ribbin unwrap --global --all
   ```

## Expected Behavior

Before the fix:
- `ribbin wrap` would show "Summary: 0 wrapped, 0 skipped, 0 failed"
- No wrappers would be installed

After the fix:
- `ribbin wrap` should show "Summary: 3 wrapped, 0 skipped, 0 failed"
- All three commands should be wrapped and blocked when invoked
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Scope Wrappers Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo ""
echo "This scenario tests that wrappers defined ONLY in scopes"
echo "(not at root level) are correctly installed by 'ribbin wrap'."
echo ""
echo "Test procedure:"
echo "  1. Run: ribbin wrap"
echo "     Expected: 3 wrapped (tsc, eslint, jest)"
echo ""
echo "  2. Verify wrappers: ls -la ~/.local/bin/*.ribbin-original"
echo "     Expected: See tsc, eslint, jest sidecars"
echo ""
echo "  3. Activate and test: ribbin activate --global && tsc"
echo "     Expected: tsc should be blocked with message"
echo ""
echo "  4. Clean up: ribbin unwrap --global --all"
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
