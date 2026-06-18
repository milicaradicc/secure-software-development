import os
with open("/etc/shadow") as f:
    print(f.read())
