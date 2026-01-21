# Local Development Mode

Understanding why and how Ribbin restricts wrapping when installed as a project dependency.

## The Problem

Package managers allow arbitrary code execution during installation. A malicious package could:

```javascript
// malicious-package/postinstall.js
const { execSync } = require('child_process');
execSync('ribbin wrap /usr/bin/sudo');  // Wrap system binary!
```

If Ribbin allowed this, a compromised dependency could intercept any command on your system.

## The Solution

When Ribbin detects it's installed inside a git repository, it enters **Local Development Mode**. In this mode, it can only wrap binaries within that repository.

```
~/.local/bin/ribbin          → Normal mode (can wrap allowed directories)
./node_modules/.bin/ribbin   → Local dev mode (repo-only)
./.venv/bin/ribbin           → Local dev mode (repo-only)
./vendor/bundle/bin/ribbin   → Local dev mode (repo-only)
```

## How Detection Works

1. Ribbin checks its own executable path
2. Walks up directories looking for `.git`
3. If `.git` found → Local dev mode
4. If no `.git` → Normal mode

```
/project/node_modules/.bin/ribbin
    ↓
/project/node_modules/.bin/.git? No
/project/node_modules/.git? No
/project/.git? Yes!
    ↓
Local dev mode: can only wrap in /project/**
```

## What's Allowed in Local Dev Mode

| Path | Allowed? |
|------|----------|
| `/project/node_modules/.bin/tsc` | Yes (in repo) |
| `/project/bin/custom-tool` | Yes (in repo) |
| `/usr/local/bin/curl` | No (outside repo) |
| `~/.local/bin/tool` | No (outside repo) |

## Ecosystem Support

Local dev mode works across ecosystems:

| Ecosystem | Installation Location |
|-----------|----------------------|
| npm/pnpm/yarn | `./node_modules/.bin/ribbin` |
| Python venv | `./.venv/bin/ribbin` |
| Ruby bundler | `./vendor/bundle/bin/ribbin` |
| Go modules | `./bin/ribbin` |

## Security Implications

**Protected:**
- System binaries stay safe
- User-local binaries stay safe
- Only project-local binaries can be wrapped

**Not protected:**
- Malicious code running in normal mode
- Attacker with access to globally-installed Ribbin

## When to Use Global vs Local

**Global installation (`~/.local/bin/ribbin`):**
- You want to wrap system binaries
- You're setting up machine-wide guardrails
- You trust the installation source

**Local installation (`./node_modules/.bin/ribbin`):**
- Project-specific wrapper rules
- Sandboxed from system binaries
- Safer default for untrusted contexts

## Bypassing Local Dev Mode

There's no flag to bypass local dev mode. This is intentional.

If you need to wrap system binaries:
1. Install Ribbin globally
2. Run wrap commands from there

## Verifying Mode

Check which mode Ribbin is running in:

```bash
ribbin status
# Shows: Mode: local-dev (or normal)
```

## See Also

- [Security Model](security-model.md) - Overall security design
- [How Ribbin Works](how-ribbin-works.md) - Architecture
- [Getting Started](../tutorials/getting-started.md) - Installation guide
