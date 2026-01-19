# Benchmark Results

## Shim Overhead Measurement

These benchmarks measure the overhead of having a ribbin shim in place by testing both fast commands (cat) and slower commands (grep) to understand how the overhead scales with command execution time.

### Running the Benchmarks

```bash
# Cat benchmark - fast command (10,000 iterations, ~40 seconds)
make benchmark

# Grep benchmark - slower command (1,000 iterations, ~22 seconds)
make benchmark-grep

# Run all benchmarks
make benchmark-all

# Full cat benchmark (1,000,000 iterations)
make benchmark-full
```

### Latest Results

**Date:** 2026-01-18
**Version:** After audit logging implementation

#### Linux (Docker, golang:1.23-alpine on ARM64)

##### Test 1: Fast Command (cat)
**Iterations:** 10,000
**Test:** Running `cat` on a 10-line text file

| Configuration | Time per operation | Overhead |
|--------------|-------------------|----------|
| **With Shim** | 3,041,908 ns/op (3.04ms) | +115% |
| **Without Shim** | 1,412,829 ns/op (1.41ms) | baseline |

**Absolute Overhead:** ~1.63ms per invocation

##### Test 2: Slower Command (grep)
**Iterations:** 10,000 / 1,000
**Test:** Running `grep -r` to search 100 files with 100 lines each (10,000 lines total)

| Configuration | Time per operation | Overhead |
|--------------|-------------------|----------|
| **With Shim** | 12,568,960 ns/op (12.57ms) | +16% |
| **Without Shim** | 10,847,468 ns/op (10.85ms) | baseline |

**Absolute Overhead:** ~1.72ms per invocation

#### macOS (Apple M1 Pro, Go 1.23)

##### Test 1: Fast Command (cat)
**Iterations:** 100
**Test:** Running `cat` on a 10-line text file

| Configuration | Time per operation | Overhead |
|--------------|-------------------|----------|
| **With Shim** | 10,165,978 ns/op (10.17ms) | +71% |
| **Without Shim** | 5,956,256 ns/op (5.96ms) | baseline |

**Absolute Overhead:** ~4.21ms per invocation

##### Test 2: Slower Command (grep)
**Iterations:** 100
**Test:** Running `grep -r` to search 100 files with 100 lines each (10,000 lines total)

| Configuration | Time per operation | Overhead |
|--------------|-------------------|----------|
| **With Shim** | 14,340,174 ns/op (14.34ms) | +42% |
| **Without Shim** | 10,125,991 ns/op (10.13ms) | baseline |

**Absolute Overhead:** ~4.21ms per invocation

### Interpretation

The shim overhead varies significantly by platform but is consistent within each environment:

| Platform | Overhead | Fast Command | Slow Command |
|----------|----------|--------------|--------------|
| **Linux (Docker)** | ~1.6-1.7ms | +115% (cat) | +16% (grep) |
| **macOS (M1)** | ~4.2ms | +71% (cat) | +42% (grep) |

This overhead includes:

1. Loading and parsing the registry JSON file
2. Checking if ribbin is active (global or PID ancestry)
3. Walking up directories to find `ribbin.toml`
4. Loading and parsing the project config (TOML)
5. Looking up the command in the shim configuration
6. **Audit logging** (writing security events to log file)
7. Executing the original command via `syscall.Exec`

#### Key Insights

- **Linux is 2.5x faster:** 1.6ms overhead vs 4.2ms on macOS
- **Absolute overhead is constant per platform:** Regardless of command duration
- **Relative overhead decreases with command duration:**
  - Fast commands (cat): +71-115% overhead
  - Slower commands (grep): +16-42% overhead
- **The overhead becomes less noticeable as commands take longer to execute**
- **Audit logging adds:** ~38 µs (0.038ms) per operation - negligible compared to total overhead

#### Platform Performance Comparison

**Why macOS is slower:**
- Process spawning overhead: macOS fork/exec is ~2-3x slower than Linux
- File system: APFS has higher latency than ext4 for small I/O operations
- Security checks: macOS performs code signing and SIP validation on each exec
- System call overhead: macOS system calls have higher base latency

**Audit logging overhead:**
- Measured at **38 µs** (0.038ms) on both platforms
- Represents **2.3%** of overhead on Linux (38µs / 1.6ms)
- Represents **0.9%** of overhead on macOS (38µs / 4.2ms)
- **Essentially free** compared to other operations

### Practical Impact

For interactive command-line usage, the overhead is imperceptible to humans on both platforms:
- **Linux:** 1.6ms per command
- **macOS:** 4.2ms per command

However, this overhead becomes noticeable in tight loops or scripts that invoke shimmed commands thousands of times:

| Invocations | Linux Overhead | macOS Overhead |
|-------------|----------------|----------------|
| 100 | 0.16s | 0.42s |
| 1,000 | 1.6s | 4.2s |
| 10,000 | 16s | 42s |
| 100,000 | 2.7 min | 7 min |

**Bypass mechanism:** For performance-critical scripts, use `RIBBIN_BYPASS=1` to skip the shim logic entirely:

```bash
RIBBIN_BYPASS=1 cat file.txt  # Direct passthrough, no overhead
```

### Note on Benchmark Methodology

The benchmark uses Go's `testing.B` framework with `exec.Command` to spawn processes, which more accurately reflects real-world usage than in-process function calls. Each iteration:

1. Spawns a new process for the shimmed/non-shimmed command
2. Passes the test file as an argument
3. Captures the output
4. Waits for process completion

This measures the complete end-to-end overhead including process spawning, which is the actual cost users will experience.
