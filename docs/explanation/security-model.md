# Security Model

Understanding Ribbin's security design decisions and threat model.

## Why Security Matters

Ribbin operates on system binaries—files that execute with user privileges and can affect system behavior. A compromised or buggy wrapper tool could:

- Intercept and modify commands
- Expose sensitive data
- Escalate privileges
- Corrupt system binaries

Security is not optional for this type of tool.

## Design Principles

### 1. Defense in Depth

Multiple layers of protection rather than relying on a single check:

```
Request to wrap /usr/bin/curl
        ↓
Path validation (no traversal)
        ↓
Allowlist check (is /usr/bin allowed?)
        ↓
Confirmation required (--confirm-system-dir)
        ↓
Critical binary check (not in blocklist)
        ↓
Symlink validation (target is safe)
        ↓
File lock acquisition
        ↓
Atomic operation with rollback
        ↓
Audit logging
```

### 2. Fail Secure

When something goes wrong, Ribbin fails safely:

- **Partial operations roll back** - If symlink creation fails after rename, the original is restored
- **Unknown commands pass through** - If config parsing fails, the original command runs
- **Audit log failures don't block operations** - Logging is best-effort

### 3. Least Privilege

Ribbin doesn't require special privileges for normal operation:

- Works with user-local directories (`~/.local/bin`)
- System directory wrapping requires explicit flag (`--confirm-system-dir`)
- Never stores credentials or secrets

### 4. Transparency

All security-relevant actions are logged:

- Every wrapper installation
- Every bypass usage
- Every security violation
- All privileged operations

## Threat Model

### What Ribbin Protects Against

| Threat | Protection |
|--------|------------|
| Path traversal | Input validation, canonicalization |
| TOCTOU races | File locking throughout operations |
| Symlink attacks | Target validation, chain limits |
| Critical binary modification | Hardcoded blocklist |
| Accidental system damage | Confirmation requirements |
| Malicious packages | Local dev mode |

### What Ribbin Does NOT Protect Against

| Threat | Why |
|--------|-----|
| Kernel exploits | Out of scope—OS level |
| Physical access | Can't protect against hardware access |
| Existing root compromise | Attacker already has full control |
| Supply chain attacks | Can't verify binary authenticity |
| Social engineering | User education problem |

## Critical Binary Protection

Certain binaries can never be wrapped, regardless of flags:

- **Shells:** `bash`, `sh`, `zsh`, `fish`
- **Privilege escalation:** `sudo`, `su`, `doas`
- **Remote access:** `ssh`, `sshd`
- **Authentication:** `login`, `passwd`
- **System init:** `init`, `systemd`, `launchd`

These are hardcoded in the allowlist module. Even `--confirm-system-dir` won't override this.

Why? Wrapping these could:
- Create infinite loops (shell calling wrapped shell)
- Lock users out of their systems
- Provide privilege escalation vectors

## Local Development Mode

When Ribbin is installed inside a git repository (e.g., `node_modules/.bin/ribbin`), it automatically restricts wrapping to that repository only.

**Problem solved:** A malicious npm package can't wrap your system binaries.

**Detection:** Ribbin walks up directories looking for `.git`. If found, it's in "local dev mode" and can only wrap binaries within that repo.

## TOCTOU Prevention

Time-of-Check to Time-of-Use (TOCTOU) is a race condition:

```
Time 1: Check that /bin/curl is safe
Time 2: Attacker replaces /bin/curl
Time 3: Wrap /bin/curl (attacker's version!)
```

Ribbin prevents this by:
1. Acquiring a file lock before any check
2. Holding the lock through the entire operation
3. Verifying the file hasn't changed

## Atomic Operations

Binary operations are atomic—they either fully succeed or fully roll back:

```
1. Acquire lock on /bin/curl
2. Rename /bin/curl → /bin/curl.ribbin-original
3. Create symlink /bin/curl → ribbin
4. If step 3 fails: Rename /bin/curl.ribbin-original → /bin/curl
5. Release lock
```

The system is never left in an inconsistent state.

## Audit Trail

Every security-relevant action is logged to `~/.local/state/ribbin/audit.log`:

- **Who:** Username, UID
- **What:** Event type, binary/path
- **When:** Timestamp
- **Outcome:** Success/failure
- **Context:** Error messages, elevated status

This provides:
- Incident investigation capability
- Compliance evidence
- Pattern detection for anomalies

## Privilege Escalation Prevention

Ribbin warns and logs when running as root:

```bash
sudo ribbin wrap
# Warning: Running as root. Use caution.
# Logged with elevated=true
```

It doesn't prevent root operations (sometimes necessary), but ensures visibility.

## See Also

- [Security Features Reference](../reference/security-features.md) - Technical details
- [How Ribbin Works](how-ribbin-works.md) - Architecture overview
- [Local Dev Mode](local-dev-mode.md) - Repository protection
