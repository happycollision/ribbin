# Performance

Understanding Ribbin's overhead and when it matters.

## Summary

| Platform | Overhead | Impact |
|----------|----------|--------|
| Linux | ~1.0ms | Imperceptible |
| macOS | ~5.4ms | Imperceptible |

For interactive use, you won't notice it.

## Overhead at Scale

| Invocations | Linux | macOS |
|-------------|-------|-------|
| 100 | 0.10s | 0.54s |
| 1,000 | 1.0s | 5.4s |
| 10,000 | 10s | 54s |

## Where the Overhead Comes From

When Ribbin intercepts a command, it performs these steps:

1. **Load registry** (~0.1ms) - Read `~/.config/ribbin/registry.json`
2. **Check activation** (~0.05ms) - Is Ribbin active globally/shell/config?
3. **Find config** (~0.3ms) - Walk directories for `ribbin.jsonc`
4. **Parse config** (~0.2ms) - Parse JSONC file
5. **Look up command** (~0.05ms) - Find wrapper in config
6. **Audit logging** (~0.04ms) - Write to audit log
7. **Execute** (~0.2ms) - `syscall.Exec` the original or action

Total: ~1ms on Linux

## Why macOS is Slower

macOS adds overhead that Linux doesn't have:

**Process spawning:**
- macOS fork/exec is 2-3x slower than Linux
- APFS has higher latency than ext4 for small I/O

**Security checks:**
- Code signing verification on each exec
- System Integrity Protection (SIP) validation
- Gatekeeper checks

This is inherent to macOS, not specific to Ribbin.

## Relative vs Absolute Overhead

The overhead is constant per invocation. For longer commands, it becomes negligible:

**Fast command (cat):**
- Without wrapper: 0.97ms
- With wrapper: 1.96ms
- Overhead: +103%

**Slow command (grep on large file):**
- Without wrapper: 8.84ms
- With wrapper: 9.94ms
- Overhead: +12%

As commands take longer, relative overhead decreases.

## When to Use RIBBIN_BYPASS

Use `RIBBIN_BYPASS=1` for:

**Tight loops:**
```bash
# Without bypass - 10,000 iterations adds 10 seconds
for i in $(seq 10000); do
    cat small-file.txt > /dev/null
done

# With bypass - no overhead
for i in $(seq 10000); do
    RIBBIN_BYPASS=1 cat small-file.txt > /dev/null
done
```

**Performance-critical scripts:**
```bash
#!/bin/bash
# Build script that runs cat thousands of times
RIBBIN_BYPASS=1 make build
```

**Known-safe contexts:**
```json
{
  "scripts": {
    "build": "RIBBIN_BYPASS=1 webpack"
  }
}
```

## When NOT to Worry

For typical development workflows:

- Running `tsc` once: 1ms overhead on a multi-second compile
- Running `eslint`: 1ms overhead on a multi-second lint
- Running `npm install`: 1ms overhead on a multi-minute install

The overhead is noise compared to the actual work.

## Measuring Overhead

Run the benchmarks yourself:

```bash
make benchmark          # Fast command (cat), 10k iterations
make benchmark-grep     # Slow command (grep), 1k iterations
make benchmark-all      # Both
```

## Detailed Benchmark Results

### Linux (Docker, golang:1.23-alpine, ARM64)

**cat (fast command), 10,000 iterations:**
```
Without wrapper: 9.7s (0.97ms/call)
With wrapper:    19.6s (1.96ms/call)
Overhead:        +1.0ms/call (+103%)
```

**grep (slow command), 1,000 iterations:**
```
Without wrapper: 8.84s (8.84ms/call)
With wrapper:    9.94s (9.94ms/call)
Overhead:        +1.1ms/call (+12%)
```

### macOS (Apple M1 Pro)

**cat (fast command), 10,000 iterations:**
```
Without wrapper: 61.2s (6.12ms/call)
With wrapper:    115.5s (11.55ms/call)
Overhead:        +5.4ms/call (+89%)
```

**grep (slow command), 1,000 iterations:**
```
Without wrapper: 10.58s (10.58ms/call)
With wrapper:    15.47s (15.47ms/call)
Overhead:        +4.9ms/call (+46%)
```

## See Also

- [How Ribbin Works](how-ribbin-works.md) - Architecture
- [Environment Variables](../reference/environment-vars.md) - RIBBIN_BYPASS
