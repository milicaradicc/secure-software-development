# Documentation: `users_review.sh`

## 1. Overview

`users_review.sh` is part of a larger tool for auditing the security configuration of a Linux system, inspired by the [LinPEAS](https://github.com/peass-ng/PEASS-ng) project. This module covers the **Users Review** section from the *Secure Deployment Environment* document.

The script is **defensive in nature**, it finds and reports on potentially insecure configurations, but does not perform any offensive or exploitation actions. It uses an exclusively **LOTL approach** (Living Off The Land), meaning it relies only on standard system commands that are already present on every Linux system.

### Commands used

`cat`, `awk`, `grep`, `find`, `ls`, `stat`, `getent`, `id`, `hostname`, `whoami`, `date`, `cut`, `sort`, `uniq`, `wc`, `tr`, `head`

No additional libraries or tools are installed.

---

## 2. Running the script

### Prerequisites
- Linux system (tested on Ubuntu/Debian/WSL)
- `bash`
- Root access for complete results (access to `/etc/shadow`, `/etc/sudoers`)

### Commands

```bash
# 1. Make the script executable
chmod +x users_review.sh

# 2. Run it (root is recommended)
sudo ./users_review.sh

# 3. Save the report to a file
sudo ./users_review.sh > report.txt 2>&1

# 4. Or display on screen and save at the same time
sudo ./users_review.sh 2>&1 | tee report.txt
```

### Behavior without root access

If the script is run as a regular user, it will print a warning and continue, however, sections that require access to protected files (`/etc/shadow`, `/etc/sudoers`) will be skipped with a message about insufficient privileges.

---

## 3. Implemented functionality

The script is divided into **5 functional sections**. Each section detects a specific type of security problem and classifies findings by severity.

### 3.1. `/etc/passwd` review

**Function in script:** `check_passwd_file()`

**What it checks:**

| Check | Command | Security issue it detects |
|---|---|---|
| Users with UID = 0 | `awk -F: '$3 == 0' /etc/passwd` | **Backdoor accounts** — any user with UID 0 has identical privileges to `root`. This is a classic technique for maintaining access after system compromise. |
| Duplicate UIDs | `awk + sort + uniq -d` | **Lack of identity separation** — two users sharing the same UID share permissions, which makes per-user tracking and auditing impossible. |
| Users with an interactive shell | `awk` filter on field 7 | **Increased attack surface** — system accounts (`www-data`, `mysql`, etc.) should not have a shell. If they do, compromising the service may allow interactive login. |
| System accounts (UID < 1000) with a shell | Combined check | **Privilege escalation vector** — service accounts should use `/usr/sbin/nologin` or `/bin/false`. |

**Example critical finding:**
```
[CRITICAL] User 'backdoor' has UID 0 — effectively root!
[CRITICAL] UID 1500 is used by multiple users: dupli1 dupli2
```

#### 🔧 Suggested remediation

**Problem: Additional account with UID 0**
```bash
# 1. Check if the account is legitimate (perhaps a leftover from migration)
sudo getent passwd | awk -F: '$3 == 0'

# 2. If not legitimate — delete the account immediately
sudo userdel -r backdoor

# 3. Review /var/log/auth.log for suspicious logins by that account
sudo grep backdoor /var/log/auth.log
```

**Problem: Duplicate UIDs**
```bash
# Find files owned by the duplicate UID
sudo find / -uid 1500 -ls 2>/dev/null

# Change the UID of one of the accounts and update file ownership
sudo usermod -u 1501 dupli2
sudo find / -uid 1500 -user dupli2 -exec chown 1501 {} \;
```

**Problem: System account (UID < 1000) with an interactive shell**
```bash
# Change the service account's shell to nologin
sudo usermod -s /usr/sbin/nologin www-data

# Verify after the change
getent passwd www-data
```

---

### 3.2. `/etc/shadow` review

**Function in script:** `check_shadow_file()`

**What it checks:**

| Check | Command | Security issue it detects |
|---|---|---|
| Password hashing algorithm | `case` matching on hash prefix | **Weak cryptographic algorithms** — DES limits passwords to 8 characters and can be cracked in seconds; MD5 is trivial to brute-force with modern GPUs. SHA-512 or yescrypt are recommended. |
| Accounts without a password | `awk -F: '$2 == ""'` | **Anonymous login** — an account with an empty password field can be used without any authentication. |
| Locked/disabled accounts | `awk` check for `!` or `*` | Informational — helps understand which accounts are active. |
| Password aging policy | Fields 4–6 in `/etc/shadow` | **Compromised passwords remain valid indefinitely** — if `max` is not set (value 99999), the password never expires. Recommendation: 90 days. |

**Detected algorithms:**

| Hash prefix | Algorithm | Status |
|---|---|---|
| (no `$`) | DES | ❌ CRITICAL |
| `$1$` | MD5 | ❌ CRITICAL |
| `$2$`, `$2a$`, `$2y$` | Blowfish | ⚠️ WARN |
| `$5$` | SHA-256 | ✅ OK |
| `$6$` | SHA-512 | ✅ OK |
| `$y$` | yescrypt | ✅ OK |

#### 🔧 Suggested remediation

**Problem: Weak hashing algorithm (DES/MD5)**
```bash
# 1. Change the default algorithm in the PAM configuration
sudo sed -i 's/pam_unix.so.*$/& sha512/' /etc/pam.d/common-password
# (alternatively: manually add 'sha512' to the pam_unix.so line)

# 2. Force all users to change their password on next login
# (a new hash will be generated using the new algorithm)
sudo chage -d 0 username

# 3. For all users at once
awk -F: '$3 >= 1000 && $1 != "nobody" {print $1}' /etc/passwd | \
  xargs -I{} sudo chage -d 0 {}
```

**Problem: Account without a password**
```bash
# 1. Lock the account immediately
sudo passwd -l username

# 2. Set a new password (or delete the account if not needed)
sudo passwd username
# or
sudo userdel -r username

# 3. Check all accounts without a password
sudo awk -F: '($2 == "") {print $1}' /etc/shadow
```

**Problem: Password never expires (max=99999)**
```bash
# Set policy for a single user:
#   -M 90  → max 90 days
#   -m 7   → min 7 days between changes
#   -W 14  → warning 14 days before expiration
sudo chage -M 90 -m 7 -W 14 username

# Set defaults for all new users in /etc/login.defs:
sudo sed -i 's/^PASS_MAX_DAYS.*/PASS_MAX_DAYS   90/' /etc/login.defs
sudo sed -i 's/^PASS_MIN_DAYS.*/PASS_MIN_DAYS   7/'  /etc/login.defs
sudo sed -i 's/^PASS_WARN_AGE.*/PASS_WARN_AGE   14/' /etc/login.defs

# Verify
sudo chage -l username
```

---

### 3.3. PAM password policy review

**Function in script:** `check_pam_policy()`

**What it checks:**

| Check | Command | Security issue it detects |
|---|---|---|
| Presence of `pam_cracklib` / `pam_pwquality` | `grep` over PAM files | **Weak passwords** — without these modules, users can set any password (even `1234`), making the system a trivial target for brute-force attacks. |
| `minlen` parameter | `grep -oE 'minlen=[0-9]+'` | **Short passwords** — a value below 8 (ideally 12+) drastically reduces the time required for cracking. |
| `lcredit`, `ucredit`, `dcredit`, `ocredit` | Same as above | **Low password entropy** — without requirements for different character types, passwords like `passwordpassword` pass the length check but remain weak. |
| `difok` | Same | **Recycled passwords** — without this parameter, the user can change only one character and consider it a "new" password. |
| Default algorithm in `pam_unix.so` | `grep` for keywords | Determines which algorithm is used the next time a password is set. |

**Configuration locations:**
- Debian/Ubuntu: `/etc/pam.d/common-password`
- RHEL/CentOS/Fedora: `/etc/pam.d/system-auth`

#### 🔧 Suggested remediation

**Problem: No password quality module**
```bash
# Debian/Ubuntu
sudo apt-get update
sudo apt-get install libpam-cracklib  # or libpam-pwquality

# RHEL/CentOS/Fedora (already installed in most cases)
sudo dnf install libpwquality
```

**Problem: minlen too small or unset / missing credit parameters**
```bash
# Open the PAM configuration
sudo nano /etc/pam.d/common-password

# Add/modify the line (above pam_unix.so):
password requisite pam_cracklib.so retry=3 minlen=12 difok=4 \
    ucredit=-1 lcredit=-1 dcredit=-1 ocredit=-1

# Meaning:
#   retry=3       → 3 attempts to enter the new password
#   minlen=12     → minimum length 12 characters
#   difok=4       → 4 characters must differ from the old password
#   ucredit=-1    → at least 1 uppercase letter
#   lcredit=-1    → at least 1 lowercase letter
#   dcredit=-1    → at least 1 digit
#   ocredit=-1    → at least 1 special character
```

**Problem: pam_unix.so does not specify an algorithm (legacy default is used)**
```bash
# In /etc/pam.d/common-password, find the pam_unix.so line and add 'sha512' or 'yescrypt':
password [success=1 default=ignore] pam_unix.so obscure sha512

# Verify
sudo grep pam_unix /etc/pam.d/common-password
```

**Additional: block reuse of previous passwords**
```bash
# Prevent users from reusing the last 5 passwords:
sudo nano /etc/pam.d/common-password
# Add 'remember=5' to the pam_unix.so line:
password [success=1 default=ignore] pam_unix.so obscure sha512 remember=5
```

---

### 3.4. `sudo` configuration review

**Function in script:** `check_sudo_config()`

**What it checks:**

| Check | Command | Security issue it detects |
|---|---|---|
| `NOPASSWD` rules | `grep -E '^\s*[^#].*NOPASSWD'` | **User account compromise = instant root** — if an attacker gets hold of a user session (e.g. stolen SSH key), they can immediately escalate without knowing the password. |
| `ALL=(ALL) ALL` rules for non-root users | regex over `sudoers` files | **Unrestricted administrative access** — often unnecessary; most users only need a limited set of commands. |
| Dangerous commands in `sudoers` | `grep` over a list of dangerous commands | **Privilege escalation via legitimate commands** — commands like `chmod`, `chown`, `vi`, `find`, `less`, `python` allow breaking out of a restricted sudo set into a root shell (see [GTFOBins](https://gtfobins.github.io/)). |
| Members of `sudo`, `wheel`, `admin` groups | `getent group` | Informational — who can become root. |

**List of dangerous commands the script looks for:**

`chmod`, `chown`, `cp`, `dd`, `mv`, `tee`, `vi`, `vim`, `nano`, `less`, `more`, `man`, `find`, `awk`, `sed`, `perl`, `python`, `python3`, `ruby`, `bash`, `sh`, `tar`, `zip`, `unzip`, `rsync`, `env`, `make`

**Example escalation (why `chmod` is dangerous):**
A user with `sudo` permission to run `/bin/chmod` can:
1. Copy `/bin/bash` to their home directory
2. Run `sudo /bin/chmod u+s ~/bash`
3. Run `~/bash -p` and obtain a root shell

#### 🔧 Suggested remediation

> ⚠️ **IMPORTANT:** always edit the `sudoers` file via the `visudo` command — it validates the syntax before saving. An error in `/etc/sudoers` can completely disable sudo on the system.

**Problem: NOPASSWD rule**
```bash
sudo visudo

# Change the line:
#   user ALL=(ALL) NOPASSWD: ALL
# to:
#   user ALL=(ALL) ALL

# If NOPASSWD is absolutely necessary (e.g. for automated deployment),
# limit it to a specific command only:
#   deploy ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart myapp
```

**Problem: User with unrestricted ALL=(ALL) ALL access**
```bash
# 1. Consider whether the user really needs full access
# 2. Define a Cmnd_Alias with only the required commands:

sudo visudo
# Add:
Cmnd_Alias WEB_ADMIN = /bin/systemctl restart apache2, \
                       /bin/systemctl reload apache2, \
                       /usr/bin/tail -f /var/log/apache2/*

webadmin ALL=(ALL) WEB_ADMIN
```

**Problem: Dangerous commands in sudoers (chmod, vi, find, ...)**
```bash
sudo visudo

# Before:
#   user ALL=(ALL) NOPASSWD: /bin/chmod, /usr/bin/find
# After (best solution — remove dangerous commands):
#   user ALL=(ALL) /bin/systemctl status myapp

# If a dangerous command must stay, restrict it to specific arguments:
#   user ALL=(ALL) /bin/chmod 644 /var/www/html/*

# Log every sudo command invocation:
echo 'Defaults logfile=/var/log/sudo.log' | sudo EDITOR='tee -a' visudo
```

**Problem: Review who has sudo access**
```bash
# List members of the sudo group
getent group sudo

# Remove a user from the sudo group if access is no longer needed:
sudo deluser username sudo     # Debian/Ubuntu
sudo gpasswd -d username wheel # RHEL/CentOS
```

---

### 3.5. SSH `authorized_keys` review

**Function in script:** `check_ssh_keys()`

**What it checks:**

| Check | Command | Security issue it detects |
|---|---|---|
| Permissions on `authorized_keys` | `stat -c "%a"` | **Modification by other users** — if the file has permissions wider than `600`, another user can append their public key and log in as that user. |
| File owner | `stat -c "%U"` | **Wrong owner** — if the owner is not the user themselves, SSH may reject the login (security check) or behave unexpectedly. |
| Number of public keys | `grep -c` | Informational — helps notice if more keys have been added than expected. |

#### 🔧 Suggested remediation

**Problem: Wrong permissions on `authorized_keys`**
```bash
# Permissions must be exactly 600 (only owner can read and write)
chmod 600 ~/.ssh/authorized_keys

# The .ssh directory should be 700
chmod 700 ~/.ssh

# For all users at once:
for home in /home/*; do
    user=$(basename "$home")
    if [ -f "$home/.ssh/authorized_keys" ]; then
        sudo chmod 700 "$home/.ssh"
        sudo chmod 600 "$home/.ssh/authorized_keys"
        sudo chown -R "$user:$user" "$home/.ssh"
    fi
done
```

**Problem: Wrong file owner**
```bash
# Restore ownership to the legitimate user
sudo chown username:username /home/username/.ssh/authorized_keys

# SSH will reject login if StrictModes (default: yes) detects
# improper permissions or ownership — that's why this matters.
```

**Problem: Unexpected public key in authorized_keys**
```bash
# 1. Review all keys and their comment (the last field contains name/host)
cat ~/.ssh/authorized_keys

# 2. Delete the unknown key
nano ~/.ssh/authorized_keys
# (manually remove the line with the unknown key)

# 3. Check login history for that account
sudo grep "Accepted publickey for username" /var/log/auth.log

# 4. Consider rotating all SSH keys if compromise is suspected
```

**Additional: general SSH recommendation**
```bash
# Disable password authentication (keys only)
# in /etc/ssh/sshd_config:
PasswordAuthentication no
PubkeyAuthentication yes
PermitRootLogin no

sudo systemctl restart sshd
```

---

## 4. Interpreting the output

The script classifies findings into 4 levels:

| Label | Color | Meaning |
|---|---|---|
| `[OK]` | green | Configuration is in line with recommendations |
| `[INFO]` | white | Informational finding — does not indicate a problem, just shows state |
| `[WARN]` | yellow | Not critical, but should be reviewed / improved |
| `[CRITICAL]` | red | Critical security problem that should be fixed immediately |

**Colors are automatically disabled** when the output is redirected to a file (`> report.txt`), so the file remains readable without ANSI escape sequences.

---

## 5. Limitations

- The script has been tested on **Debian/Ubuntu** systems. On RHEL/CentOS/Fedora most checks work, but configuration file paths may differ (the script attempts to detect both variants).
- Some checks require **root** privileges; without them the script degrades gracefully with a warning.
- The "dangerous commands" check in `sudoers` uses a list of well-known `GTFOBins` commands — it may miss some exotic cases.
- The script **does not modify** anything on the system — it only reads configuration.

---