# Security Features Reference

Technical reference for Ribbin's security protections.

## Overview

Ribbin operates on system binaries and critical directories. These operations are hardened against common attack vectors.

## 1. Path Sanitization and Validation

**Implementation:** [internal/security/paths.go](../../internal/security/paths.go)

All file paths are validated before use.

**Protections:**
- Path traversal detection (`..` sequences)
- Canonicalization and absolute path resolution
- System directory detection
- Null byte rejection

**Blocked:**
```bash
ribbin wrap /usr/local/bin/../../etc/passwd  # Path traversal
ribbin wrap /etc/shadow                       # Forbidden directory
```

## 2. Directory Security

**Implementation:** [internal/security/allowlist.go](../../internal/security/allowlist.go)

Ribbin uses a blacklist model: all directories are allowed by default except known system directories.

### Requires --confirm-system-dir

These system directories require explicit confirmation:

- `/bin`, `/sbin`, `/usr/bin`, `/usr/sbin`
- `/usr/libexec`
- `/System` (macOS)

### Allowed by Default

All other directories are allowed without confirmation, including:

- User-local: `~/.local/bin`, `~/bin`
- Project-local: `./bin`, `./node_modules/.bin`, `./test-bin`
- Homebrew: `/usr/local/bin`, `/opt/homebrew/bin`
- Any custom directory

### Always Blocked (by name)

Critical system binaries cannot be wrapped:

- **Shell:** `bash`, `sh`, `zsh`, `fish`
- **Privilege escalation:** `sudo`, `su`, `doas`
- **Remote access:** `ssh`, `sshd`
- **Authentication:** `login`, `passwd`
- **System init:** `init`, `systemd`, `launchd`

## 3. File Locking (TOCTOU Prevention)

**Implementation:** [internal/security/filelock.go](../../internal/security/filelock.go)

Advisory locks prevent Time-of-Check to Time-of-Use race conditions.

**Protections:**
- Lock acquired before checking file state
- Lock held throughout entire operation
- File verified unchanged after lock acquisition
- Atomic rename operations

**Attack prevented:**
```
Thread 1 (attacker)        Thread 2 (Ribbin)
-------------------        -----------------
                           Check: /bin/curl exists
Replace /bin/curl
                           Shim /bin/curl ← Would wrap attacker's binary
```

With locking, Thread 2 holds the lock throughout, preventing Thread 1's modification.

## 4. Symlink Attack Prevention

**Implementation:** [internal/security/symlinks.go](../../internal/security/symlinks.go)

**Protections:**
- Symlink target must be in allowed directory
- Chain depth limit (max 10 levels)
- No symlinks allowed in parent directory paths
- Warning logged for symlink wrapping

**Scenarios prevented:**

**Target escape:**
```bash
ln -s /etc/passwd /usr/local/bin/mycommand
ribbin wrap  # Blocked - target outside allowed directories
```

**Parent directory symlink:**
```bash
ln -s /etc /tmp/fake-bin
ribbin wrap  # Blocked - parent contains symlink
```

**Chain attack:**
```bash
ln -s link2 link1
ln -s link3 link2
# ... 15 levels deep
ln -s /etc/passwd link15
ribbin wrap  # Blocked - chain depth exceeds limit
```

## 5. Audit Logging

**Implementation:** [internal/security/audit.go](../../internal/security/audit.go)

All security-relevant operations are logged.

**What gets logged:**
- Wrapper installations/uninstalls (success and failure)
- Bypass usage (`RIBBIN_BYPASS=1`)
- Security violations (path traversal, forbidden directories)
- Privileged operations (running as root)

**What is NOT logged:**
- Command arguments (avoid logging sensitive data)
- Environment variables (except `RIBBIN_BYPASS` detection)
- File contents
- Network activity

**Log location:** `~/.local/state/ribbin/audit.log`

## 6. Atomic Operations

File operations are atomic to prevent partial failures.

**Protections:**
- Atomic rename with `O_EXCL` flag
- Rollback on symlink creation failure
- Verification after operations

**Rollback example:**
```
1. Rename /bin/curl → /bin/curl.ribbin-original ✓
2. Create symlink /bin/curl → ribbin ✗ (failed)
3. ROLLBACK: /bin/curl.ribbin-original → /bin/curl ✓
```

## 7. Environment Variable Validation

Protected variables:

| Variable | Purpose |
|----------|---------|
| `HOME` | Registry location |
| `XDG_CONFIG_HOME` | Config location |
| `XDG_STATE_HOME` | Audit log location |
| `RIBBIN_BYPASS` | Bypass mechanism |

## 8. Privilege Warnings

Operations requiring elevated privileges are logged and warned.

```bash
ribbin wrap
# Output: wrapping /usr/bin/curl requires explicit confirmation
#
# Use --confirm-system-dir flag if you understand the implications

sudo ribbin wrap --confirm-system-dir
# Logged with elevated=true
```

## Threat Model

### In Scope

Ribbin protects against:

| Threat | Protection |
|--------|------------|
| Path traversal attacks | Path validation, `..` detection |
| TOCTOU race conditions | File locking |
| Symlink attacks | Target validation, chain limits |
| Unauthorized privilege escalation | Critical binary blocklist |
| System directory modification | Confirmation requirement |

### Out of Scope

Ribbin does NOT protect against:

- Kernel exploits
- Physical access attacks
- Existing root compromise
- Supply chain attacks
- Social engineering

## See Also

- [Audit Log Format](audit-log-format.md) - Log structure reference
- [How Ribbin Works](../explanation/how-ribbin-works.md) - Architecture
- [Security Model](../explanation/security-model.md) - Design decisions
