# Performance

## TL;DR

| Platform | Overhead | Impact |
|----------|----------|--------|
| **Linux** | ~1.0ms | Imperceptible |
| **macOS** | ~5.4ms | Imperceptible |

For interactive use, you won't notice it. For tight loops, use `RIBBIN_BYPASS=1`.

## Overhead at Scale

| Invocations | Linux | macOS |
|-------------|-------|-------|
| 100 | 0.10s | 0.54s |
| 1,000 | 1.0s | 5.4s |
| 10,000 | 10s | 54s |

## Bypass for Performance-Critical Scripts

```bash
RIBBIN_BYPASS=1 cat file.txt  # Direct passthrough, no overhead
```

## What Causes the Overhead

1. Load and parse registry JSON
2. Check activation status (global or PID ancestry)
3. Walk directories to find `ribbin.toml`
4. Parse project config (TOML)
5. Look up command in shim configuration
6. Audit logging (~0.04ms - negligible)
7. Execute original command via `syscall.Exec`

## Why macOS is Slower

- Process spawning: macOS fork/exec is 2-3x slower than Linux
- File system: APFS has higher latency than ext4 for small I/O
- Security checks: Code signing and SIP validation on each exec

## Running Benchmarks

```bash
make benchmark          # Fast command (cat), 10k iterations
make benchmark-grep     # Slower command (grep), 1k iterations
make benchmark-all      # Both
```

## Detailed Results

### Linux (Docker, golang:1.23-alpine on ARM64)

**Fast command (cat):** 1.96ms with shim vs 0.97ms without (+103%)
**Slow command (grep):** 9.94ms with shim vs 8.84ms without (+12%)

### macOS (Apple M1 Pro)

**Fast command (cat):** 11.55ms with shim vs 6.12ms without (+89%)
**Slow command (grep):** 15.47ms with shim vs 10.58ms without (+46%)

The absolute overhead is constant per platform. Relative overhead decreases as commands take longer.
