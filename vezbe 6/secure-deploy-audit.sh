#!/usr/bin/env bash
set -u

VERSION="1.0.0"
WARNINGS=0
INFOS=0
OKS=0

COLOR=1
if [ ! -t 1 ]; then
  COLOR=0
fi

red() { color 31 "$1"; }
green() { color 32 "$1"; }
yellow() { color 33 "$1"; }
blue() { color 34 "$1"; }

color() {
  local code="$1"
  local text="$2"
  if [ "$COLOR" -eq 1 ]; then
    printf '\033[%sm%s\033[0m' "$code" "$text"
  else
    printf '%s' "$text"
  fi
}

usage() {
  cat <<'EOF'
secure-deploy-audit.sh - defensive Linux deployment review helper

Usage:
  ./secure-deploy-audit.sh [options]

Options:
  --no-color       Disable ANSI colors
  --help           Show this help message

Scope for one-student implementation:
  - Operating system, kernel, uptime, and time synchronization context
  - Network interfaces, routing, DNS, and listening services
  - Firewall rules and persistence indicators
  - OpenSSH hardening settings

The script is read-only and uses local system commands/configuration files.
Some checks produce better results when run as root.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-color)
      COLOR=0
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown option: %s\n\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

rule() {
  printf '%*s\n' "${COLUMNS:-80}" '' | tr ' ' '-'
}

section() {
  printf '\n'
  rule
  printf '%s\n' "$(blue "$1")"
  rule
}

ok() {
  OKS=$((OKS + 1))
  printf '%s %s\n' "$(green '[OK]')" "$1"
}

warn() {
  WARNINGS=$((WARNINGS + 1))
  printf '%s %s\n' "$(yellow '[WARN]')" "$1"
}

info() {
  INFOS=$((INFOS + 1))
  printf '%s %s\n' "$(blue '[INFO]')" "$1"
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

print_command() {
  local label="$1"
  shift

  printf '\n%s\n' "$(blue "$label")"
  printf '$'
  printf ' %q' "$@"
  printf '\n'

  if "$@" 2>&1; then
    return 0
  fi

  local status=$?
  printf '(command exited with status %s)\n' "$status"
  return "$status"
}

read_key_value() {
  local file="$1"
  local key="$2"
  awk -F= -v key="$key" '
    $1 == key {
      gsub(/^"|"$/, "", $2)
      print $2
      exit
    }
  ' "$file" 2>/dev/null
}

without_comments() {
  local file="$1"
  sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' "$file" 2>/dev/null
}

show_os_context() {
  section "Operating System Context"

  if [ "$(uname -s 2>/dev/null)" != "Linux" ]; then
    warn "This script is intended for Linux systems. Results on this platform may be incomplete."
  else
    ok "Linux platform detected."
  fi

  if [ -r /etc/os-release ]; then
    local pretty_name
    pretty_name="$(read_key_value /etc/os-release PRETTY_NAME)"
    if [ -n "$pretty_name" ]; then
      info "Distribution: $pretty_name"
    else
      print_command "OS release" cat /etc/os-release
    fi
  elif [ -r /etc/debian_version ]; then
    info "Debian version: $(cat /etc/debian_version 2>/dev/null)"
  elif [ -r /etc/redhat-release ]; then
    info "RedHat release: $(cat /etc/redhat-release 2>/dev/null)"
  else
    warn "Could not identify distribution from common release files."
  fi

  if command_exists uname; then
    print_command "Kernel version" uname -a
  else
    warn "uname command is not available."
  fi

  if command_exists uptime; then
    print_command "Uptime" uptime
    local days
    days="$(uptime 2>/dev/null | grep -oE 'up[[:space:]]+[0-9]+[[:space:]]+days?' | grep -oE '[0-9]+' | head -n 1 || true)"
    if [ -n "$days" ] && [ "$days" -gt 60 ]; then
      warn "System has been up for $days days. Confirm kernel/security updates have been applied and rebooted into."
    else
      ok "Uptime does not indicate an obviously stale kernel."
    fi
  fi

  if [ -r /etc/timezone ]; then
    info "Timezone: $(cat /etc/timezone 2>/dev/null)"
  elif command_exists timedatectl; then
    print_command "Time configuration" timedatectl status
  else
    info "Timezone source not found; check system time configuration manually."
  fi

  if command_exists timedatectl; then
    local synced
    synced="$(timedatectl show -p NTPSynchronized --value 2>/dev/null || true)"
    if [ "$synced" = "yes" ]; then
      ok "NTP synchronization is enabled according to timedatectl."
    elif [ "$synced" = "no" ]; then
      warn "NTP synchronization is not enabled according to timedatectl."
    else
      info "Could not determine NTP synchronization state with timedatectl."
    fi
  fi

  if pgrep -x chronyd >/dev/null 2>&1 || pgrep -x ntpd >/dev/null 2>&1 || pgrep -x systemd-timesyncd >/dev/null 2>&1; then
    ok "A common time synchronization daemon appears to be running."
  else
    warn "No common time synchronization daemon was detected."
  fi
}

show_network_context() {
  section "Network Context"

  if command_exists ip; then
    print_command "Network addresses" ip -brief address
    print_command "Routes" ip route
  else
    warn "ip command is not available. Install iproute2 or use ifconfig/route manually."
    command_exists ifconfig && print_command "Network interfaces" ifconfig -a
    command_exists route && print_command "Routes" route -n
  fi

  if [ -r /etc/resolv.conf ]; then
    print_command "DNS resolver configuration" cat /etc/resolv.conf
  else
    warn "/etc/resolv.conf is not readable."
  fi

  if [ -r /etc/hosts ]; then
    print_command "Local host mappings" cat /etc/hosts
  else
    warn "/etc/hosts is not readable."
  fi
}

show_listening_services() {
  section "Listening Services"

  local listener_output=""
  if command_exists ss; then
    print_command "TCP/UDP listeners" ss -tulpen
    listener_output="$(ss -tulpen 2>/dev/null || true)"
  elif command_exists netstat; then
    print_command "TCP/UDP listeners" netstat -tulpen
    listener_output="$(netstat -tulpen 2>/dev/null || true)"
  elif command_exists lsof; then
    print_command "TCP listeners" lsof -i TCP -n -P
    print_command "UDP listeners" lsof -i UDP -n -P
    listener_output="$(lsof -i TCP -n -P 2>/dev/null; lsof -i UDP -n -P 2>/dev/null || true)"
  else
    warn "No supported listener command found (ss, netstat, or lsof)."
  fi

  if printf '%s\n' "$listener_output" | grep -Eq '(^|[[:space:]])(\*|0\.0\.0\.0|\[::\]):(22|ssh)([[:space:]]|$)'; then
    warn "SSH appears to listen on all interfaces. Restrict firewall access to administrator IP addresses."
  fi

  if printf '%s\n' "$listener_output" | grep -Eq '(^|[[:space:]])(\*|0\.0\.0\.0|\[::\]):(3306|mysql)([[:space:]]|$)'; then
    warn "MySQL appears to listen on all interfaces. Bind it to 127.0.0.1 unless remote database access is required."
  fi

  if printf '%s\n' "$listener_output" | grep -Eq '(^|[[:space:]])(\*|0\.0\.0\.0|\[::\]):(80|http|443|https)([[:space:]]|$)'; then
    info "Web service appears to be exposed. Confirm only intended ports are public."
  fi
}

show_firewall() {
  section "Firewall Review"

  local saw_firewall=0

  if command_exists nft; then
    saw_firewall=1
    print_command "nftables ruleset" nft list ruleset
  fi

  if command_exists iptables; then
    saw_firewall=1
    print_command "iptables filter rules" iptables -L -v -n
    local input_policy
    input_policy="$(iptables -S INPUT 2>/dev/null | awk '/^-P INPUT/ {print $3; exit}')"
    if [ "$input_policy" = "DROP" ] || [ "$input_policy" = "REJECT" ]; then
      ok "iptables INPUT default policy is restrictive: $input_policy."
    elif [ -n "$input_policy" ]; then
      warn "iptables INPUT default policy is $input_policy. Confirm explicit rules restrict inbound traffic."
    fi
  fi

  if command_exists ufw; then
    saw_firewall=1
    print_command "ufw status" ufw status verbose
  fi

  if [ "$saw_firewall" -eq 0 ]; then
    warn "No supported firewall frontend found (nft, iptables, or ufw)."
  fi

  local persistence_found=0
  for file in \
    /etc/iptables/rules.v4 \
    /etc/iptables/rules.v6 \
    /etc/iptables.up.rules \
    /etc/network/if-pre-up.d/iptables \
    /etc/nftables.conf; do
    if [ -e "$file" ]; then
      persistence_found=1
      if [ -r "$file" ]; then
        info "Firewall persistence candidate found: $file"
      else
        warn "Firewall persistence candidate exists but is not readable: $file"
      fi
    fi
  done

  if [ "$persistence_found" -eq 1 ]; then
    ok "Firewall persistence configuration indicators were found."
  else
    warn "No common firewall persistence files were found. Verify rules survive reboot."
  fi
}

sshd_effective_value() {
  local key="$1"

  if command_exists sshd; then
    sshd -T 2>/dev/null | awk -v key="$(printf '%s' "$key" | tr '[:upper:]' '[:lower:]')" '$1 == key {print $2; exit}'
    return
  fi

  if [ -r /etc/ssh/sshd_config ]; then
    without_comments /etc/ssh/sshd_config | awk -v key="$(printf '%s' "$key" | tr '[:upper:]' '[:lower:]')" '
      tolower($1) == key {print $2; found=1}
      END {if (!found) exit 1}
    '
  fi
}

show_ssh() {
  section "OpenSSH Hardening"

  if [ -r /etc/ssh/sshd_config ]; then
    print_command "Active sshd_config directives" sh -c "sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' /etc/ssh/sshd_config"
  else
    warn "/etc/ssh/sshd_config is not readable or OpenSSH server is not installed."
    return
  fi

  local permit_root_login
  permit_root_login="$(sshd_effective_value PermitRootLogin || true)"
  case "$permit_root_login" in
    no)
      ok "PermitRootLogin is disabled."
      ;;
    prohibit-password|forced-commands-only)
      warn "PermitRootLogin is partially restricted ($permit_root_login). Prefer 'no' for internet-facing servers."
      ;;
    yes|"")
      warn "PermitRootLogin is not safely disabled. Set 'PermitRootLogin no'."
      ;;
    *)
      warn "PermitRootLogin has unusual value: $permit_root_login."
      ;;
  esac

  local password_auth
  password_auth="$(sshd_effective_value PasswordAuthentication || true)"
  case "$password_auth" in
    no)
      ok "PasswordAuthentication is disabled."
      ;;
    yes|"")
      warn "PasswordAuthentication is enabled or unspecified. Prefer key-based auth for exposed servers."
      ;;
    *)
      warn "PasswordAuthentication has unusual value: $password_auth."
      ;;
  esac

  local tcp_forwarding
  tcp_forwarding="$(sshd_effective_value AllowTcpForwarding || true)"
  case "$tcp_forwarding" in
    no)
      ok "AllowTcpForwarding is disabled."
      ;;
    yes|"")
      warn "AllowTcpForwarding is enabled or unspecified. Disable it unless this host is intended as a jump/bastion server."
      ;;
    *)
      warn "AllowTcpForwarding has unusual value: $tcp_forwarding."
      ;;
  esac

  local protocol
  protocol="$(sshd_effective_value Protocol || true)"
  if [ "$protocol" = "1" ]; then
    warn "SSH protocol 1 is enabled. Use protocol 2 only."
  else
    ok "No SSH protocol 1 setting was detected."
  fi

  local port
  port="$(sshd_effective_value Port || true)"
  if [ -n "$port" ] && [ "$port" != "22" ]; then
    info "SSH is configured on non-default port $port. Confirm firewall rules match this value."
  else
    info "SSH uses the default port 22. This is acceptable, but exposes the service to common automated scans."
  fi

  local allow_users allow_groups
  allow_users="$(sshd_effective_value AllowUsers || true)"
  allow_groups="$(sshd_effective_value AllowGroups || true)"
  if [ -n "$allow_users" ] || [ -n "$allow_groups" ]; then
    ok "SSH login allow-list directive detected."
  else
    info "No AllowUsers or AllowGroups directive detected. Consider an allow-list for administrative access."
  fi
}

main() {
  printf 'Secure Deployment Environment Audit v%s\n' "$VERSION"
  printf 'Host: %s\n' "$(hostname 2>/dev/null || printf unknown)"
  printf 'Started: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date 2>/dev/null || printf unknown)"

  if [ "$(id -u 2>/dev/null || printf 1)" != "0" ]; then
    info "Running without root privileges. Some firewall and process-owner details may be unavailable."
  fi

  show_os_context
  show_network_context
  show_listening_services
  show_firewall
  show_ssh

  section "Summary"
  printf '%s OK, %s warnings, %s informational notes\n' "$OKS" "$WARNINGS" "$INFOS"
  if [ "$WARNINGS" -gt 0 ]; then
    printf 'Review warnings above and apply hardening changes appropriate to the server role.\n'
    exit 1
  fi
  printf 'No warnings were raised by the implemented checks.\n'
}

main
