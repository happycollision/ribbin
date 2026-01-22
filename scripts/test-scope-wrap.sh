#!/bin/bash
# Automated test for scope wrappers functionality
# Tests that `ribbin wrap` installs wrappers defined in scopes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "  Testing Scope Wrappers"
echo "========================================"
echo ""

# Create temporary test directory
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "Test directory: $TEST_DIR"
echo ""

# Create test project structure
mkdir -p "$TEST_DIR/project"/{frontend,backend,bin}
cd "$TEST_DIR/project"

# Initialize git repo (required for ribbin)
git init -q
git config user.email "test@example.com"
git config user.name "Test User"

# Create local bin for mock binaries INSIDE the test project
# This avoids local dev mode restrictions since binaries are in same repo
LOCAL_BIN="$TEST_DIR/project/bin"

# Create mock binaries
cat > "$LOCAL_BIN/tsc" << 'EOF'
#!/bin/bash
echo "tsc: TypeScript compiler executed"
EOF
chmod +x "$LOCAL_BIN/tsc"

cat > "$LOCAL_BIN/eslint" << 'EOF'
#!/bin/bash
echo "eslint: Linter executed"
EOF
chmod +x "$LOCAL_BIN/eslint"

cat > "$LOCAL_BIN/jest" << 'EOF'
#!/bin/bash
echo "jest: Test runner executed"
EOF
chmod +x "$LOCAL_BIN/jest"

# Add to PATH
export PATH="$LOCAL_BIN:$PATH"

# Create ribbin.jsonc with wrappers ONLY in scopes
cat > ribbin.jsonc << EOF
{
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

git add ribbin.jsonc
git commit -q -m "Initial commit"

echo -e "${YELLOW}Test 1: Running 'ribbin wrap'${NC}"
echo "Expected: Should wrap 3 binaries (tsc, eslint, jest)"
echo ""

# Use the built ribbin binary
RIBBIN="$PROJECT_ROOT/bin/ribbin"
if [ ! -f "$RIBBIN" ]; then
    echo -e "${RED}FAIL: ribbin binary not found at $RIBBIN${NC}"
    echo "Run 'make build' first"
    exit 1
fi

# Run wrap command and capture output
WRAP_OUTPUT=$($RIBBIN wrap 2>&1)
echo "$WRAP_OUTPUT"
echo ""

# Check if 3 binaries were wrapped
if echo "$WRAP_OUTPUT" | grep -q "Summary: 3 wrapped"; then
    echo -e "${GREEN}✓ Test 1 PASSED: 3 binaries wrapped${NC}"
else
    echo -e "${RED}✗ Test 1 FAILED: Expected '3 wrapped' in output${NC}"
    exit 1
fi
echo ""

echo -e "${YELLOW}Test 2: Verifying sidecar files exist${NC}"
echo "Expected: .ribbin-original files should exist for tsc, eslint, jest"
echo ""

for cmd in tsc eslint jest; do
    if [ -f "$LOCAL_BIN/$cmd.ribbin-original" ]; then
        echo -e "${GREEN}✓ Found $cmd.ribbin-original${NC}"
    else
        echo -e "${RED}✗ Missing $cmd.ribbin-original${NC}"
        exit 1
    fi
done
echo ""

echo -e "${YELLOW}Test 3: Verifying symlinks created${NC}"
echo "Expected: Commands should be symlinks to ribbin"
echo ""

for cmd in tsc eslint jest; do
    if [ -L "$LOCAL_BIN/$cmd" ]; then
        TARGET=$(readlink "$LOCAL_BIN/$cmd")
        echo -e "${GREEN}✓ $cmd is symlink -> $TARGET${NC}"
    else
        echo -e "${RED}✗ $cmd is not a symlink${NC}"
        exit 1
    fi
done
echo ""

echo -e "${YELLOW}Test 4: Testing wrapped commands (should block)${NC}"
echo "Expected: Commands should be blocked with custom messages"
echo ""

# Activate ribbin globally for current shell
eval "$($RIBBIN activate --global --quiet 2>/dev/null || true)"

# Test tsc (should be blocked)
TSC_OUTPUT=$($LOCAL_BIN/tsc 2>&1 || true)
if echo "$TSC_OUTPUT" | grep -q "Use 'pnpm nx type-check' instead"; then
    echo -e "${GREEN}✓ tsc is blocked with correct message${NC}"
else
    echo -e "${RED}✗ tsc not blocked correctly${NC}"
    echo "Output: $TSC_OUTPUT"
    exit 1
fi

# Test eslint (should be blocked)
ESLINT_OUTPUT=$($LOCAL_BIN/eslint 2>&1 || true)
if echo "$ESLINT_OUTPUT" | grep -q "Use 'pnpm nx lint' instead"; then
    echo -e "${GREEN}✓ eslint is blocked with correct message${NC}"
else
    echo -e "${RED}✗ eslint not blocked correctly${NC}"
    echo "Output: $ESLINT_OUTPUT"
    exit 1
fi

# Test jest (should be blocked)
JEST_OUTPUT=$($LOCAL_BIN/jest 2>&1 || true)
if echo "$JEST_OUTPUT" | grep -q "Use 'pnpm nx test' instead"; then
    echo -e "${GREEN}✓ jest is blocked with correct message${NC}"
else
    echo -e "${RED}✗ jest not blocked correctly${NC}"
    echo "Output: $JEST_OUTPUT"
    exit 1
fi
echo ""

echo -e "${YELLOW}Test 5: Cleanup with 'ribbin unwrap'${NC}"
echo "Expected: Should unwrap all 3 binaries"
echo ""

UNWRAP_OUTPUT=$($RIBBIN unwrap --all 2>&1)
echo "$UNWRAP_OUTPUT"
echo ""

if echo "$UNWRAP_OUTPUT" | grep -q "3 unwrapped"; then
    echo -e "${GREEN}✓ Test 5 PASSED: 3 binaries unwrapped${NC}"
else
    echo -e "${RED}✗ Test 5 FAILED: Expected '3 unwrapped' in output${NC}"
    exit 1
fi
echo ""

# Verify originals restored
for cmd in tsc eslint jest; do
    if [ ! -L "$LOCAL_BIN/$cmd" ] && [ -x "$LOCAL_BIN/$cmd" ]; then
        echo -e "${GREEN}✓ $cmd restored to original${NC}"
    else
        echo -e "${RED}✗ $cmd not properly restored${NC}"
        exit 1
    fi
done
echo ""

echo "========================================"
echo -e "${GREEN}  ALL TESTS PASSED!${NC}"
echo "========================================"
echo ""
echo "Summary:"
echo "  ✓ Wrappers defined in scopes are installed"
echo "  ✓ Sidecar files created correctly"
echo "  ✓ Symlinks point to ribbin"
echo "  ✓ Wrapped commands execute block actions"
echo "  ✓ Unwrap restores original binaries"
