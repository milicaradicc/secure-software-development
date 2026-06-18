# Oblak 

Platforma za bezbedno izvršavanje korisničkog Python koda u Firecracker microVM
okruženju (slično AWS Lambda / Google Cloud Functions).

Ova dokumentacija je organizovana po aktivnostima iz specifikacije projekta.
Svaka aktivnost je u posebnom dokumentu.

## Sadržaj

| # | Aktivnost | Dokument |
|---|-----------|----------|
| 0 | Pregled sistema i arhitektura | [00_pregled_sistema.md](00_pregled_sistema.md) |
| 1 | Bezbednosni zahtevi + analiza izolacije VM | [01_bezbednosni_zahtevi.md](01_bezbednosni_zahtevi.md) |
| 2 | Modeli pretnji (threat model) | [02_threat_model.md](02_threat_model.md) |
| 3 | Mehanizmi za reviziju (audit) | [03_revizija_audit.md](03_revizija_audit.md) |
| 4 | STRIDE analiza | [04_stride_analiza.md](04_stride_analiza.md) |
| 5 | Mehanizmi za ublažavanje + otvorene stavke | [05_mitigacije_otvorene_stavke.md](05_mitigacije_otvorene_stavke.md) |
| 7 | Statička analiza koda (gosec) | [07_staticka_analiza.md](07_staticka_analiza.md) |
| 8 | Dokumentacija (uputstvo) | [08_uputstvo.md](08_uputstvo.md) |
| 9 | Testni primeri (benigni + maliciozni) | [09_testni_primeri.md](09_testni_primeri.md) |
| 10 | Threat model dijagrami (L0/L1/L2) | [10_threat_model_diagrams.md](10_threat_model_diagrams.md) |

## Tim

- Milica Radić SV26/2022
- Mijat Krivokapić SV41/2022
- Nađa Zorić SV35/2022

## Tehnologije

Go (Gin, GORM, Cobra CLI), SQLite, JWT, bcrypt, Bandit, ClamAV, Firecracker
microVM, HTTPS/TLS.
