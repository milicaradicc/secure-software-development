# 1. Bezbednosni zahtevi i analiza izolacije izvršavanja

## 1.1 Bezbednosni zahtevi celovitog sistema

Glavni ciljevi sistema su:

- sprečiti da nepoverljiv Python kod kompromituje host;
- sprečiti pristup funkcijama i podacima drugih korisnika;
- zaštititi korisničke kredencijale i JWT tokene;
- ograničiti zloupotrebu CPU, memorije, diska i broja VM instanci;
- obezbediti trag događaja kroz audit log;
- sprečiti izvršavanje očigledno malicioznog koda.

Iz ovih ciljeva izvode se konkretni zahtevi:

| ID | Zahtev |
|----|--------|
| BZ-1 | Sva komunikacija CLI–server mora ići preko šifrovanog kanala (HTTPS/TLS). |
| BZ-2 | Lozinke se čuvaju isključivo kao bcrypt hash, nikada u plain textu. |
| BZ-3 | Pristup zaštićenim endpointima zahteva validan JWT token sa istekom. |
| BZ-4 | Korisnik sme da vidi i briše samo svoje funkcije (autorizacija po vlasniku). |
| BZ-5 | Svaki uploadovani kod mora proći verifikaciju pre nego što postane izvršiv. |
| BZ-6 | Mora postojati ograničenje brzine zahteva (rate limiting) za auth, deploy, invoke. |
| BZ-7 | Veličina uploadovanog koda mora biti ograničena. |
| BZ-8 | Ključni događaji moraju biti zabeleženi u audit log. |
| BZ-9 | Korisnički kod se nikada ne izvršava direktno na hostu. |

## 1.2 Posebna analiza: izvršavanje koda u virtuelnoj mašini

Pošto je korisnički Python kod nepoverljiv, izvršavanje u Firecracker microVM je
najkritičnija bezbednosna granica celog sistema. Ova analiza stoji samostalno,
nezavisno od statičkih provera - pretpostavlja se da je kod možda prošao
verifikaciju zaobilazeći je.

### Model pretnje za izvršavanje

- bekstvo iz VM-a na host (VM escape) i kompromitacija hosta;
- iscrpljivanje resursa hosta (CPU, memorija, disk, broj instanci);
- pristup podacima drugih funkcija ili korisnika;
- mrežna eksfiltracija podataka ili uspostavljanje C2 kanala;
- trajna izmena okruženja koja bi uticala na buduća izvršavanja.

### Mehanizmi izolovanosti procesa u implementaciji

**Hardverska izolacija (KVM + Firecracker).** Svaka funkcija se izvršava u
zasebnoj microVM instanci sa sopstvenim kernelom. Za razliku od kontejnera, VM ne
deli kernel sa hostom, pa je površina napada znatno manja.

**Ograničeni resursi.** VM se konfiguriše sa 1 vCPU i 128 MiB memorije, bez SMT-a
(hyperthreading isključen radi izbegavanja side-channel napada između niti).

**Vremensko ograničenje.** Izvršavanje skripte je ograničeno na 10 sekundi unutar
VM-a, a orchestrator ima dodatni timeout od 15 sekundi. Beskonačne petlje i CPU
bombe se time automatski prekidaju.

**Bez mrežnog interfejsa.** VM se namerno pokreće bez konfigurisanog mrežnog
interfejsa - nema eksfiltracije, reverse shell-a, ni C2 komunikacije. Boot
argumenti isključuju PCI (`pci=off`) i module kernela (`nomodules`).

**Neprivilegovani korisnik.** Korisnička skripta se izvršava kao `runner`
korisnik (uid 1000), ne kao root.

**Efemerno okruženje.** Za svako izvršavanje pravi se sveža kopija rootfs-a, koja
se po završetku briše. Nema deljenog stanja između poziva.

**Izolovana priprema zavisnosti.** `pip install --no-index` unutar VM-a, bez
pristupa mreži.

**Kontrolna ravan preko Unix socketa.** Firecracker se kontroliše preko Unix
socketa, ne preko TCP porta - kontrolni API nije izložen mreži.

### Bezbednosni zahtevi specifični za izvršavanje

| ID | Zahtev |
|----|--------|
| IZV-1 | Korisnički kod se izvršava isključivo unutar microVM. |
| IZV-2 | Svaka VM ima ograničene CPU, memorijske i vremenske resurse. |
| IZV-3 | VM nema mrežni pristup u podrazumevanoj konfiguraciji. |
| IZV-4 | Kod se izvršava kao neprivilegovani korisnik unutar VM. |
| IZV-5 | Okruženje je efemerno - bez deljenog stanja između izvršavanja. |
| IZV-6 | Kontrolni interfejs Firecracker-a nije izložen mreži. |

### Otvoreni rizici za izvršavanje (vidi i dokument 5)

- **VM escape** ostaje teorijski moguć kroz ranjivost u Firecracker/KVM.
  Mitigacija: ažuriranje binarija/kernela i pokretanje pod Firecracker `jailer`-om
  (trenutno neimplementirano - otvorena stavka).
- **Ograničenje diska** nije eksplicitno postavljeno (samo posredno preko veličine
  rootfs-a).
- **Ograničenje broja istovremenih VM instanci** nije implementirano - predlaže se
  semafor.
- **Rootfs se montira read-write** zbog instalacije zavisnosti; alternativa je
  read-only rootfs sa overlay slojem.
