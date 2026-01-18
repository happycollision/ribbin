# ribbin

Command shimming tool - blocks direct tool calls and redirects to project-specific alternatives.

## Go

This project uses Go (managed via mise).

- `make build` - Build binary to bin/ribbin
- `make install` - Install to GOPATH/bin
- `make test` - Run tests (in Docker)
- `make clean` - Remove build artifacts
- `go build ./cmd/ribbin` - Direct build

## Project Structure

```
cmd/ribbin/         # CLI entry point
internal/cli/       # CLI commands (Cobra)
internal/config/    # Config file parsing (TOML)
internal/shim/      # Shim logic
internal/process/   # PID ancestry checking
```

## Config Format

Project config uses TOML (`ribbin.toml`):

```toml
# Block direct tsc usage
[shims.tsc]
action = "block"
message = "Use 'pnpm run typecheck' instead"

# Block cat, suggest bat
[shims.cat]
action = "block"
message = "Use 'bat' for syntax highlighting"
paths = ["/usr/bin/cat", "/bin/cat"]
```

## Project Status

Implementation in progress. See Plan.md for design notes.
