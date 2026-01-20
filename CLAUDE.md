# ribbin

Command shimming tool - blocks direct tool calls and redirects to project-specific alternatives.

## Go

This project uses Go (managed via mise).

- `make build` - Build binary to bin/ribbin
- `make install` - Install to GOPATH/bin
- `make test` - Run unit tests (in Docker container for safety)
- `make test-coverage` - Run tests with coverage report
- `make test-integration` - Run integration tests
- `make scenario` - Interactive scenario testing (see below)
- `make clean` - Remove build artifacts
- `go build ./cmd/ribbin` - Direct build

## Project Structure

```
cmd/ribbin/         # CLI entry point
internal/cli/       # CLI commands (Cobra)
internal/config/    # Config file parsing (TOML)
internal/shim/      # Shim logic
internal/process/   # PID ancestry checking
internal/testutil/  # Test utilities
testdata/           # Test fixtures
```

## Config Format

Project config uses TOML (`ribbin.toml`):

```toml
# Block direct tsc usage
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' instead"

# Block npm - this project uses pnpm
[shims.npm]
action = "block"
message = "This project uses pnpm. Run 'pnpm install' instead."
paths = ["/usr/local/bin/npm", "/usr/bin/npm"]
```

## Local Development Mode

When ribbin is installed as a dev dependency (e.g., in `node_modules/.bin/`), it automatically enables **Local Development Mode**. In this mode, ribbin can only shim binaries within the same git repository.

This protects developers from malicious packages that might try to shim system binaries.

**Detection**: ribbin checks if its own executable is inside a git repository by walking up directories looking for `.git`.

**Behavior**:
- If ribbin is inside a git repo → can only shim binaries in that repo
- If ribbin is NOT in a git repo (global install) → normal security rules apply

This works across ecosystems:
- npm/pnpm/yarn: `./node_modules/.bin/ribbin`
- Python venv: `./.venv/bin/ribbin`
- Ruby bundler: `./vendor/bundle/bin/ribbin`

## Interactive Scenario Testing

Test ribbin in isolated Docker environments without affecting your host system:

```bash
make scenario                           # Show menu to pick a scenario
make scenario SCENARIO=basic            # Run specific scenario directly
```

**Available scenarios:**

| Scenario | Description |
|----------|-------------|
| `basic` | Block and redirect actions with local wrapper commands |
| `local-dev-mode` | Simulates ribbin in node_modules/.bin - tests repo-only shimming |
| `mixed-permissions` | Demonstrates allowed vs forbidden directory security |
| `scopes` | Directory-based configs (monorepo style) |
| `extends` | Config inheritance from mixins and external files |

Inside the scenario shell, ribbin is pre-installed and you can test shim/unshim/activate commands. Type `exit` to leave.

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
