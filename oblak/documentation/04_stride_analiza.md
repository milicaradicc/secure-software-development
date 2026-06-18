# 4. STRIDE analiza

STRIDE pokriva šest kategorija pretnji: Spoofing, Tampering, Repudiation,
Information Disclosure, Denial of Service i Elevation of Privilege.

## S — Spoofing (lažno predstavljanje)

**Pretnje.** Napadač se predstavlja kao drugi korisnik krađom JWT tokena ili
brute force napadom na login.

**Postojeće kontrole.** bcrypt hash lozinki; JWT tokeni sa istekom; rate limiting
login/register; validacija `Authorization: Bearer` headera.

**Rizik.** Srednji. HTTPS smanjuje rizik presretanja, ali ukraden token i dalje
omogućava pristup dok ne istekne.

**Preporuke.** Produkcioni CA sertifikat ili reverse proxy sa TLS terminacijom;
revokacija tokena/logout; refresh token model; ograničiti login i po korisničkom
imenu, ne samo po IP.

## T — Tampering (neovlašćena izmena)

**Pretnje.** Izmena koda nakon verifikacije, izmena rootfs image-a, izmena audit
loga, manipulacija fajlovima funkcija na disku.

**Postojeće kontrole.** Kod u direktorijumu sa UUID imenom; SHA-256 hash
uploadovanog fajla; funkcija se izvršava samo ako je verifikovana; GORM
parametrizovani upiti; rootfs se kopira po izvršavanju.

**Rizik.** Srednji. Hash se računa, ali se trenutno ne koristi za proveru
integriteta neposredno pre izvršavanja.

**Preporuke.** Proveriti SHA-256 hash neposredno pre `RunInVM`; potpisati/hashovati
bazni rootfs, kernel i Firecracker binary; ograničiti OS dozvole nad `functions`,
`logs`, `oblak.db`; audit log u append-only ili eksterni sistem.

## R — Repudiation (poricanje)

**Pretnje.** Korisnik poriče deploy, delete, login ili invoke. Napadač sa
pristupom hostu menja log.

**Postojeće kontrole.** JSON audit log; loguju se auth, deploy, rejected, invoke,
delete; UTC timestamp; deploy log sadrži function ID, naziv, veličinu, SHA-256.

**Rizik.** Srednji. Audit postoji ali nije nepromenjiv, a invoke se loguje kao
`anonymous`.

**Preporuke.** Za invoke zahtevati JWT ili dokumentovati URL kao tajni; dodati IP i
user-agent; append-only log; konzistentno logovati rezultat verifikacije.

## I — Information Disclosure (curenje informacija)

**Pretnje.** Curenje kroz output funkcije, čitanje sistemskih fajlova unutar VM-a,
pristup tuđoj funkciji preko poznatog invoke URL-a.

**Postojeće kontrole.** bcrypt hash; `HashedPassword` nije izložen kroz JSON;
funkcije se listaju samo za vlasnika; izvršavanje u microVM-u; verifier blokira
neke pokušaje čitanja sistemskih lokacija; VM bez mrežnog interfejsa.

**Rizik.** Srednji do visok zbog javnog invoke endpointa.

**Preporuke.** Stroga provera TLS sertifikata u produkciji; autentikacija za
invoke ili potpisani kratkoživeći URL-ovi; filtriranje sistemskih detalja iz VM
logova; enkripcija baze i zaštita `SECRET_KEY`.

## D — Denial of Service (uskraćivanje usluge)

**Pretnje.** Veliki broj invoke zahteva, mnogo funkcija, CPU/memory exhaustion
unutar VM-a, iscrpljivanje diska privremenim rootfs kopijama.

**Postojeće kontrole.** Rate limit za login/register, deploy, invoke; upload limit
1 MiB; VM 1 vCPU / 128 MiB; VM timeout 15s; pattern matching blokira neke
beskonačne petlje.

**Rizik.** Srednji. Postoje lokalni limiti, ali nema globalnog limita paralelnih
VM-ova i rate limiter je in-memory.

**Preporuke.** Globalni semafor za maksimalan broj paralelnih VM instanci;
per-user invoke limit; ograničiti broj funkcija po korisniku; nadgledati `/tmp`;
distribuirani rate limiter za produkciju.

## E — Elevation of Privilege (eskalacija privilegija)

**Pretnje.** Maliciozni kod pokušava root unutar VM-a, VM escape kroz
Firecracker/KVM ranjivost, zloupotreba mount/rootfs procesa, uticaj na host preko
privilegovanog server procesa.

**Postojeće kontrole.** Izvršavanje u microVM-u; odvojena VM po invoke-u;
ograničeni CPU/memorija; `nomodules` boot argument; runner neprivilegovani
korisnik; verifier blokira deo opasnih obrazaca.

**Rizik.** Srednji do visok zbog visokog uticaja VM escape scenarija i činjenice
da server koristi privilegovane operacije (mount, pokretanje Firecracker-a).

**Preporuke.** Firecracker jailer; server sa najmanjim privilegijama, privilegovane
operacije u minimalan helper; seccomp/AppArmor/SELinux profili; ažuriranje
Firecracker/kernel/rootfs; provera integriteta artefakata.

## Sažetak rizika

| Kategorija | Rizik | Glavna preostala slabost |
|------------|-------|--------------------------|
| Spoofing | Srednji | Nema revokacije tokena |
| Tampering | Srednji | Hash se ne proverava pre izvršavanja |
| Repudiation | Srednji | Log nije immutable, invoke je anoniman |
| Information Disclosure | Srednji–visok | Javni invoke endpoint |
| Denial of Service | Srednji | Nema globalnog limita VM instanci |
| Elevation of Privilege | Srednji–visok | Nema jailer-a, server privilegovan |
