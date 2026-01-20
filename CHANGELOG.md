# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-alpha.6] - 2026-01-20

### Breaking Changes
- **Config format migrated to JSONC**: Config files are now `ribbin.jsonc` instead of `ribbin.toml`. JSONC supports comments and is more familiar to web developers. A JSON Schema is provided for editor autocompletion.
- **Command rename**: `shim`/`unshim` → `wrap`/`unwrap`
- **Config sections renamed**: `[shims.*]` → `"wrappers": {...}`
- **Activation commands overhauled**: `on`/`off` replaced with `activate`/`deactivate` with explicit scope flags

### Added
- **Three-tier activation model**: Control wrapper scope precisely
  - `ribbin activate` (default) - activate specific config file
  - `ribbin activate --shell` - activate for current shell session
  - `ribbin activate --global` - activate system-wide
- **Recovery command**: `ribbin recover` searches for and restores all wrapped binaries (alias for `unwrap --global --search`)
- **Metadata sidecars**: `.ribbin-meta` files track original binary hash for stale detection and conflict resolution during unwrap
- **Status command**: `ribbin status` shows current activation state
- **Scaffold command**: `ribbin scaffold` generates example configs
- New recovery scenario for interactive testing

### Changed
- Improved conflict detection during unwrap when binaries have been reinstalled
- Better error messages with links to documentation
- Global mode warning banner when wrappers are firing everywhere

### Internal
- Registry structure updated for three-tier activation: `wrappers`, `shell_activations`, `config_activations`, `global_active`
- Package rename: `internal/shim` → `internal/wrap`
- Config parsing switched from BurntSushi/toml to tailscale/hujson for JSONC support
- `make release VERSION=x.y.z` command to automate releases

## [0.1.0-alpha.5] - 2026-01-20

### Added
- **Local Development Mode**: When ribbin is installed as a dev dependency (e.g., in `node_modules/.bin/`), it automatically restricts shimming to binaries within the same git repository. This protects against malicious packages attempting to shim system binaries.
- **Interactive scenario testing**: `make scenario` launches isolated Docker environments for testing ribbin configurations without affecting the host system. Available scenarios: `basic`, `local-dev-mode`, `mixed-permissions`, `scopes`, `extends`.
- `--verbose` flag for shim execution to debug shim behavior
- `--confirm-system-dir` flag to explicitly allow shimming in system directories when needed

### Fixed
- Fixed `--confirm-system-dir` flag not properly unlocking system directories

## [0.1.0-alpha.4] - 2026-01-19

### Added
- **Scopes**: Directory-based configuration with `[scopes]` section for monorepo-style setups
- **Config inheritance**: `extends` field to inherit from mixin files or external configurations
- **Passthrough action**: Conditional shim bypass with `action = "passthrough"` for specific contexts
- `ribbin config show` command with provenance tracking to debug config resolution
- **Comprehensive audit logging**: Security events logged to `~/.ribbin/audit.log` in JSON format
- **Symlink attack prevention**: Validates symlink targets to prevent TOCTOU attacks
- **Environment variable validation**: Sanitizes `RIBBIN_*` environment variables
- **P0 security hardening**: Directory allowlist, path sanitization, and file locking
- `redirect` action for custom script execution (run wrapper scripts instead of blocking)
- `ribbin config add`, `ribbin config remove`, `ribbin config list` commands for managing `ribbin.toml`
- Performance benchmarks measuring shim overhead (~0.3ms on Linux, ~1.5ms on macOS)
- Claude Code integration documentation

### Improved
- Enhanced `--help` documentation for all commands with detailed descriptions, examples, and usage information
- Fixed inaccurate help text for `on`/`off` commands (previously said "current shell session" but behavior is global)
- Improved generated `ribbin.toml` with explanatory comments

### Fixed
- Shimmed commands now work when invoked by name (e.g., `npm`) instead of requiring the full path
- Fixed macOS-specific test failures
- Fixed user detection in audit logging for containers

### Internal
- Comprehensive security attack test suite
- Integration tests for scopes, extends, mise, and asdf compatibility
- Test safety check to prevent tests from accidentally running on host

## [0.1.0-alpha.3] - 2026-01-18

### Added
- `--version` / `-V` flag to display version information

## [0.1.0-alpha.2] - 2026-01-18

### Added
- `ribbin init` command to create `ribbin.toml` configuration file
- Comprehensive README documentation

### Fixed
- Error messages now correctly reference `ribbin.toml` instead of `ribbin.json`

## [0.1.0-alpha.1] - 2026-01-18

### Added
- Initial implementation of ribbin CLI
- Commands: `shim`, `unshim`, `on`, `off`, `activate`
- TOML-based project configuration (`ribbin.toml`)
- Process ancestry checking for shell-scoped activation

### Internal
- Docker-based test suite for safe testing
