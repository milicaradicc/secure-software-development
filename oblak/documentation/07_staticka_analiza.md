# 7. Statička analiza kreiranog softvera (gosec)

Napomena: Bandit (u `internal/verifier`) skenira **korisnički Python kod** kao deo
verifikacije. Ova aktivnost se odnosi na skeniranje **našeg Go koda** alatom
`gosec`.

## 7.1 Alat i pokretanje

Korišćen je `gosec` (Go Security Checker).

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
~/go/bin/gosec -fmt=text -out=gosec_report.txt ./...
```

Skenirano: 14 fajlova, 1755 linija koda.

## 7.2 Rezultat skena

### Početno stanje

Prvi sken je prijavio **59 nalaza**.

### Posle ispravki

Nakon ispravljanja stvarnih slabosti, broj je smanjen na **50 nalaza**, svi u
kategorijama koje su analizirane i opravdane kao prihvatljiv rizik.

| Kategorija | Broj | Severity | Status |
|------------|------|----------|--------|
| G104 - neproverena greška | 36 | Low | Prihvaćeno (cleanup operacije) |
| G304 - file inclusion preko promenljive | 8 | Medium | Prihvaćeno (kontrolisane putanje) |
| G204 - subprocess sa promenljivom | 5 | Medium | Prihvaćeno (fiksne komande) |
| G703 - path traversal (CA cert) | 1 | High | Prihvaćeno (korisnikov sopstveni CLI) |

## 7.3 Ispravljeni nalazi

Sledeće slabosti su **stvarno ispravljene** u kodu:

| Kategorija | Opis | Ispravka |
|------------|------|----------|
| G302 | Audit log imao dozvole `0644` (svi mogu čitati) | Promenjeno na `0600` |
| G301 | Direktorijumi (`logs`, `functions`, rootfs, socket) na `0755` | Promenjeno na `0750` |
| G112 | Nedostajao `ReadHeaderTimeout` (Slowloris DoS) | Dodato `ReadHeaderTimeout: 10s` |
| G706 | Log injection - `port` iz env varijable u log poruci | Validacija porta + uklonjen iz log poruke |

Ovih 9 nalaza je nestalo iz izveštaja (59 → 50).

## 7.4 Tabela odluka za preostale nalaze

| ID | Primer lokacije | Severity | Odluka i obrazloženje |
|----|-----------------|----------|------------------------|
| G104 | `firecracker.go` cleanup, `audit.go` Init | Low | **Prihvaćeno.** Greške su u cleanup operacijama (`os.Remove`, `os.MkdirAll`, `os.Chown`) gde neuspeh ne utiče na bezbednost niti na ispravnost. |
| G204 | `bandit.go`, `antivirus.go`, `rootfs.go`, `firecracker.go` | Medium | **Prihvaćeno.** Nazivi komandi su konstantni (`bandit`, `clamscan`, `mount`, `firecracker`). Samo putanja fajla je promenljiva, a ona dolazi iz UUID foldera koji generiše server, ne korisnik. Nema command injection-a. |
| G304 | `pipeline.go`, `llm.go`, `rootfs.go`, `server/main.go` | Medium | **Prihvaćeno.** Putanje fajlova dolaze iz kontrolisanih izvora - UUID folderi po funkciji ili env konfiguracija. Korisnik ne kontroliše proizvoljnu putanju. |
| G703 | `cli/main.go:69` (OBLAK_CA_CERT) | High | **Prihvaćeno.** Putanja CA sertifikata dolazi iz env varijable koju korisnik postavlja za svoj sopstveni CLI. Korisnik čita svoj fajl - nije napadački vektor. |

## 7.5 Obrazloženje pristupa

Nalazi tipa G204 i G304 su **inherentni** ovakvom sistemu - platforma po svojoj
prirodi pokreće eksterne alate (Bandit, ClamAV, Firecracker) i čita fajlove sa
putanja. gosec ne može da zna da su ulazi kontrolisani, pa ih označava iz opreza.
Ručna analiza svakog nalaza pokazala je da:

- nijedan ulaz u `exec.Command` ne dolazi direktno od neautentifikovanog
  korisnika u obliku koji bi omogućio injection;
- sve putanje fajlova prolaze kroz server-generisane UUID foldere ili
  kontrolisane env varijable.
