# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Shimmed commands now work when invoked by name (e.g., `npm`) instead of requiring the full path. Previously, the shim looked for the `.ribbin-original` file in the current working directory instead of the directory containing the symlink.

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
