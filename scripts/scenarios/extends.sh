#!/bin/bash
# Description: Extends - config inheritance from mixins and external files

set -e

SCENARIO_DIR="$HOME/scenario"
LOCAL_BIN="$HOME/.local/bin"

echo "Setting up extends scenario..."
echo ""

# Create local bin directory
mkdir -p "$LOCAL_BIN"
export PATH="$LOCAL_BIN:$PATH"

# Create local tools that we can wrap
for tool in my-rm my-curl my-wget my-ssh my-docker my-kubectl; do
    cat > "$LOCAL_BIN/$tool" << EOF
#!/bin/bash
echo "$tool: executed with args: \$*"
EOF
    chmod +x "$LOCAL_BIN/$tool"
done

# Create scenario directory structure
mkdir -p "$SCENARIO_DIR"/{team-configs,projects/web-app,projects/data-pipeline}
cd "$SCENARIO_DIR"

# Initialize git repo
git init -q
git config user.email "tester@example.com"
git config user.name "Tester"

# Create a shared team config file (external)
cat > team-configs/security-baseline.jsonc << EOF
{
  // Team security baseline - shared across projects
  // Other configs can extend this file
  "wrappers": {
    "my-rm": {
      "action": "block",
      "message": "Use trash-cli for safety",
      "paths": ["$LOCAL_BIN/my-rm"]
    },
    "my-curl": {
      "action": "warn",
      "message": "Prefer httpie for better security headers",
      "paths": ["$LOCAL_BIN/my-curl"]
    },
    "my-wget": {
      "action": "warn",
      "message": "Use curl or httpie instead",
      "paths": ["$LOCAL_BIN/my-wget"]
    }
  }
}
EOF

# Create another shared config for production rules
cat > team-configs/production-hardened.jsonc << EOF
{
  // Production hardening rules
  // Use in addition to security baseline
  "wrappers": {
    "my-ssh": {
      "action": "warn",
      "message": "Ensure you're using the correct SSH key for production",
      "paths": ["$LOCAL_BIN/my-ssh"]
    },
    "my-docker": {
      "action": "warn",
      "message": "Double-check you're targeting the right registry",
      "paths": ["$LOCAL_BIN/my-docker"]
    },
    "my-kubectl": {
      "action": "warn",
      "message": "Verify KUBECONFIG is set to the correct cluster",
      "paths": ["$LOCAL_BIN/my-kubectl"]
    }
  }
}
EOF

# Create main ribbin.jsonc that uses extends
cat > ribbin.jsonc << EOF
{
  // Main project config
  // Demonstrates extends from:
  // 1. External files (team shared configs)
  // 2. Internal mixins (defined in same file)
  // 3. The special "root" keyword
  "scopes": {
    // MIXIN: development (no path, for extending)
    "development": {
      // No path = this is a mixin, not a directory scope
      "wrappers": {
        "my-rm": {
          "action": "warn",  // Softer than baseline's "block"
          "message": "Be careful with rm in development",
          "paths": ["$LOCAL_BIN/my-rm"]
        }
      }
    },

    // MIXIN: ci-mode (no path, for extending)
    "ci-mode": {
      "wrappers": {
        "my-curl": {
          "action": "passthrough",  // Allow curl in CI
          "paths": ["$LOCAL_BIN/my-curl"]
        },
        "my-wget": {
          "action": "passthrough",  // Allow wget in CI
          "paths": ["$LOCAL_BIN/my-wget"]
        }
      }
    },

    // WEB-APP SCOPE
    // Extends: external security baseline + dev mixin
    "web-app": {
      "path": "projects/web-app",
      "extends": [
        "./team-configs/security-baseline.jsonc",  // External file
        "root.development"                         // Internal mixin
      ]
    },

    // DATA-PIPELINE SCOPE
    // Extends: security baseline + production hardening
    "data-pipeline": {
      "path": "projects/data-pipeline",
      "extends": [
        "./team-configs/security-baseline.jsonc",
        "./team-configs/production-hardened.jsonc"
      ],
      // Override: block rm entirely in data pipeline (data safety)
      "wrappers": {
        "my-rm": {
          "action": "block",
          "message": "NEVER use rm in data pipeline - use versioned storage",
          "paths": ["$LOCAL_BIN/my-rm"]
        }
      }
    }
  }
}
EOF

# Create README files
cat > README.md << 'EOF'
# Extends Scenario

This scenario demonstrates ribbin's `extends` feature for config inheritance.

## Extends Sources

1. **External files** - Share configs across projects
   ```jsonc
   "extends": ["./team-configs/security-baseline.jsonc"]
   ```

2. **Internal mixins** - Scopes without `path` (can't be entered, only extended)
   ```jsonc
   "scopes": {
     "development": {
       // No path = mixin
     },
     "web-app": {
       "extends": ["root.development"]  // Reference internal mixin
     }
   }
   ```

3. **Root keyword** - Inherit from root-level wrappers
   ```jsonc
   "extends": ["root"]
   ```

## Config Structure

```
scenario/
├── ribbin.jsonc                     # Main config with mixins
├── team-configs/
│   ├── security-baseline.jsonc      # Shared security rules
│   └── production-hardened.jsonc    # Production rules
└── projects/
    ├── web-app/                     # Extends: baseline + development
    └── data-pipeline/               # Extends: baseline + production
```

## Inheritance Example

**data-pipeline** extends:
1. `security-baseline.jsonc` → my-rm=block, my-curl=warn, my-wget=warn
2. `production-hardened.jsonc` → my-ssh=warn, my-docker=warn, my-kubectl=warn
3. Local override → my-rm=block (with custom message)

**web-app** extends:
1. `security-baseline.jsonc` → my-rm=block, my-curl=warn
2. `development` mixin → my-rm=warn (overrides baseline's block!)

## Try these commands:

1. Install wrappers and activate:
   ```
   ribbin wrap && ribbin activate --global
   ```

2. Check effective config at root:
   ```
   ribbin config show
   ```

3. Check web-app (development mixin softens rm):
   ```
   cd projects/web-app
   ribbin config show
   my-rm test.txt      # WARN (not block - development override)
   ```

4. Check data-pipeline (production hardened):
   ```
   cd projects/data-pipeline
   ribbin config show
   my-rm test.txt      # BLOCK
   my-kubectl get pods # WARN (production hardening)
   my-docker build .   # WARN (production hardening)
   ```
EOF

cat > projects/web-app/README.md << 'EOF'
# Web App

Extends: security-baseline + development mixin

- my-rm: WARN (development overrides baseline's BLOCK)
- my-curl: WARN (from baseline)
EOF

cat > projects/data-pipeline/README.md << 'EOF'
# Data Pipeline

Extends: security-baseline + production-hardened + local override

- my-rm: BLOCK (local override, even stricter)
- my-curl: WARN (from baseline)
- my-ssh: WARN (from production-hardened)
- my-docker: WARN (from production-hardened)
- my-kubectl: WARN (from production-hardened)
EOF

# Initial commit
git add .
git commit -q -m "Initial commit"

echo "========================================"
echo "  Extends Scenario Ready!"
echo "========================================"
echo ""
echo "Working directory: $SCENARIO_DIR"
echo ""
echo "Config inheritance:"
echo ""
echo "  team-configs/security-baseline.jsonc"
echo "    └─ my-rm=block, my-curl=warn, my-wget=warn"
echo ""
echo "  team-configs/production-hardened.jsonc"
echo "    └─ my-ssh=warn, my-docker=warn, my-kubectl=warn"
echo ""
echo "  ribbin.jsonc mixins:"
echo "    └─ development: my-rm=warn (softer)"
echo "    └─ ci-mode: curl/wget=passthrough"
echo ""
echo "  projects/web-app extends baseline + development"
echo "  projects/data-pipeline extends baseline + production"
echo ""
echo "Quick start:"
echo "  1. ribbin wrap && ribbin activate --global"
echo "  2. cd projects/web-app && ribbin config show"
echo "  3. cd ../data-pipeline && ribbin config show"
echo ""
echo "Type 'exit' to leave the scenario."
echo "========================================"
echo ""

# Export for the shell
export SCENARIO_DIR
export LOCAL_BIN
