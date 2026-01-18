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

**Environment:** Docker container (golang:1.23-alpine)

#### Test 1: Fast Command (cat)
**Iterations:** 10,000
**Test:** Running `cat` on a 10-line text file

| Configuration | Time per operation | Overhead |
|--------------|-------------------|----------|
| **With Shim** | 2,550,830 ns/op (2.55ms) | +117% |
| **Without Shim** | 1,175,774 ns/op (1.18ms) | baseline |

**Absolute Overhead:** ~1.375ms per invocation

#### Test 2: Slower Command (grep)
**Iterations:** 1,000
**Test:** Running `grep -r` to search 100 files with 100 lines each (10,000 lines total)

| Configuration | Time per operation | Overhead |
|--------------|-------------------|----------|
| **With Shim** | 10,249,142 ns/op (10.25ms) | +17% |
| **Without Shim** | 8,731,936 ns/op (8.73ms) | baseline |

**Absolute Overhead:** ~1.517ms per invocation

### Interpretation

The shim adds approximately **1.4-1.5 milliseconds** of overhead per command invocation, regardless of how long the underlying command takes to execute. This overhead includes:

1. Loading and parsing the registry JSON file
2. Checking if ribbin is active (global or PID ancestry)
3. Walking up directories to find `ribbin.toml`
4. Loading and parsing the project config (TOML)
5. Looking up the command in the shim configuration
6. Executing the original command via `syscall.Exec`

#### Key Insights

- **Absolute overhead is constant:** ~1.4-1.5ms regardless of command duration
- **Relative overhead decreases with command duration:**
  - Fast commands (cat): +117% overhead
  - Slower commands (grep): +17% overhead
- **The overhead becomes less noticeable as commands take longer to execute**

### Practical Impact

For interactive command-line usage, 1.4ms is imperceptible to humans. However, this overhead becomes noticeable in tight loops or scripts that invoke shimmed commands thousands of times.

**Example:** A script running a shimmed command 10,000 times would add ~14 seconds of overhead.

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
