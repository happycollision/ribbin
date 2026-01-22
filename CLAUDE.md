# Ribbin

Command shimming tool - blocks direct tool calls and redirects to project-specific alternatives.

## Go

This project uses Go (managed via mise).

- `make build` - Build binary to bin/ribbin
- `make install` - Install to GOPATH/bin
- `make test` - Run all tests (in Docker container for safety)
- `make test RUN=TestName` - Run specific tests matching pattern
- `make test-coverage` - Run tests with coverage report
- `make scenario` - Interactive scenario testing (see below)
- `make clean` - Remove build artifacts
- `go build ./cmd/ribbin` - Direct build

## Project Structure

```
cmd/ribbin/         # CLI entry point
internal/cli/       # CLI commands (Cobra)
internal/config/    # Config file parsing (JSONC)
internal/wrap/      # Wrapper logic (installer, runner)
internal/process/   # PID ancestry checking
internal/security/  # Path validation and security checks
internal/testutil/  # Test utilities
testdata/           # Test fixtures
```

## Config Format

Project config uses JSONC (`ribbin.jsonc`) - JSON with comments:

```jsonc
{
  "$schema": "https://github.com/happycollision/ribbin/ribbin.schema.json",
  "wrappers": {
    // Block direct tsc usage
    "tsc": {
      "action": "block",
      "message": "Use 'pnpm run typecheck' instead"
    },
    // Block npm - this project uses pnpm
    "npm": {
      "action": "block",
      "message": "This project uses pnpm. Run 'pnpm install' instead.",
      "paths": ["/usr/local/bin/npm", "/usr/bin/npm"]
    }
  }
}
```

A JSON Schema is available at `ribbin.schema.json` for editor autocompletion and validation.

### User-Local Config Override

Create `ribbin.local.jsonc` to define personal overrides that aren't committed to the repo. When present, this file is loaded **instead of** `ribbin.jsonc`.

To extend the shared config while adding your own rules:

```jsonc
{
  "scopes": {
    "local": {
      "extends": ["./ribbin.jsonc"],
      "wrappers": {
        // Your personal overrides here
      }
    }
  }
}
```

**Recommended**: Add `ribbin.local.jsonc` to your `.gitignore`.

## Local Development Mode

When Ribbin is installed as a dev dependency (e.g., in `node_modules/.bin/`), it automatically enables **Local Development Mode**. In this mode, Ribbin can only wrap binaries within the same git repository.

This protects developers from malicious packages that might try to wrap system binaries.

**Detection**: Ribbin checks if its own executable is inside a git repository by walking up directories looking for `.git`.

**Behavior**:
- If Ribbin is inside a git repo → can only wrap binaries in that repo
- If Ribbin is NOT in a git repo (global install) → normal security rules apply

This works across ecosystems:
- npm/pnpm/yarn: `./node_modules/.bin/ribbin`
- Python venv: `./.venv/bin/ribbin`
- Ruby bundler: `./vendor/bundle/bin/ribbin`

## Interactive Scenario Testing

Test Ribbin in isolated Docker environments without affecting your host system:

```bash
make scenario                           # Show menu to pick a scenario
make scenario SCENARIO=basic            # Run specific scenario directly
```

**Available scenarios:**

| Scenario | Description |
|----------|-------------|
| `basic` | Block and redirect actions with local wrapper commands |
| `extends` | Config inheritance from mixins and external files |
| `find-orphaned` | Test finding orphaned sidecars and unwrap --all --find |
| `local-dev-mode` | Simulates Ribbin in node_modules/.bin - tests repo-only shimming |
| `mixed-permissions` | Demonstrates allowed vs forbidden directory security |
| `recovery` | Test recovery command |
| `scopes` | Directory-based configs (monorepo style) |

Inside the scenario shell, Ribbin is pre-installed and you can test wrap/unwrap/activate commands. Type `exit` to leave.

Scenario files are in `scripts/scenarios/`.

## Releasing

To create a new release:

```bash
make release VERSION=0.1.0-alpha.6
```

This will:
1. Update CHANGELOG.md (move Unreleased content to new version section)
2. Commit the changelog update
3. Create and push the git tag
4. Trigger GitHub Actions to build and publish the release

The release script validates semver format and checks for uncommitted changes before proceeding.

## Project Status

Implementation in progress. See Plan.md for design notes.
