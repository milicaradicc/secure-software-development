# 3. Mehanizmi za reviziju (audit log)

## Implementacija

Sistem ima JSON audit log u `./logs/audit.log`. Implementiran je u
`internal/audit/audit.go`. Svaki red je jedan JSON objekat, što omogućava lako
parsiranje i pretragu (npr. preko `jq`).

Svaki zapis sadrži:

- `timestamp` - vreme događaja u UTC formatu (RFC 3339);
- `event` - tip događaja;
- `user` - korisnik koji je izvršio akciju;
- `details` - dodatni podaci specifični za događaj.

## Događaji koji se beleže

| Event | Kada se beleži | Ključni detalji |
|-------|----------------|-----------------|
| `AUTH_REGISTER` | Registracija naloga | username |
| `AUTH_LOGIN_SUCCESS` | Uspešan login | IP adresa |
| `AUTH_LOGIN_FAILURE` | Neuspešan login | IP adresa |
| `FUNCTION_DEPLOY` | Upload funkcije | function_id, naziv, veličina, SHA-256 |
| `FUNCTION_REJECTED` | Verifikacija odbila kod | function_id, razlog |
| `FUNCTION_INVOKE` | Pokretanje funkcije | function_id, exit_code, trajanje |
| `FUNCTION_DELETE` | Brisanje funkcije | function_id |

## Primer zapisa

```json
{"timestamp":"2026-06-18T17:00:47Z","event":"FUNCTION_DEPLOY","user":"milica","details":{"function_id":"b7474c0a-815b-4fdf-a10d-f8ddae79854c","name":"hello","sha256":"3bdccfd6...","size_bytes":28}}
{"timestamp":"2026-06-18T17:00:47Z","event":"FUNCTION_INVOKE","user":"anonymous","details":{"duration_ms":1718,"exit_code":0,"function_id":"b7474c0a-815b-4fdf-a10d-f8ddae79854c"}}
```

## Čemu služi revizija

Audit log omogućava rekonstrukciju ko je šta radio i kada:

- koje su funkcije deployovane i sa kojim hash-om koda (dokaz integriteta);
- koje su funkcije odbijene i zašto (trag rada verifikatora);
- kada je i koliko puta funkcija pokrenuta;
- pokušaji neuspešnog logina (detekcija brute force napada).

## Kako pregledati log

```bash
# Svi zapisi
cat logs/audit.log

# Poslednjih 20
tail -20 logs/audit.log

# Formatirano (zahteva jq)
cat logs/audit.log | jq .

# Samo odbijene funkcije
cat logs/audit.log | jq 'select(.event == "FUNCTION_REJECTED")'

# Neuspešni login pokušaji
cat logs/audit.log | jq 'select(.event == "AUTH_LOGIN_FAILURE")'
```

## Ograničenja (otvorene stavke)

- Audit log nije nepromenjiv (immutable) - ako je host kompromitovan, log se može
  izmeniti. Predlog: append-only storage ili slanje na udaljeni log collector.
- Invoke se beleži kao `anonymous` jer invoke endpoint ne zahteva autentikaciju.
- Predlog: dodati IP adresu i user-agent u sve relevantne događaje.
