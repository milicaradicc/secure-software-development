# Filesystem Review Module — Documentation

**Script:** `filesystem_review.sh`
**Scope (one student):** Filesystem hardening review — chapter 6 ("FILESYSTEM
REVIEW") of the *Secure Deployment Environment* material.

This module is a companion to `users_review.sh` (users/PAM/sudo) and
`secure-deploy-audit.sh` (network/SSH/firewall). It covers the part of the
LinPEAS-style review that those two do not: how the filesystem itself is
configured and whether file permissions expose the system.

The tool is **read-only and defensive**. It uses only standard, legitimate
system commands (`cat`, `ls`, `stat`, `find`, `awk`, `grep`, `mount`) following
the **LOTL (Living Off The Land)** approach. It does not modify files, does not
exploit anything, and performs no offensive actions. Some checks scan the whole
filesystem or read protected files, so they are more complete when run as root.

## How to run

```bash
# from the audited system, working in /tmp as suggested by the material
cd /tmp
cp /path/to/filesystem_review.sh .
chmod +x filesystem_review.sh

# run and save the output for the report
sudo ./filesystem_review.sh 2>&1 | tee "Filesystem-$(hostname)-$(date +"%d-%b-%Y_%H-%M").txt"
```

Output uses the same legend as the other modules:
`[OK]` = good, `[INFO]` = context only, `[WARN]` = review this, `[CRITICAL]` = fix.

## Implemented checks

| # | Check | Commands used | Security issue it helps identify |
| --- | --- | --- | --- |
| 1 | Mount options in `/etc/fstab` | `awk`, `grep` | Missing `noexec`/`nosuid`/`nodev` on `/tmp`, `/var/tmp`, `/home`, `/dev/shm` lets users run dropped binaries or abuse planted setuid files. `noatime` removes access-time data useful in an intrusion investigation. |
| 2 | Sensitive file permissions | `stat`, `find` | World-readable `/etc/shadow`, `my.cnf`, SSL private keys, etc. leak password hashes and secrets. A single readable copy defeats correct permissions on the original. |
| 3 | setuid / setgid binaries | `find / -perm -4000`, `find / -perm -2000` | A setuid-root binary runs as root no matter who starts it. Any unexpected one is a potential privilege-escalation path, so the list should be minimal and trusted. |
| 4 | World-writable files & directories | `find / -perm -0002` | Files writable by any user can be altered by an attacker (web shell / defacement in `/var/www`, root compromise if a root-run script is writable). World-writable directories without the sticky bit let any user delete others' files. |
| 5 | Insecure backups | `stat`, `find` | Backups often contain copies of `/etc/shadow`, keys or databases. A world-readable backup file or `/backup` directory lets an attacker extract secrets even when originals are protected. |

## Detail per check

### 1. Mount options (`/etc/fstab`)

For each of `/tmp`, `/var/tmp`, `/home`, `/dev/shm` the script reads the 4th
field (mount options) of the matching `/etc/fstab` line and reports which of the
recommended options (`noexec`, `nosuid`, `nodev`) are missing. If the mount
point is not a separate partition, this is reported as informational (the
options are inherited from `/`). It also lists any partition using `noatime`,
which the material flags as undesirable on sensitive systems because it removes
inode access-time information used during incident response.

### 2. Sensitive file permissions

Checks a list of known-sensitive files (`/etc/shadow`, `/etc/gshadow`,
`/etc/sudoers`, MySQL config files, GRUB config) and reports `[CRITICAL]` if the
"other" permission bits allow access. It then searches common key locations
(`/etc/ssh`, `/etc/ssl/private`, web-server SSL directories) for `*.key`,
`*.pem` and `*_key` files and flags any that are accessible beyond the owner.
This directly reflects the material's point that private keys and password files
must never be readable by ordinary users.

### 3. setuid / setgid binaries

Runs `find / -perm -4000` (and `-2000` for setgid), comparing each result
against a baseline of binaries that are normally setuid on a standard Linux
system. Known binaries are reported `[OK]`; anything else gets a `[WARN]` so it
can be checked manually. `-xdev` keeps the scan on the local filesystem and
`2>/dev/null` suppresses the "No such file or directory" noise, exactly as the
material recommends.

### 4. World-writable files and directories

Finds world-writable regular files (`-perm -0002`), excluding the virtual
filesystems `/proc`, `/sys`, `/dev`. Anything inside `/var/www` is escalated to
`[CRITICAL]` because the material specifically warns that a writable web root
eases an attacker's progress. It also reports world-writable directories that
lack the sticky bit, since those allow any user to delete other users' files.
Output is capped to avoid runaway listings on misconfigured systems.

### 5. Insecure backups

Looks for stray copies of sensitive files (e.g. `/etc/shadow.backup`,
`/etc/shadow.bak`, `/etc/passwd.bak`) and for common backup directories
(`/backup`, `/backups`, `/var/backups`). Any world-readable backup file — or
world-readable file *inside* a backup directory — is reported `[CRITICAL]`,
reproducing the material's example where an attacker reads `etc.tgz` from
`/backup` to recover the shadow file.

## Notes and limitations

- Whole-filesystem scans and reading some files require root; without root the
  results are partial and the script says so.
- The setuid known-good list is a baseline for common Debian/Ubuntu systems; a
  `[WARN]` means "verify", not "definitely malicious".
- The tool reports configuration issues only. It does not change permissions,
  edit `/etc/fstab`, or remove files — remediation is left to the administrator.