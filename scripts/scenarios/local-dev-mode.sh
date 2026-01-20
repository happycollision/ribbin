#!/bin/bash
# Description: Test Local Development Mode (ribbin in node_modules/.bin)

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$SCENARIO_DIR/node_modules/.bin"

echo "Setting up local-dev-mode scenario..."
echo ""

# Create scenario directory structure (simulating a node project)
mkdir -p "$SCENARIO_DIR"/{src,scripts}
mkdir -p "$LOCAL_BIN"
cd "$SCENARIO_DIR"

# Initialize git repo FIRST (ribbin needs to be inside it)
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Copy ribbin into node_modules/.bin (simulating npm install)
# This makes ribbin think it's a dev dependency
cp /usr/local/bin/ribbin "$LOCAL_BIN/ribbin"
chmod +x "$LOCAL_BIN/ribbin"

# Add local bin to PATH (at the front so our ribbin is found first)
export PATH="$LOCAL_BIN:$PATH"

# Create some local scripts that CAN be wrapped (inside the repo)
cat > "$LOCAL_BIN/my-lint" << 'EOF'
#!/bin/bash
echo "Running linter..."
echo "All files OK!"
EOF
chmod +x "$LOCAL_BIN/my-lint"

cat > "$LOCAL_BIN/my-build" << 'EOF'
#!/bin/bash
echo "Building project..."
echo "Build complete!"
EOF
chmod +x "$LOCAL_BIN/my-build"

# Create a ribbin.toml
cat > ribbin.toml << EOF
# Local Development Mode demo
# ribbin is installed in node_modules/.bin, so it can only wrap
# binaries within this repository.

# This WILL work - my-lint is inside the repo
[wrappers.my-lint]
action = "redirect"
redirect = "./scripts/lint-wrapper.sh"
message = "Using project lint wrapper"
paths = ["$LOCAL_BIN/my-lint"]

# This WILL work - my-build is inside the repo
[wrappers.my-build]
action = "block"
message = "Use 'npm run build' instead"
paths = ["$LOCAL_BIN/my-build"]

# This will be REFUSED - cat is outside the repo (system binary)
[wrappers.cat]
action = "block"
message = "Use bat instead"
# No paths specified - will try to resolve from PATH (system /bin/cat)
EOF

# Create the redirect script
cat > scripts/lint-wrapper.sh << 'EOF'
#!/bin/bash
echo "=== Project Lint Wrapper ==="
echo "Running pre-lint checks..."
exec "$RIBBIN_ORIGINAL_BIN" "$@"
EOF
chmod +x scripts/lint-wrapper.sh

# Create package.json to make it look like a real node project
cat > package.json << 'EOF'
{
  "name": "local-dev-test",
  "version": "1.0.0",
  "scripts": {
    "build": "my-build",
    "lint": "my-lint"
  },
  "devDependencies": {
    "ribbin": "file:./node_modules/.bin/ribbin"
  }
}
EOF

cat > README.md << 'EOF'
# Local Development Mode Test

This scenario demonstrates ribbin's Local Development Mode.

## What is Local Development Mode?

When ribbin is installed as a dev dependency (e.g., in `node_modules/.bin`),
it automatically detects that it's inside a git repository and restricts
itself to only wrap binaries within that same repository.

This protects developers from malicious packages that might try to wrap
system binaries like `cat`, `curl`, `ssh`, etc.

## In this scenario:

- ribbin is installed at `./node_modules/.bin/ribbin`
- It can wrap `./node_modules/.bin/my-lint` (inside repo)
- It can wrap `./node_modules/.bin/my-build` (inside repo)
- It CANNOT wrap `/bin/cat` (outside repo - system binary)

## Try these commands:

1. Check ribbin location and mode:
   ```
   which ribbin
   ribbin config show
   ```

2. Try to wrap everything:
   ```
   ribbin wrap
   ```
   Notice that `cat` is refused because it's outside the repository.

3. Activate and test:
   ```
   ribbin activate --global
   my-lint          # Should redirect through wrapper
   my-build         # Should be blocked
   ```

## Why this matters:

If you `npm install malicious-package` and it includes a ribbin config
that tries to wrap `ssh` or `curl`, Local Development Mode prevents it.
The malicious package's ribbin can only affect binaries within its own
node_modules, not your system.
EOF

# Initial commit
git add .
git commit -q -m "Initial commit with local ribbin"

echo "========================================"
echo "  Local Development Mode Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo "Ribbin location:   $LOCAL_BIN/ribbin (inside repo)"
echo ""
echo "This simulates ribbin installed via npm/pnpm/yarn."
echo "Ribbin will only wrap binaries INSIDE this repository."
echo ""
echo "Local binaries (CAN be wrapped):"
echo "  my-lint   - will be redirected"
echo "  my-build  - will be blocked"
echo ""
echo "System binaries (CANNOT be wrapped):"
echo "  cat, curl, etc. - refused by Local Development Mode"
echo ""
echo "Quick start:"
echo "  1. ribbin wrap               # Watch 'cat' get refused"
echo "  2. ribbin activate --global  # Activate globally"
echo "  3. my-build                  # Blocked!"
echo "  4. my-lint                   # Redirected through wrapper"
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
