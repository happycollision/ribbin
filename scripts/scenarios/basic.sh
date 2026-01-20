#!/bin/bash
# Description: Basic shim testing with block and redirect actions

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up basic scenario..."
echo ""

# Create local bin directory and add to PATH (at the front!)
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create local wrapper scripts that call the real binaries
# These are what ribbin will shim (it can't shim system binaries for security)

# Create a fake npm for testing (simulates npm being blocked in favor of pnpm)
cat > "$LOCAL_BIN/mynpm" << 'EOF'
#!/bin/bash
echo "npm (fake for testing)"
echo "Version 10.0.0"
echo "Running: npm $*"
EOF
chmod +x "$LOCAL_BIN/mynpm"

cat > "$LOCAL_BIN/mycurl" << 'EOF'
#!/bin/bash
exec /usr/bin/curl "$@"
EOF
chmod +x "$LOCAL_BIN/mycurl"

cat > "$LOCAL_BIN/myecho" << 'EOF'
#!/bin/bash
exec /bin/echo "$@"
EOF
chmod +x "$LOCAL_BIN/myecho"

# Create a fake tsc for testing
cat > "$LOCAL_BIN/tsc" << 'EOF'
#!/bin/bash
echo "TypeScript Compiler (fake for testing)"
echo "Version 5.0.0"
EOF
chmod +x "$LOCAL_BIN/tsc"

# Create scenario directory structure
mkdir -p "$SCENARIO_DIR"/{src,scripts}
cd "$SCENARIO_DIR"

# Initialize git repo (ribbin uses git root for config discovery)
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Create a ribbin.toml with various shim examples
# Using explicit paths to our local wrappers
cat > ribbin.toml << EOF
# Example ribbin configuration for testing
# Try running blocked commands to see ribbin in action!

# Block direct npm usage - this project uses pnpm
[shims.mynpm]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' or 'pnpm add <pkg>' instead."
paths = ["$LOCAL_BIN/mynpm"]

# Block direct tsc usage - enforce project script
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' instead of direct tsc"
paths = ["$LOCAL_BIN/tsc"]

# Redirect myecho to a custom script (for demonstration)
[shims.myecho]
action = "redirect"
redirect = "./scripts/fancy-echo.sh"
message = "Using fancy echo wrapper"
paths = ["$LOCAL_BIN/myecho"]

# Block mycurl - use the project's API client
[shims.mycurl]
action = "block"
message = "Use the project's API client at ./scripts/api.sh instead"
paths = ["$LOCAL_BIN/mycurl"]
EOF

# Create the redirect script
cat > scripts/fancy-echo.sh << 'EOF'
#!/bin/bash
# Fancy echo wrapper - adds decoration
echo ">>> $* <<<"
EOF
chmod +x scripts/fancy-echo.sh

# Create some sample files to work with
cat > src/example.ts << 'EOF'
// Example TypeScript file
export function greet(name: string): string {
  return `Hello, ${name}!`;
}

console.log(greet("World"));
EOF

cat > README.md << 'EOF'
# Test Project

This is a sample project for testing ribbin shims.

## Local Wrapper Commands

These are local wrappers in ~/.local/bin that ribbin can safely shim:
- `mynpm` - fake npm (will be blocked, suggests pnpm)
- `tsc` - fake TypeScript compiler (will be blocked)
- `myecho` - wrapper for echo (will be redirected)
- `mycurl` - wrapper for curl (will be blocked)

## Try these steps:

1. First, test the commands work without shims:
   ```
   mynpm install
   tsc --version
   myecho "hello world"
   ```

2. View the config:
   ```
   ribbin config show
   ```

3. Install the shims:
   ```
   ribbin shim
   ```

4. Enable shims globally:
   ```
   ribbin on
   ```

5. Now try the commands again - they should be blocked/redirected:
   ```
   mynpm install        # Should be blocked
   tsc --version        # Should be blocked
   myecho "hello"       # Should redirect to fancy-echo.sh
   ```

6. Remove shims when done:
   ```
   ribbin unshim
   ```
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Basic Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo "Shim directory:    $LOCAL_BIN"
echo ""
echo "Local wrapper commands (in ~/.local/bin):"
echo "  mynpm   - fake npm          (will be blocked, suggests pnpm)"
echo "  tsc     - fake tsc          (will be blocked)"
echo "  myecho  - wrapper for echo  (will be redirected)"
echo "  mycurl  - wrapper for curl  (will be blocked)"
echo ""
echo "Quick start:"
echo "  1. mynpm install           # Works normally"
echo "  2. ribbin shim             # Install shims"
echo "  3. ribbin on               # Enable shims globally"
echo "  4. mynpm install           # Now blocked!"
echo "  5. ribbin unshim           # Restore originals"
echo ""
echo "More commands: ribbin config show, ribbin --help"
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
