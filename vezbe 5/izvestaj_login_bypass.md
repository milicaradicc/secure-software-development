# Izveštaj — Login Bypass

## 1. Polazna tačka: statička analiza i provera aplikacije

Prvi korak bila je analiza toka autentikacije u TUDO aplikaciji. Fokus je bio na stranici:

```text
/login.php
```

Aplikacija je u alpha fazi i na login stranici navodi da se korisnici mogu prijaviti samo ako su dobili kredencijale od administratora. Međutim, proverom aplikacije utvrđeno je da postoje poznati lab nalozi sa predvidivim lozinkama:

| Korisnik | Lozinka |
|---|---|
| `admin` | `admin` |
| `user1` | `user1` |
| `user2` | `user2` |

Najkritičniji nalog je `admin`, jer prijava sa `admin:admin` odmah daje administratorsku sesiju. Time se mehanizam autentikacije zaobilazi bez potrebe za validnim, tajnim kredencijalima.

---

## 2. Opis ranjivosti

Ranjivost predstavlja upotreba **podrazumevanih i predvidivih kredencijala** u aplikaciji. Administratorski nalog koristi korisničko ime `admin` i lozinku `admin`, što je trivijalno pogoditi.

Posledice:

- napadač može da se prijavi bez prethodno kreiranog naloga;
- napadač može direktno da dobije administratorsku sesiju;
- dobijeni `PHPSESSID` može da se koristi za pristup admin funkcionalnostima;
- ovaj korak može biti početak kompletnog exploit chain-a:

```text
login bypass -> user/admin sesija -> admin funkcionalnosti -> RCE
```

U ovom konkretnom lab okruženju, prijava sa:

```text
username = admin
password = admin
```

vraća validnu sesiju i aplikacija prikazuje administratorski deo početne stranice.

---

## 3. Implementirani exploit

Napisana je Python skripta `login_bypass.py` koja automatizuje proveru login bypass-a. Skripta šalje POST zahtev na:

```text
/login.php
```

sa poznatim lab kredencijalima i dodatnim SQL injection payload-ima kao rezervnom proverom.

Tok izvršavanja:

**Korak 1 — Slanje login zahteva.** Skripta koristi `requests.Session()` i šalje korisničko ime i lozinku na `/login.php`.

**Korak 2 — Provera sesije.** Nakon login zahteva proverava se da li je server postavio `PHPSESSID` cookie.

**Korak 3 — Verifikacija pristupa.** Skripta otvara `/index.php` i proverava da li odgovor sadrži indikatore ulogovanog korisnika, kao što su `logout`, `profile`, `admin` ili `motd`.

**Korak 4 — Ispis korisne sesije.** Ako je bypass uspešan, skripta ispisuje kompletan cookie i izdvojeni `PHPSESSID`.

Primer pokretanja:

```powershell
python login_bypass.py http://localhost:8000
```

---

## 4. Rezultat

Skripta uspešno dobija validnu sesiju:

```text
[*] Target: http://localhost:8000
[*] Trying lab credentials and bypass payloads against /login.php...
[+] Login bypass succeeded
[+] Username: 'admin'
[+] Password: 'admin'
[+] Verified on: http://localhost:8000/index.php
[=] Cookie: PHPSESSID=<vrednost>
[=] PHPSESSID: <vrednost>
```

Dobijeni `PHPSESSID` predstavlja aktivnu autentifikovanu sesiju. Ako je korišćen nalog `admin`, sesija ima administratorske privilegije i može se direktno koristiti za sledeći korak exploit chain-a, odnosno RCE preko SSTI ranjivosti.

U kombinovanoj skripti `exploit_chain.py`, ovaj korak je spojen sa preostala dva koraka:

```powershell
python exploit_chain.py http://localhost:8000
```

---

## 5. Preporuke za otklanjanje

| Propust | Preporuka |
|---|---|
| Podrazumevani nalog `admin:admin` | Ukloniti default kredencijale iz produkcionog i test okruženja |
| Predvidive lozinke za lab korisnike | Koristiti jake, nasumične lozinke po okruženju |
| Admin nalog dostupan običnim login tokom | Ograničiti admin pristup dodatnim kontrolama i odvojiti inicijalizaciju admin naloga |
| Nema zaštite od pogađanja lozinki | Dodati rate limiting, lockout i monitoring neuspešnih pokušaja |
| Slaba operativna praksa | Tajne i početne lozinke čuvati van repozitorijuma i menjati pri prvom pokretanju |

