# Secure Deployment Environment Audit

Defensive Linux hardening review helper inspired by the assignment brief in
`Secure deployment environment`, with a one-student implementation scope.

The tool is read-only. It uses local, legitimate system commands and
configuration files to summarize potentially unsafe deployment settings. It does
not exploit vulnerabilities, modify files, brute-force credentials, or perform
offensive actions.

## Implemented Scope

For one student, this project implements one larger functional unit:

**Network exposure and remote administration review**

The script also includes basic operating system context because these values are
needed to interpret network and SSH findings.

Implemented checks:

| Area | What the script checks | Security issue it helps identify |
| --- | --- | --- |
| OS context | Distribution, kernel version, uptime | Unsupported or stale systems may miss security fixes; long uptime can indicate kernel updates were not rebooted into. |
| Time management | Timezone and common NTP daemons | Incorrect time breaks log correlation, TLS checks, and time-based authentication. |
| Network context | Interfaces, routes, DNS resolver, hosts file | Unexpected interfaces, routes, or name resolution settings can expose the host or misroute traffic. |
| Listening services | TCP/UDP listeners using `ss`, `netstat`, or `lsof` | Services listening on all interfaces increase attack surface; MySQL and SSH exposure are highlighted. |
| Firewall rules | `nftables`, `iptables`, and `ufw` status | Missing or permissive inbound rules can expose services that should be restricted. |
| Firewall persistence | Common persistent rule files | Runtime firewall rules that are not persisted may disappear after reboot. |
| OpenSSH hardening | `PermitRootLogin`, `PasswordAuthentication`, `AllowTcpForwarding`, protocol, port, and allow-list directives | Weak SSH settings can allow direct root login, password brute force, unintended tunneling, or broad administrative access. |

## Usage

Run on a Linux server:
*check if there is ubuntu installed:
```bash
wsl -l -v
wsl --install -d Ubuntu
wsl -d Ubuntu
```

```bash
cd /tmp
cp "/mnt/c/Users/Nadja/Documents/secure-deployment-environment/secure-deploy-audit.sh" .
chmod +x secure-deploy-audit.sh
./secure-deploy-audit.sh --no-color 2>&1 | tee "Audit-$(hostname)-$(date +"%d-%b-%Y_%H-%M").txt"
```

Some checks are more complete when run as root: (nadja)

```bash
sudo ./secure-deploy-audit.sh
```

Install services
```bash
sudo apt update
sudo apt install ufw openssh-server
```
The script exits with:

- `0` when no warnings are found by the implemented checks
- `1` when one or more warnings are found
- `2` for invalid command-line usage

## Example Review Workflow

Save the output for documentation:

```bash
mkdir -p /tmp/audit
sudo ./secure-deploy-audit.sh --no-color > /tmp/audit/secure-deploy-audit.txt 2>&1
cp /tmp/audit/secure-deploy-audit.txt /mnt/c/Users/Nadja/Documents/secure-deployment-environment/
```

Review each `[WARN]` item and decide whether it is expected for the server role.
For example, a public web server may intentionally expose HTTP/HTTPS, but SSH
should usually be restricted to administrator IP addresses through firewall
rules.

## Notes and Limitations

- The tool is designed for Linux systems.
- It does not query online vulnerability databases. Kernel and package versions
  should be checked against the distribution security tracker manually.
- It does not edit firewall, SSH, database, or web server configuration.
- The checks are intentionally conservative: a warning means "review this",
  not always "this is definitely vulnerable".
