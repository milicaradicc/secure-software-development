# 5. Mehanizmi za ublažavanje pretnji i otvorene stavke

## 5.1 Implementirane mitigacije

| Mitigacija | Pretnja koju adresira | Gde |
|------------|------------------------|-----|
| bcrypt hash lozinki | Spoofing, Information Disclosure | `internal/auth` |
| JWT token sa istekom (60 min) | Spoofing | `internal/auth` |
| Rate limiting (auth/deploy/invoke) | Denial of Service | `internal/ratelimit` |
| Upload limit 1 MiB | Denial of Service | `cmd/server` |
| Validacija `.py` ekstenzije | Tampering | `cmd/server` |
| GORM parametrizovani upiti | Tampering (SQL injection) | `internal/database` |
| SHA-256 hash uploadovanog koda | Tampering, Repudiation | `cmd/server` |
| Pattern matching opasnih obrazaca | Elevation, Information Disclosure | `internal/verifier/pipeline.go` |
| ClamAV antivirus | Tampering (malware) | `internal/verifier/antivirus.go` |
| Bandit statička analiza | Elevation, Information Disclosure | `internal/verifier/bandit.go` |
| Firecracker microVM po izvršavanju | Elevation of Privilege | `internal/orchestrator` |
| 1 vCPU + 128 MiB po VM | Denial of Service | `internal/orchestrator` |
| Timeout 10s/15s | Denial of Service | `scripts/init.sh`, orchestrator |
| VM bez mreže | Information Disclosure | `scripts/init.sh`, boot args |
| Runner neprivilegovani korisnik | Elevation of Privilege | `scripts/init.sh` |
| Efemerni rootfs | Tampering, Information Disclosure | `internal/orchestrator/rootfs.go` |
| HTTPS/TLS 1.2+ | Spoofing, Tampering, Disclosure | `cmd/server`, `certs/` |
| Audit log | Repudiation | `internal/audit` |
| Autorizacija po vlasniku (list/delete) | Information Disclosure | `cmd/server` |

## 5.2 Otvorene stavke

Sledeći mehanizmi nisu implementirani jer zahtevaju značajno više posla ili
infrastrukture. Za svaki je opisano šta bi trebalo uraditi.

### Visok prioritet

**Zaštita invoke endpointa.** Trenutno je invoke javan po function ID-u i ne
proverava vlasnika/JWT. Function ID služi kao bearer-like tajna. *Trebalo bi:*
zahtevati JWT za invoke, ili generisati potpisane kratkoživeće URL-ove (npr. HMAC
potpis sa istekom).

**Provera integriteta pre izvršavanja.** SHA-256 hash se računa pri deploy-u ali
se ne proverava pre `RunInVM`. *Trebalo bi:* uporediti hash `script.py` sa
sačuvanim hash-om neposredno pre pokretanja VM-a.

**Globalni limit paralelnih VM instanci.** Nema ograničenja broja istovremenih
VM-ova; mnogo paralelnih poziva može iscrpeti host. *Trebalo bi:* dodati semafor
(buffered channel u Go-u) koji ograničava broj istovremenih izvršavanja.

**Firecracker jailer.** Server pokreće Firecracker bez jailer-a. *Trebalo bi:*
koristiti `jailer` binary koji stavlja Firecracker u zaseban chroot, cgroup i
namespace, dodatno ograničavajući uticaj eventualnog VM escape-a.

**Produkcioni TLS sertifikat.** Trenutno self-signed. *Trebalo bi:* koristiti
sertifikat potpisan od pouzdanog CA ili reverse proxy (nginx) sa TLS terminacijom;
ukloniti `OBLAK_INSECURE_SKIP_VERIFY`.

### Srednji prioritet

**LLM analiza koda.** Implementirana je u `internal/verifier/llm.go` ali nije
aktivirana jer zahteva plaćeni `ANTHROPIC_API_KEY`. *Trebalo bi:* postaviti API
ključ i uključiti LLM korak u pipeline; LLM ocenjuje semantiku koda koju statički
alati propuštaju.

**Append-only / eksterni audit log.** Log nije nepromenjiv. *Trebalo bi:* slati
zapise na udaljeni log collector ili koristiti append-only storage.

**IP i user-agent u audit logu.** *Trebalo bi:* dodati ova polja u sve relevantne
događaje radi bolje forenzike.

**Provera integriteta artefakata.** Rootfs, kernel i Firecracker binary se ne
proveravaju pre pokretanja. *Trebalo bi:* čuvati i proveravati njihove hash-eve.

**Revokacija JWT tokena.** Nema logout/revokacije. *Trebalo bi:* uvesti blacklistu
tokena ili kratkoživeće tokene sa refresh mehanizmom.

### Niži prioritet

**Read-only rootfs sa overlay slojem.** Trenutno read-write zbog pip install.
*Trebalo bi:* read-only bazni rootfs + zaseban writable overlay.

**Disk kvote za VM.** *Trebalo bi:* ograničiti maksimalan upis unutar VM-a.

**Kvote po korisniku.** *Trebalo bi:* ograničiti ukupan broj funkcija po
korisniku.

**seccomp/AppArmor/SELinux profil** za server proces radi dodatne izolacije.
