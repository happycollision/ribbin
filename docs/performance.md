# Performance

## TL;DR

| Platform | Overhead | Impact |
|----------|----------|--------|
| **Linux** | ~1.6ms | Imperceptible |
| **macOS** | ~4.2ms | Imperceptible |

For interactive use, you won't notice it. For tight loops, use `RIBBIN_BYPASS=1`.

## Overhead at Scale

| Invocations | Linux | macOS |
|-------------|-------|-------|
| 100 | 0.16s | 0.42s |
| 1,000 | 1.6s | 4.2s |
| 10,000 | 16s | 42s |

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

**Fast command (cat):** 3.04ms with shim vs 1.41ms without (+115%)
**Slow command (grep):** 12.57ms with shim vs 10.85ms without (+16%)

### macOS (Apple M1 Pro)

**Fast command (cat):** 10.17ms with shim vs 5.96ms without (+71%)
**Slow command (grep):** 14.34ms with shim vs 10.13ms without (+42%)

The absolute overhead is constant per platform. Relative overhead decreases as commands take longer.
