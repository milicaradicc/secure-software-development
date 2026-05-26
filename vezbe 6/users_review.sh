#!/bin/bash

# -------------------------- COLORS / FORMATTING ------------------------------
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

# Check root privileges (some files cannot be read without root)
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        echo -e "${YEL}[!] Script is not running as root.${RST}"
        echo -e "${YEL}    Some checks (e.g. /etc/shadow) will not be possible.${RST}"
        echo -e "${YEL}    Recommendation: sudo $0${RST}"
        echo ""
    fi
}

###############################################################################
# 1) /etc/passwd review
###############################################################################
check_passwd_file() {
    print_section "1) /etc/passwd REVIEW"

    if [ ! -r /etc/passwd ]; then
        print_critical "/etc/passwd is not readable (unusual!)"
        return
    fi

    # --- 1.1 Users with UID 0 -----------------------------------------------
    print_sub "Accounts with UID = 0 (root equivalents)"
    # awk: find all lines where the 3rd field (UID) equals 0
    uid0_users=$(awk -F: '$3 == 0 {print $1}' /etc/passwd)
    uid0_count=$(echo "$uid0_users" | wc -w)

    if [ "$uid0_count" -eq 1 ] && [ "$uid0_users" = "root" ]; then
        print_ok "Only 'root' has UID 0 (expected state)"
    else
        for u in $uid0_users; do
            if [ "$u" = "root" ]; then
                print_info "root (expected)"
            else
                print_critical "User '$u' has UID 0 — effectively root! (possible backdoor account)"
            fi
        done
    fi

    # --- 1.2 Duplicate UIDs --------------------------------------------------
    print_sub "Duplicate UIDs"
    dup_uids=$(awk -F: '{print $3}' /etc/passwd | sort | uniq -d)
    if [ -z "$dup_uids" ]; then
        print_ok "No duplicate UIDs"
    else
        for uid in $dup_uids; do
            names=$(awk -F: -v u="$uid" '$3 == u {print $1}' /etc/passwd | tr '\n' ' ')
            print_critical "UID $uid is used by multiple users: $names"
        done
    fi

    # --- 1.3 Users with an interactive shell ---------------------------------
    print_sub "Users with an interactive shell"
    # Valid shells are usually listed in /etc/shells; here we check whether
    # the shell is NOT nologin/false (i.e. whether the user can log in).
    shell_users=$(awk -F: '$7 !~ /(nologin|false|sync|halt|shutdown)$/ {print $1":"$3":"$7}' /etc/passwd)
    if [ -z "$shell_users" ]; then
        print_ok "No user has an interactive shell (very restrictive)"
    else
        while IFS=: read -r user uid shell; do
            if [ "$uid" -lt 1000 ] && [ "$user" != "root" ]; then
                # System accounts (UID < 1000) should NOT have a shell.
                print_warn "System account '$user' (UID $uid) has shell: $shell"
            else
                print_info "$user (UID $uid) -> $shell"
            fi
        done <<< "$shell_users"
    fi
}

###############################################################################
# 2) /etc/shadow review
###############################################################################
check_shadow_file() {
    print_section "2) /etc/shadow REVIEW"

    if [ ! -r /etc/shadow ]; then
        print_warn "/etc/shadow is not readable — run as root for a complete check"
        return
    fi

    # --- 2.1 Password hashing algorithm --------------------------------------
    print_sub "Password hashing algorithms (only accounts with an active password)"
    # Hash format per "crypt(3)":
    #   $1$  -> MD5
    #   $2$ / $2a$ / $2y$ -> Blowfish
    #   $5$  -> SHA-256
    #   $6$  -> SHA-512
    #   $y$  -> yescrypt (modern default on newer distributions)
    #   no '$' -> DES (legacy, max 8 chars, easily cracked)
    active_hash_count=0
    while IFS=: read -r user hash rest; do
        # Skip accounts without an active password (empty, !, *, !!)
        case "$hash" in
            ""|"!"|"*"|"!!"|"!*") continue ;;
        esac

        active_hash_count=$((active_hash_count + 1))

        case "$hash" in
            \$1\$*)    print_critical "$user: MD5 (WEAK algorithm — use SHA-512 or yescrypt)" ;;
            \$2*)      print_warn     "$user: Blowfish (acceptable, but SHA-512/yescrypt are better)" ;;
            \$5\$*)    print_ok       "$user: SHA-256" ;;
            \$6\$*)    print_ok       "$user: SHA-512" ;;
            \$y\$*)    print_ok       "$user: yescrypt (modern default)" ;;
            \$7\$*)    print_ok       "$user: scrypt" ;;
            *)
                # If it starts with a letter/digit without '$' — likely DES
                if [ "${hash:0:1}" != "\$" ]; then
                    print_critical "$user: DES (very weak — max 8 characters, trivially cracked)"
                else
                    print_warn "$user: unknown hash format ($hash)"
                fi
                ;;
        esac
    done < /etc/shadow

    if [ "$active_hash_count" -eq 0 ]; then
        print_info "No account has an active password (all locked or SSH-key only)"
    fi

    # --- 2.2 Accounts without a password -------------------------------------
    print_sub "Accounts WITHOUT a password (empty field)"
    empty_pw=$(awk -F: '($2 == "") {print $1}' /etc/shadow)
    if [ -z "$empty_pw" ]; then
        print_ok "No accounts with an empty password"
    else
        for u in $empty_pw; do
            print_critical "Account '$u' has no password set — login possible without authentication!"
        done
    fi

    # --- 2.3 Locked / disabled accounts --------------------------------------
    print_sub "Locked accounts (info)"
    locked=$(awk -F: '($2 ~ /^[!*]/) {print $1}' /etc/shadow)
    if [ -z "$locked" ]; then
        print_info "No locked accounts"
    else
        cnt=$(echo "$locked" | wc -l)
        print_info "$cnt locked/disabled accounts (usually expected for system accounts)"
    fi

    # --- 2.4 Password aging policy -------------------------------------------
    print_sub "Password aging policy (user accounts, UID >= 1000)"
    # Fields in /etc/shadow:
    #   1 user, 2 hash, 3 lastchange, 4 min, 5 max, 6 warn, 7 inactive, 8 expire
    awk -F: '{print $1":"$4":"$5":"$6}' /etc/shadow | while IFS=: read -r u mn mx wn; do
        # Filter only real users (UID >= 1000)
        uid=$(getent passwd "$u" 2>/dev/null | cut -d: -f3)
        if [ -z "$uid" ] || [ "$uid" -lt 1000 ]; then continue; fi
        if [ "$u" = "nobody" ]; then continue; fi

        # max=99999 means "never expires" -> warning
        if [ "$mx" = "99999" ] || [ -z "$mx" ]; then
            print_warn "$u: password never expires (max=$mx). Recommendation: 90 days."
        else
            print_info "$u: min=$mn, max=$mx, warn=$wn days"
        fi
    done
}

###############################################################################
# 3) PAM password policy
###############################################################################
check_pam_policy() {
    print_section "3) PAM PASSWORD POLICY"

    PAM_FILE="/etc/pam.d/common-password"
    # On RHEL/CentOS systems the path is different
    [ ! -f "$PAM_FILE" ] && PAM_FILE="/etc/pam.d/system-auth"

    if [ ! -r "$PAM_FILE" ]; then
        print_warn "PAM configuration is not readable or does not exist: $PAM_FILE"
        return
    fi

    print_info "Reading configuration from: $PAM_FILE"

    # --- 3.1 Is pam_cracklib / pam_pwquality installed? ---------------------
    print_sub "Password quality module"
    if grep -qE '^\s*password\s+.*pam_(cracklib|pwquality)\.so' "$PAM_FILE"; then
        mod=$(grep -oE 'pam_(cracklib|pwquality)\.so' "$PAM_FILE" | head -1)
        print_ok "Active module: $mod"

        # --- 3.2 Complexity parameters --------------------------------------
        print_sub "Password complexity parameters"
        line=$(grep -E '^\s*password\s+.*pam_(cracklib|pwquality)\.so' "$PAM_FILE")

        # Helper that extracts the value of the given parameter from the PAM line
        extract() {
            echo "$line" | grep -oE "$1=[-]?[0-9]+" | cut -d= -f2 | head -1
        }

        minlen=$(extract minlen)
        difok=$(extract difok)
        lcredit=$(extract lcredit)
        ucredit=$(extract ucredit)
        dcredit=$(extract dcredit)
        ocredit=$(extract ocredit)
        retry=$(extract retry)

        # minlen: recommended >= 8 (ideally 12+)
        if [ -z "$minlen" ]; then
            print_warn "minlen is not explicitly set (recommendation: >= 12)"
        elif [ "$minlen" -lt 8 ]; then
            print_critical "minlen=$minlen is too short (recommendation: >= 12)"
        else
            print_ok "minlen=$minlen"
        fi

        [ -n "$difok"   ] && print_info "difok=$difok (chars that must differ from previous password)"
        [ -n "$retry"   ] && print_info "retry=$retry (number of input attempts)"
        [ -n "$lcredit" ] && print_info "lcredit=$lcredit (lowercase letters)"
        [ -n "$ucredit" ] && print_info "ucredit=$ucredit (uppercase letters)"
        [ -n "$dcredit" ] && print_info "dcredit=$dcredit (digits)"
        [ -n "$ocredit" ] && print_info "ocredit=$ocredit (special characters)"

        # If no credit is set, a password can be weak even with minlen=8
        if [ -z "$lcredit$ucredit$dcredit$ocredit" ]; then
            print_warn "No character-type requirement is set (lcredit/ucredit/dcredit/ocredit)"
        fi
    else
        print_critical "No password quality module (cracklib/pwquality) is active!"
        print_info "Recommendation: 'apt-get install libpam-cracklib' (Debian) or 'libpwquality' (RHEL)"
    fi

    # --- 3.3 Default pam_unix.so algorithm ----------------------------------
    print_sub "Default hashing algorithm (pam_unix.so)"
    if grep -qE '^\s*password\s+.*pam_unix\.so' "$PAM_FILE"; then
        pam_unix_line=$(grep -E '^\s*password\s+.*pam_unix\.so' "$PAM_FILE")
        if   echo "$pam_unix_line" | grep -q "sha512";    then print_ok       "sha512 (recommended)"
        elif echo "$pam_unix_line" | grep -q "yescrypt";  then print_ok       "yescrypt (recommended)"
        elif echo "$pam_unix_line" | grep -q "sha256";    then print_ok       "sha256"
        elif echo "$pam_unix_line" | grep -q "blowfish";  then print_warn     "blowfish (outdated, sha512/yescrypt is better)"
        elif echo "$pam_unix_line" | grep -q "md5";       then print_critical "md5 (weak algorithm — change it!)"
        else                                                   print_warn     "Algorithm not explicitly specified (system default)"
        fi
    fi
}

###############################################################################
# 4) sudo configuration
###############################################################################
check_sudo_config() {
    print_section "4) SUDO CONFIGURATION"

    # Collect all readable sudo config files (main + drop-in files)
    sudo_files=""
    [ -r /etc/sudoers ] && sudo_files="/etc/sudoers"
    if [ -d /etc/sudoers.d ]; then
        for f in /etc/sudoers.d/*; do
            [ -f "$f" ] && [ -r "$f" ] && sudo_files="$sudo_files $f"
        done
    fi

    if [ -z "$sudo_files" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            print_warn "No sudo file is readable — run as root"
        else
            print_info "No sudo configuration file found (sudo may not be installed)"
        fi
        return
    fi

    print_info "Files analyzed:$sudo_files"

    # --- 4.1 NOPASSWD rules --------------------------------------------------
    print_sub "NOPASSWD rules (sudo without a password)"
    found_nopass=0
    for f in $sudo_files; do
        # Filter out comments and empty lines, search for NOPASSWD
        matches=$(grep -E '^\s*[^#].*NOPASSWD' "$f" 2>/dev/null)
        if [ -n "$matches" ]; then
            found_nopass=1
            while IFS= read -r line; do
                print_critical "[$f] $line"
            done <<< "$matches"
        fi
    done
    [ "$found_nopass" -eq 0 ] && print_ok "No NOPASSWD rules"

    # --- 4.2 ALL=(ALL) ALL rules for non-root users -------------------------
    print_sub "Unrestricted sudo access (ALL=(ALL) ALL)"
    for f in $sudo_files; do
        # Looks for lines starting with a username (not % group, not comment)
        matches=$(grep -E '^\s*[a-zA-Z_][a-zA-Z0-9_-]*\s+ALL\s*=\s*\(ALL(:ALL)?\)\s+ALL\s*$' "$f" 2>/dev/null)
        if [ -n "$matches" ]; then
            while IFS= read -r line; do
                user=$(echo "$line" | awk '{print $1}')
                if [ "$user" = "root" ]; then
                    print_ok "[$f] root has full access (expected)"
                else
                    print_warn "[$f] User '$user' has unrestricted sudo access"
                fi
            done <<< "$matches"
        fi
    done

    # --- 4.3 Dangerous commands allowed via sudo ----------------------------
    # Commands that, even if they are the only ones allowed, enable
    # privilege escalation (from the GTFOBins list and the lab material).
    print_sub "Dangerous commands in sudo configuration"
    dangerous="chmod chown cp dd mv tee vi vim nano less more man find awk sed perl python python3 ruby bash sh tar zip unzip rsync env make"
    found_danger=0
    for f in $sudo_files; do
        for cmd in $dangerous; do
            # Looks for the full path to a command in the sudoers file (e.g. /bin/chmod)
            if grep -qE "(=|,|\s)/[^ ,]*/${cmd}(\s|,|$)" "$f" 2>/dev/null; then
                line=$(grep -E "(=|,|\s)/[^ ,]*/${cmd}(\s|,|$)" "$f" | head -1)
                print_critical "[$f] dangerous command '$cmd' is allowed — privilege escalation possible"
                print_info "    > $line"
                found_danger=1
            fi
        done
    done
    [ "$found_danger" -eq 0 ] && print_ok "No obviously dangerous commands in sudoers files"

    # --- 4.4 Members of sudo / wheel groups ---------------------------------
    print_sub "Members of sudo / wheel groups"
    for grp in sudo wheel admin; do
        members=$(getent group "$grp" 2>/dev/null | cut -d: -f4)
        if [ -n "$members" ]; then
            print_info "Group '$grp': $members"
        fi
    done
}

###############################################################################
# 5) SSH authorized_keys in home directories
###############################################################################
check_ssh_keys() {
    print_section "5) SSH AUTHORIZED_KEYS CHECK"

    # Walk through all home directories from /etc/passwd
    while IFS=: read -r user _ uid _ _ home shell; do
        # Only real users with a shell
        [ "$uid" -lt 1000 ] && [ "$user" != "root" ] && continue
        [ -z "$home" ] || [ ! -d "$home" ] && continue

        auth_file="$home/.ssh/authorized_keys"
        if [ -f "$auth_file" ]; then
            # Check permissions (should be 600 or stricter)
            perms=$(stat -c "%a" "$auth_file" 2>/dev/null)
            owner=$(stat -c "%U" "$auth_file" 2>/dev/null)
            key_count=$(grep -cvE '^\s*(#|$)' "$auth_file" 2>/dev/null)

            print_sub "$user ($auth_file)"
            print_info "owner: $owner, permissions: $perms, key count: $key_count"

            if [ "$owner" != "$user" ]; then
                print_critical "File owner ($owner) is not the user ($user)!"
            fi
            if [ "$perms" != "600" ] && [ "$perms" != "400" ]; then
                print_warn "Permissions $perms are too open (should be 600)"
            fi
        fi
    done < /etc/passwd
}

###############################################################################
# MAIN
###############################################################################
main() {
    echo -e "${BLD}========================================================${RST}"
    echo -e "${BLD}   LINUX SECURITY AUDIT — USERS REVIEW MODULE${RST}"
    echo -e "${BLD}   Host:    $(hostname)${RST}"
    echo -e "${BLD}   Date:    $(date)${RST}"
    echo -e "${BLD}   Run by:  $(whoami) (UID $(id -u))${RST}"
    echo -e "${BLD}========================================================${RST}"

    check_root
    check_passwd_file
    check_shadow_file
    check_pam_policy
    check_sudo_config
    check_ssh_keys

    echo ""
    echo -e "${CYA}========================================================${RST}"
    echo -e "${CYA}  Review complete.${RST}"
    echo -e "${CYA}  Legend: [${GRN}OK${CYA}] = good  [${YEL}WARN${CYA}] = needs review  [${RED}CRITICAL${CYA}] = fix immediately${RST}"
    echo -e "${CYA}========================================================${RST}"
}

main "$@"
