#!/bin/bash
# Description: Scopes - different shim configs for different directories

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up scopes scenario..."
echo ""

# Create local bin directory
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create local tools that we can shim
for tool in my-npm my-yarn my-pnpm my-rm; do
    cat > "$LOCAL_BIN/$tool" << EOF
#!/bin/bash
echo "$tool: executed with args: \$*"
EOF
    chmod +x "$LOCAL_BIN/$tool"
done

# Create scenario directory structure (monorepo-style)
mkdir -p "$SCENARIO_DIR"/{apps/frontend,apps/backend,packages/shared}
cd "$SCENARIO_DIR"

# Initialize git repo
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Create a ribbin.toml with scoped configurations
cat > ribbin.toml << EOF
# Scopes demonstration
# Different directories get different shim rules

# ============================================
# ROOT LEVEL SHIMS
# These apply everywhere unless overridden
# ============================================

[shims.my-npm]
action = "block"
message = "Use 'pnpm' instead of npm"
paths = ["$LOCAL_BIN/my-npm"]

[shims.my-rm]
action = "warn"
message = "Be careful with rm!"
paths = ["$LOCAL_BIN/my-rm"]

# ============================================
# FRONTEND SCOPE
# Extends root + adds yarn block
# ============================================

[scopes.frontend]
path = "apps/frontend"
extends = ["root"]

# Add yarn block specific to frontend
[scopes.frontend.shims.my-yarn]
action = "block"
message = "Use pnpm, not yarn, in frontend"
paths = ["$LOCAL_BIN/my-yarn"]

# Override rm to be stricter in frontend
[scopes.frontend.shims.my-rm]
action = "block"
message = "Use trash-cli in frontend (rm blocked)"
paths = ["$LOCAL_BIN/my-rm"]

# ============================================
# BACKEND SCOPE
# Extends root but allows npm
# ============================================

[scopes.backend]
path = "apps/backend"
extends = ["root"]

# Override: allow npm in backend (for legacy reasons)
[scopes.backend.shims.my-npm]
action = "passthrough"
paths = ["$LOCAL_BIN/my-npm"]

# ============================================
# SHARED PACKAGES SCOPE
# Strictest rules
# ============================================

[scopes.shared]
path = "packages/shared"
extends = ["root"]

# Block everything in shared packages
[scopes.shared.shims.my-yarn]
action = "block"
message = "No yarn in shared packages"
paths = ["$LOCAL_BIN/my-yarn"]

[scopes.shared.shims.my-pnpm]
action = "block"
message = "Run from monorepo root, not package dir"
paths = ["$LOCAL_BIN/my-pnpm"]
EOF

# Create README files in each directory
cat > README.md << 'EOF'
# Scopes Scenario

This scenario demonstrates ribbin's scope-based configuration.

## Directory Structure

```
scenario/
├── ribbin.toml          # Config with scopes
├── apps/
│   ├── frontend/        # scope: frontend
│   └── backend/         # scope: backend
└── packages/
    └── shared/          # scope: shared
```

## Scope Rules

| Command  | Root (default) | Frontend        | Backend     | Shared      |
|----------|----------------|-----------------|-------------|-------------|
| my-npm   | BLOCK          | BLOCK (inherit) | PASSTHROUGH | BLOCK       |
| my-yarn  | (none)         | BLOCK           | (none)      | BLOCK       |
| my-rm    | WARN           | BLOCK (stricter)| WARN        | WARN        |
| my-pnpm  | (none)         | (none)          | (none)      | BLOCK       |

## Try these commands:

1. Install shims and activate:
   ```
   ribbin shim && ribbin on
   ```

2. Test from project root:
   ```
   my-npm install       # BLOCKED (root scope)
   my-rm file.txt       # WARNING (root scope)
   ```

3. Test from frontend:
   ```
   cd apps/frontend
   my-npm install       # BLOCKED (inherited from root)
   my-yarn add pkg      # BLOCKED (frontend-specific)
   my-rm file.txt       # BLOCKED (stricter override)
   ```

4. Test from backend:
   ```
   cd apps/backend
   my-npm install       # ALLOWED (backend overrides to passthrough)
   my-rm file.txt       # WARNING (inherited from root)
   ```

5. Check effective config per directory:
   ```
   ribbin config show              # From root
   cd apps/frontend && ribbin config show
   cd apps/backend && ribbin config show
   ```
EOF

cat > apps/frontend/README.md << 'EOF'
# Frontend App

In this directory:
- my-npm is BLOCKED (inherited from root)
- my-yarn is BLOCKED (frontend-specific rule)
- my-rm is BLOCKED (stricter than root's WARN)
EOF

cat > apps/backend/README.md << 'EOF'
# Backend App

In this directory:
- my-npm is ALLOWED (backend overrides to passthrough)
- my-rm is WARN (inherited from root)
EOF

cat > packages/shared/README.md << 'EOF'
# Shared Packages

In this directory:
- my-npm is BLOCKED (inherited from root)
- my-yarn is BLOCKED (shared-specific)
- my-pnpm is BLOCKED (shared-specific - run from root!)
- my-rm is WARN (inherited from root)
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Scopes Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo ""
echo "Directory structure:"
echo "  ./                    (root scope)"
echo "  ./apps/frontend/      (frontend scope - strict)"
echo "  ./apps/backend/       (backend scope - npm allowed)"
echo "  ./packages/shared/    (shared scope - strictest)"
echo ""
echo "Quick start:"
echo "  1. ribbin shim && ribbin on"
echo "  2. my-npm install        # blocked at root"
echo "  3. cd apps/backend"
echo "  4. my-npm install        # allowed! (backend overrides)"
echo "  5. cd ../frontend"
echo "  6. my-yarn add foo       # blocked (frontend rule)"
echo ""
echo "Use 'ribbin config show' to see effective rules per directory."
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
