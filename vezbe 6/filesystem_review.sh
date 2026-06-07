#!/bin/bash
#
# filesystem_review.sh
#
# Linux Security Audit — FILESYSTEM REVIEW module
# (companion to users_review.sh and secure-deploy-audit.sh)
#
# This script is READ-ONLY and defensive. It uses only standard, legitimate
# system commands (cat, ls, stat, find, awk, grep, mount) to report on
# potentially unsafe filesystem configuration. It does NOT modify anything,
# does NOT exploit anything, and performs no offensive actions.
#
# Implements chapter 6 ("FILESYSTEM REVIEW") of the Secure Deployment
# Environment material:
#   - mount options in /etc/fstab (noexec / nosuid / nodev / noatime)
#   - permissions on sensitive files (shadow, private keys, my.cnf)
#   - setuid / setgid binaries
#   - world-writable files
#   - insecure backup files / directories
#
# Some checks are more complete when run as root (e.g. reading /etc/shadow
# permissions or scanning the whole filesystem). Run with: sudo ./filesystem_review.sh

if [ -t 1 ]; then
    RED="\033[1;31m"
    YEL="\033[1;33m"
    GRN="\033[1;32m"
    CYA="\033[1;36m"
    BLD="\033[1m"
    RST="\033[0m"
else
    RED=""; YEL=""; GRN=""; CYA=""; BLD=""; RST=""
fi

# Helper functions for consistent output formatting 
print_section() {
    echo ""
    echo -e "${CYA}========================================================${RST}"
    echo -e "${CYA}  $1${RST}"
    echo -e "${CYA}========================================================${RST}"
}

print_sub()      { echo -e "\n${BLD}--- $1 ---${RST}"; }
print_ok()       { echo -e "  [${GRN}OK${RST}]       $1"; }
print_warn()     { echo -e "  [${YEL}WARN${RST}]     $1"; }
print_critical() { echo -e "  [${RED}CRITICAL${RST}] $1"; }
print_info()     { echo -e "  [INFO]     $1"; }

# Check root privileges (whole-filesystem scans and some files need root)
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        echo -e "${YEL}[!] Script is not running as root.${RST}"
        echo -e "${YEL}    Some checks (full setuid/world-writable scan, /etc/shadow) may be incomplete.${RST}"
        echo -e "${YEL}    Recommendation: sudo $0${RST}"
        echo ""
    fi
}

# 1) MOUNT OPTIONS (/etc/fstab)
# Why this matters:
#   - noexec  on /tmp, /home, /var/tmp prevents users from running binaries
#             dropped there (a very common attacker foothold).
#   - nosuid  prevents setuid bits from being honoured on a partition, so a
#             planted setuid-root binary can't be used for escalation.
#   - nodev   prevents device files from being interpreted on a partition.
#   - noatime is convenient but, on sensitive systems, removes the access-time
#             information that is useful when investigating an intrusion.
check_fstab() {
    print_section "1) MOUNT OPTIONS (/etc/fstab)"

    if [ ! -r /etc/fstab ]; then
        print_warn "/etc/fstab is not readable — cannot review mount options"
        return
    fi

    # Partitions we ideally want to be hardened, and the option(s) we look for.
    # Format: "mountpoint:option1,option2"
    print_sub "Hardening options on sensitive mount points"
    for entry in "/tmp:noexec,nosuid,nodev" \
                 "/var/tmp:noexec,nosuid,nodev" \
                 "/home:nosuid,nodev" \
                 "/dev/shm:noexec,nosuid,nodev"; do
        mp="${entry%%:*}"        # mount point
        wanted="${entry#*:}"     # comma-separated wanted options

        # Find a non-comment fstab line whose 2nd field is exactly this mount point
        line=$(awk -v mp="$mp" '$1 !~ /^#/ && $2 == mp {print; exit}' /etc/fstab)

        if [ -z "$line" ]; then
            print_info "$mp is not a separate partition in /etc/fstab (options inherited from /)"
            continue
        fi

        opts=$(echo "$line" | awk '{print $4}')   # 4th field = mount options
        missing=""
        IFS=',' read -ra want_arr <<< "$wanted"
        for opt in "${want_arr[@]}"; do
            if ! echo ",$opts," | grep -q ",$opt,"; then
                missing="$missing $opt"
            fi
        done

        if [ -z "$missing" ]; then
            print_ok "$mp has hardening options: $opts"
        else
            print_warn "$mp is missing recommended option(s):$missing (current: $opts)"
        fi
    done

    # noatime check — informational, depends on the system role
    print_sub "noatime usage (forensic consideration)"
    noatime_lines=$(awk '$1 !~ /^#/ && $4 ~ /noatime/ {print $2}' /etc/fstab)
    if [ -z "$noatime_lines" ]; then
        print_ok "No partition uses noatime — access times are preserved"
    else
        for mp in $noatime_lines; do
            print_info "$mp uses noatime — access times are not updated (loses forensic data on intrusion)"
        done
    fi
}

# 2) SENSITIVE FILE PERMISSIONS
# Why this matters:
#   Files containing secrets (password hashes, private keys, DB credentials)
#   must NOT be world-readable. Even one readable copy/backup defeats correct
#   permissions on the original.
check_sensitive_files() {
    print_section "2) SENSITIVE FILE PERMISSIONS"

    # file:max_allowed_other_perm — "other" (last octal digit) should be 0
    # We flag any file that is readable or writable by "other".
    print_sub "Files that should NOT be world-readable / world-writable"
    sensitive_files="/etc/shadow /etc/gshadow /etc/sudoers \
                     /etc/mysql/my.cnf /etc/mysql/debian.cnf \
                     /boot/grub/grub.cfg"

    for f in $sensitive_files; do
        [ -e "$f" ] || continue
        perms=$(stat -c "%a" "$f" 2>/dev/null)
        owner=$(stat -c "%U" "$f" 2>/dev/null)
        # last octal digit = "other" permissions
        other="${perms: -1}"
        if [ "$other" = "0" ]; then
            print_ok "$f ($owner, $perms) is not accessible by other users"
        else
            print_critical "$f ($owner, $perms) is accessible by ANY user — restrict permissions"
        fi
    done

    # Private keys: SSH host keys and any *.key / *.pem under common locations.
    print_sub "Private key permissions"
    key_paths="/etc/ssh /etc/ssl/private /etc/apache2/ssl /etc/nginx/ssl"
    found_key=0
    for dir in $key_paths; do
        [ -d "$dir" ] || continue
        # -prune-free simple search; restrict to typical private-key file names
        while IFS= read -r key; do
            [ -e "$key" ] || continue
            found_key=1
            perms=$(stat -c "%a" "$key" 2>/dev/null)
            other="${perms: -1}"
            grp="${perms: -2:1}"
            if [ "$other" != "0" ]; then
                print_critical "$key (perms $perms) is readable/writable by other users — private keys must be 600/640"
            elif [ "$grp" -gt 0 ] 2>/dev/null && echo "$key" | grep -q "private"; then
                print_warn "$key (perms $perms) is group-accessible — confirm the group is trusted"
            else
                print_ok "$key (perms $perms) has restrictive permissions"
            fi
        done < <(find "$dir" -maxdepth 2 -type f \( -name "*_key" -o -name "*.key" -o -name "*.pem" \) 2>/dev/null)
    done
    [ "$found_key" -eq 0 ] && print_info "No private key files found in common locations"
}

# 3) SETUID / SETGID BINARIES
# Why this matters:
#   A setuid-root binary runs with root privileges regardless of who starts it.
#   Each one is a potential escalation path, so the list should be minimal and
#   limited to well-known, trusted system utilities.
check_setuid() {
    print_section "3) SETUID / SETGID BINARIES"

    if [ "$(id -u)" -ne 0 ]; then
        print_info "Not root — scan limited to readable directories; results may be partial"
    fi

    # A baseline of binaries that are normally setuid on a typical Linux system.
    # Anything NOT on this list is reported for manual review.
    known="/bin/su /usr/bin/su /bin/mount /bin/umount /usr/bin/mount /usr/bin/umount \
           /usr/bin/passwd /bin/passwd /usr/bin/sudo /usr/bin/chsh /usr/bin/chfn \
           /usr/bin/newgrp /usr/bin/gpasswd /bin/ping /bin/ping6 /usr/bin/ping \
           /usr/bin/pkexec /usr/lib/openssh/ssh-keysign /usr/bin/fusermount \
           /usr/bin/fusermount3 /usr/sbin/uuidd /usr/bin/mount.cifs"

    print_sub "setuid binaries (find / -perm -4000)"
    suid_count=0
    unexpected=0
    while IFS= read -r f; do
        [ -e "$f" ] || continue
        suid_count=$((suid_count + 1))
        if echo " $known " | grep -q " $f "; then
            print_ok "$f (known/expected setuid binary)"
        else
            print_warn "$f is setuid and NOT on the known-good list — verify it is legitimate"
            unexpected=$((unexpected + 1))
        fi
    done < <(find / -xdev -type f -perm -4000 2>/dev/null)

    if [ "$suid_count" -eq 0 ]; then
        print_info "No setuid binaries found (or insufficient privileges to scan)"
    elif [ "$unexpected" -eq 0 ]; then
        print_ok "All $suid_count setuid binaries are on the known-good list"
    fi

    print_sub "setgid binaries (find / -perm -2000)"
    sgid_count=0
    while IFS= read -r f; do
        [ -e "$f" ] || continue
        sgid_count=$((sgid_count + 1))
        print_info "$f (setgid)"
    done < <(find / -xdev -type f -perm -2000 2>/dev/null)
    [ "$sgid_count" -eq 0 ] && print_info "No setgid binaries found (or insufficient privileges to scan)"
}

# 4) WORLD-WRITABLE FILES
# Why this matters:
#   A file writable by any user can be modified by an attacker. In a web root
#   this can mean defacement or planting a web shell; for a script run by root
#   (e.g. from cron) it can mean full system compromise.
check_world_writable() {
    print_section "4) WORLD-WRITABLE FILES"

    if [ "$(id -u)" -ne 0 ]; then
        print_info "Not root — scan limited to readable directories; results may be partial"
    fi

    # World-writable regular files, excluding noisy virtual filesystems.
    print_sub "World-writable regular files (excluding /proc, /sys, /dev)"
    ww_count=0
    while IFS= read -r f; do
        [ -e "$f" ] || continue
        ww_count=$((ww_count + 1))
        # The web root is a particularly sensitive location.
        if echo "$f" | grep -q "/var/www"; then
            print_critical "$f is world-writable AND inside the web root — high risk of web shell / defacement"
        else
            print_warn "$f is world-writable — restrict write access"
        fi
        # Stop runaway output on badly-configured systems
        if [ "$ww_count" -ge 200 ]; then
            print_info "(stopping after 200 world-writable files — review the system configuration)"
            break
        fi
    done < <(find / -xdev -type f -perm -0002 \
                  ! -path "/proc/*" ! -path "/sys/*" ! -path "/dev/*" 2>/dev/null)

    [ "$ww_count" -eq 0 ] && print_ok "No world-writable regular files found"

    # World-writable files WITHOUT the sticky bit in directories is also a risk,
    # but here we focus on the files themselves per the lab material.
    print_sub "World-writable directories without sticky bit"
    wwd_count=0
    while IFS= read -r d; do
        [ -e "$d" ] || continue
        wwd_count=$((wwd_count + 1))
        print_warn "$d is world-writable and has no sticky bit — any user can delete others' files here"
        [ "$wwd_count" -ge 50 ] && { print_info "(stopping after 50 entries)"; break; }
    done < <(find / -xdev -type d -perm -0002 ! -perm -1000 \
                  ! -path "/proc/*" ! -path "/sys/*" ! -path "/dev/*" 2>/dev/null)
    [ "$wwd_count" -eq 0 ] && print_ok "No world-writable directories without sticky bit found"
}

# 5) INSECURE BACKUP FILES / DIRECTORIES
# Why this matters:
#   Backups frequently contain copies of /etc/shadow, keys or databases. If the
#   backup file or directory is world-readable, an attacker can extract the
#   secrets even when the originals are correctly protected.
check_backups() {
    print_section "5) INSECURE BACKUP FILES / DIRECTORIES"

    # 5.1 Suspicious copies of sensitive files
    print_sub "Stray copies of sensitive files"
    found_copy=0
    for f in /etc/shadow.backup /etc/shadow.bak /etc/shadow- \
             /etc/passwd.bak /etc/passwd- /root/shadow /tmp/shadow; do
        if [ -e "$f" ]; then
            found_copy=1
            perms=$(stat -c "%a" "$f" 2>/dev/null)
            other="${perms: -1}"
            if [ "$other" != "0" ]; then
                print_critical "$f exists and is readable by other users (perms $perms) — copy of a sensitive file"
            else
                print_warn "$f exists (perms $perms) — verify this backup copy is necessary"
            fi
        fi
    done
    [ "$found_copy" -eq 0 ] && print_ok "No obvious stray copies of sensitive files found"

    # 5.2 Common backup directories with permissive access
    print_sub "Backup directories"
    found_dir=0
    for d in /backup /backups /var/backups; do
        [ -d "$d" ] || continue
        found_dir=1
        perms=$(stat -c "%a" "$d" 2>/dev/null)
        other="${perms: -1}"
        if [ "$other" -gt 0 ] 2>/dev/null; then
            print_warn "$d is accessible by other users (perms $perms) — restrict access to backups"
        else
            print_ok "$d permissions are restrictive ($perms)"
        fi

        # Flag world-readable files INSIDE the backup directory
        while IFS= read -r bf; do
            [ -e "$bf" ] || continue
            bperms=$(stat -c "%a" "$bf" 2>/dev/null)
            bother="${bperms: -1}"
            if [ "$bother" -gt 0 ] 2>/dev/null; then
                print_critical "  $bf (perms $bperms) is world-readable — an attacker could extract its contents"
            fi
        done < <(find "$d" -maxdepth 1 -type f 2>/dev/null)
    done
    [ "$found_dir" -eq 0 ] && print_info "No common backup directories (/backup, /backups, /var/backups) found"
}


main() {
    echo -e "${BLD}========================================================${RST}"
    echo -e "${BLD}   LINUX SECURITY AUDIT — FILESYSTEM REVIEW MODULE${RST}"
    echo -e "${BLD}   Host:    $(hostname)${RST}"
    echo -e "${BLD}   Date:    $(date)${RST}"
    echo -e "${BLD}   Run by:  $(whoami) (UID $(id -u))${RST}"
    echo -e "${BLD}========================================================${RST}"

    check_root
    check_fstab
    check_sensitive_files
    check_setuid
    check_world_writable
    check_backups

    echo ""
    echo -e "${CYA}========================================================${RST}"
    echo -e "${CYA}  Review complete.${RST}"
    echo -e "${CYA}  Legend: [${GRN}OK${CYA}] = good  [${YEL}WARN${CYA}] = needs review  [${RED}CRITICAL${CYA}] = fix immediately${RST}"
    echo -e "${CYA}========================================================${RST}"
}

main "$@"