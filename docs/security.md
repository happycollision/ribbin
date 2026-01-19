# Security Features

Ribbin includes comprehensive security hardening to protect against attacks when manipulating system binaries and critical directories.

## Overview

Ribbin writes to critical system directories (`/usr/local/bin`, `~/.local/bin`) and manipulates binary files. These operations have been hardened against common attack vectors.

## Security Features

### 1. Path Sanitization and Validation

All file paths are validated before use to prevent path traversal attacks.

**Protections:**
- Path traversal detection (`..` sequences)
- Canonicalization and absolute path resolution
- Directory allowlist enforcement
- Null byte rejection

**Example:**
```bash
# Blocked - path traversal
ribbin shim /usr/local/bin/../../etc/passwd

# Blocked - forbidden directory
ribbin shim /etc/shadow

# Allowed - user-local directory
ribbin shim ~/.local/bin/mycommand
```

**Implementation:** [internal/security/paths.go](../internal/security/paths.go)

### 2. Directory Allowlist

Only specific directories are allowed for shimming to prevent critical system modification.

**Allowed Categories:**
- User-local binaries (`~/.local/bin`, `~/bin`)
- Project-local binaries (`./bin`, `./node_modules/.bin`)
- Homebrew locations (`/usr/local/bin`, `/opt/homebrew/bin`)
- System package managers (`/usr/bin`, `/usr/local/bin` with confirmation)

**Forbidden Categories:**
- System configuration (`/etc`)
- Kernel interfaces (`/sys`, `/proc`, `/dev`)
- Core system binaries (`/bin`, `/sbin`)
- Boot files (`/boot`)

**Critical System Binaries (Always Blocked):**
- Shell: `bash`, `sh`, `zsh`, `fish`
- System: `sudo`, `su`, `login`, `init`, `systemd`
- Core utilities: `chmod`, `chown`, `rm`, `mv`, `cp`

**Implementation:** [internal/security/allowlist.go](../internal/security/allowlist.go)

### 3. File Locking (TOCTOU Prevention)

All file operations use advisory locks to prevent Time-of-Check to Time-of-Use (TOCTOU) race conditions.

**Protections:**
- Acquire lock before checking file state
- Hold lock throughout entire operation
- Verify file hasn't changed after acquiring lock
- Atomic rename operations

**Example Attack Prevented:**
```
Attacker (Thread 1)          Ribbin (Thread 2)
-----------------------      ------------------
                             Check: /bin/cat exists
Replace /bin/cat with
malicious binary
                             Shim /bin/cat
```

With file locking, Thread 2 acquires a lock before checking and holds it throughout, preventing Thread 1 from modifying the file.

**Implementation:** [internal/security/filelock.go](../internal/security/filelock.go)

### 4. Symlink Attack Prevention

Symlinks are validated to prevent attacks where an attacker creates malicious symlinks.

**Protections:**
- Symlink target validation (must be in allowed directory)
- Chain depth limits (max 10 levels)
- No symlinks in parent directory paths
- Warning for symlink shimming

**Attack Scenarios Prevented:**

**Scenario 1: Symlink Target Escape**
```bash
# Attacker creates:
ln -s /etc/passwd /usr/local/bin/mycommand

# Blocked - target outside allowed directories
ribbin shim /usr/local/bin/mycommand
```

**Scenario 2: Parent Directory Symlink**
```bash
# Attacker creates:
ln -s /etc /tmp/fake-bin
touch /tmp/fake-bin/passwd

# Blocked - parent directory contains symlink
ribbin shim /tmp/fake-bin/passwd
```

**Scenario 3: Symlink Chain Attack**
```bash
# Attacker creates deep chain:
ln -s link2 link1
ln -s link3 link2
... (15 levels deep)
ln -s /etc/passwd link15

# Blocked - chain depth exceeds limit
ribbin shim link1
```

**Implementation:** [internal/security/symlinks.go](../internal/security/symlinks.go)

### 5. Audit Logging

All security-relevant operations are logged for incident response and compliance.

**What Gets Logged:**
- Shim installations/uninstalls (success and failure)
- Bypass usage (`RIBBIN_BYPASS=1`)
- Security violations (path traversal, forbidden directories)
- Privileged operations (running as root)

**Log Location:** `~/.local/state/ribbin/audit.log`

**View audit log:**
```bash
ribbin audit show
ribbin audit show --type security.violation
ribbin audit summary
```

See [audit-logging.md](audit-logging.md) for detailed documentation.

**Implementation:** [internal/security/audit.go](../internal/security/audit.go)

### 6. Atomic Operations

File operations are atomic to prevent partial failures that leave the system in an inconsistent state.

**Protections:**
- Atomic rename with `O_EXCL` flag
- Rollback on symlink creation failure
- Verification after operations

**Example Failure Scenario:**
```
1. Rename /bin/cat → /bin/cat.ribbin-original ✓
2. Create symlink /bin/cat → ribbin ✗ (permission denied)
3. ROLLBACK: /bin/cat.ribbin-original → /bin/cat ✓
```

Without atomicity, step 2 failure would leave `/bin/cat` missing.

### 7. Environment Variable Validation

Environment variables that affect ribbin's behavior are validated.

**Protected Variables:**
- `HOME` - Used for registry location
- `XDG_CONFIG_HOME` - Used for config location
- `XDG_STATE_HOME` - Used for audit log location
- `RIBBIN_BYPASS` - Used for bypass mechanism

*Note: Full validation implementation planned in [ribbin-c1g](https://github.com/happycollision/ribbin/issues/c1g)*

### 8. Privilege Warnings

Operations that require elevated privileges are logged and warned about.

**Example:**
```bash
# Attempting to shim system directory
ribbin shim /usr/bin/cat

# Output:
# permission denied: /usr/bin/cat
#
# If you understand the security implications:
#   sudo ribbin shim cat --confirm-system-dir
```

All privileged operations are logged to the audit log with the `elevated` flag set.

## Security Best Practices

### For Users

1. **Never shim critical system binaries:**
   ```bash
   # DON'T DO THIS
   sudo ribbin shim bash
   sudo ribbin shim sudo
   ```

2. **Use user-local directories when possible:**
   ```bash
   # Good - user directory
   ribbin shim ~/.local/bin/mycommand

   # Avoid - requires sudo
   sudo ribbin shim /usr/bin/mycommand
   ```

3. **Review audit logs regularly:**
   ```bash
   ribbin audit summary
   ribbin audit show --type security.violation
   ```

4. **Be cautious with bypass:**
   ```bash
   # Only when you have a legitimate reason
   RIBBIN_BYPASS=1 command
   ```

5. **Keep ribbin updated:**
   ```bash
   go install github.com/happycollision/ribbin/cmd/ribbin@latest
   ```

### For Developers

1. **Always validate paths:**
   ```go
   if err := security.ValidateBinaryPath(path); err != nil {
       return fmt.Errorf("invalid path: %w", err)
   }
   ```

2. **Use file locking for file operations:**
   ```go
   lock, err := security.AcquireLock(path, 10*time.Second)
   if err != nil {
       return err
   }
   defer lock.Release()
   ```

3. **Log security events:**
   ```go
   if err != nil {
       security.LogSecurityViolation("operation_failed", path, details)
       return err
   }
   ```

4. **Make operations atomic:**
   ```go
   // Get state before operation
   info, _ := security.GetFileInfo(path)

   // Do operation
   if err := operation(); err != nil {
       // Rollback
       rollback()
       return err
   }

   // Verify state hasn't changed
   security.VerifyFileUnchanged(path, info)
   ```

## Threat Model

### In Scope

Ribbin protects against:

1. **Path Traversal Attacks**: Using `..` sequences to escape allowed directories
2. **TOCTOU Race Conditions**: Modifying files between check and use
3. **Symlink Attacks**: Using symlinks to target forbidden locations
4. **Unauthorized Privilege Escalation**: Shimming critical system binaries
5. **Directory Traversal**: Accessing files outside allowed directories

### Out of Scope

Ribbin does NOT protect against:

1. **Kernel Exploits**: Vulnerabilities in the OS kernel
2. **Physical Access**: Attacker with physical access to the machine
3. **Root Compromise**: Attacker already has root access
4. **Supply Chain Attacks**: Compromised dependencies
5. **Social Engineering**: Tricking users into running malicious commands

## Reporting Security Issues

If you discover a security vulnerability in ribbin:

1. **DO NOT** open a public GitHub issue
2. Email security concerns to: [security contact - TODO]
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We will respond within 48 hours and work with you to address the issue.

## Security Audits

- **Last Security Review**: January 2026
- **Security Hardening Implementation**: January 2026 (ribbin-rx1 epic)
- **Audit Logging Implementation**: January 2026 (ribbin-4rc)

## References

- [Path Sanitization](../internal/security/paths.go)
- [Directory Allowlist](../internal/security/allowlist.go)
- [File Locking](../internal/security/filelock.go)
- [Symlink Validation](../internal/security/symlinks.go)
- [Audit Logging](../internal/security/audit.go)
- [Audit Logging Documentation](audit-logging.md)

## Related Documentation

- [OWASP Path Traversal](https://owasp.org/www-community/attacks/Path_Traversal)
- [TOCTOU on Wikipedia](https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use)
- [Symlink Attack Prevention](https://en.wikipedia.org/wiki/Symlink_race)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)
