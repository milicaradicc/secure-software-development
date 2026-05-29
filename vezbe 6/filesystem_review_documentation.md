# Filesystem Review Module

**Skripta:** `filesystem_review.sh`

Alat je **read-only (samo za čitanje) i odbrambeni**. Koristi isključivo
standardne, legitimne sistemske komande (`cat`, `ls`, `stat`, `find`, `awk`,
`grep`, `mount`) po **LOTL (Living Off The Land)** pristupu. Ne menja fajlove,
ne eksploatiše ranjivosti i ne izvodi nikakve napadačke radnje. Neke provere
skeniraju ceo fajl sistem ili čitaju zaštićene fajlove, pa su potpunije kada
se pokrenu kao root.

## Kako se pokreće

```bash
# na sistemu koji se pregleda, radi se u /tmp
cd /tmp
cp /putanja/do/filesystem_review.sh .
chmod +x filesystem_review.sh

# pokretanje i čuvanje izlaza za izveštaj
sudo ./filesystem_review.sh 2>&1 | tee "Filesystem-$(hostname)-$(date +"%d-%b-%Y_%H-%M").txt"
```

`[OK]` = u redu, `[INFO]` = samo kontekst, `[WARN]` = pregledati, `[CRITICAL]` = ispraviti.

## Implementirane provere

| # | Provera | Korišćene komande | Bezbednosni problem koji pomaže da se uoči |
| --- | --- | --- | --- |
| 1 | Mount opcije u `/etc/fstab` | `awk`, `grep` | Nedostatak `noexec`/`nosuid`/`nodev` na `/tmp`, `/var/tmp`, `/home`, `/dev/shm` dozvoljava korisnicima da pokreću ubačene programe ili zloupotrebe podmetnute setuid fajlove. `noatime` uklanja podatke o vremenu pristupa korisne pri istrazi upada. |
| 2 | Permisije osetljivih fajlova | `stat`, `find` | Fajlovi čitljivi za sve, poput `/etc/shadow`, `my.cnf`, SSL privatnih ključeva, odaju heševe lozinki i tajne. Već jedna čitljiva kopija poništava ispravne permisije na originalu. |
| 3 | setuid / setgid binarni fajlovi | `find / -perm -4000`, `find / -perm -2000` | setuid-root program radi kao root bez obzira ko ga pokrene. Svaki neočekivani je potencijalni put za eskalaciju privilegija, pa lista treba da bude minimalna i poverljiva. |
| 4 | World-writable fajlovi i direktorijumi | `find / -perm -0002` | Fajlove u koje svako može da piše napadač može da izmeni (web shell / izmena sajta u `/var/www`, kompromitacija root-a ako je skripta koju root pokreće upisiva). World-writable direktorijumi bez sticky bita dozvoljavaju bilo kom korisniku da briše tuđe fajlove. |
| 5 | Nesigurni backup-ovi | `stat`, `find` | Backup-ovi često sadrže kopije `/etc/shadow`, ključeva ili baza. Backup fajl ili `/backup` direktorijum čitljiv za sve omogućava napadaču da izvuče tajne čak i kada su originali zaštićeni. |

## Detalji po proveri

### 1. Mount opcije (`/etc/fstab`)

Za svaku od tačaka `/tmp`, `/var/tmp`, `/home`, `/dev/shm` skripta čita 4. polje
(mount opcije) odgovarajuće linije u `/etc/fstab` i prijavljuje koje od
preporučenih opcija (`noexec`, `nosuid`, `nodev`) nedostaju. Ako tačka montiranja
nije zasebna particija, to se prijavljuje kao informativno (opcije se nasleđuju
od korenske `/`). Takođe navodi svaku particiju koja koristi `noatime`, koju
materijal označava kao nepoželjnu na osetljivim sistemima jer uklanja informaciju
o vremenu pristupa inode-u koja se koristi pri reagovanju na incidente.

### 2. Permisije osetljivih fajlova

Proverava listu poznatih osetljivih fajlova (`/etc/shadow`, `/etc/gshadow`,
`/etc/sudoers`, MySQL konfiguracioni fajlovi, GRUB konfiguracija) i prijavljuje
`[CRITICAL]` ako bitovi permisija za „ostale" (other) dozvoljavaju pristup.
Zatim pretražuje uobičajene lokacije ključeva (`/etc/ssh`, `/etc/ssl/private`,
SSL direktorijume veb servera) za fajlovima `*.key`, `*.pem` i `*_key` i
označava svaki koji je dostupan i nekom drugom osim vlasniku. Privatni ključevi i fajlovi sa lozinkama
nikada ne smeju biti čitljivi za obične korisnike.

### 3. setuid / setgid binarni fajlovi

Pokreće `find / -perm -4000` (i `-2000` za setgid), poredeći svaki rezultat sa
baznom listom binarnih fajlova koji su normalno setuid na standardnom Linux
sistemu. Poznati fajlovi se prijavljuju kao `[OK]`; sve ostalo dobija `[WARN]`
kako bi se ručno proverilo. `-xdev` zadržava pretragu na lokalnom fajl sistemu,
a `2>/dev/null` potiskuje poruke „No such file or directory".
### 4. World-writable fajlovi i direktorijumi

Pronalazi obične world-writable fajlove (`-perm -0002`), isključujući virtuelne
fajl sisteme `/proc`, `/sys`, `/dev`. Sve unutar `/var/www` podiže se na
`[CRITICAL]` jer upisivi veb koren olakšava
napredovanje napadaču. Takođe prijavljuje world-writable direktorijume bez
sticky bita, jer oni dozvoljavaju bilo kom korisniku da briše tuđe fajlove.
Izlaz je ograničen kako bi se izbeglo nekontrolisano izlistavanje na loše
podešenim sistemima.

### 5. Nesigurni backup-ovi

Traži zalutale kopije osetljivih fajlova (npr. `/etc/shadow.backup`,
`/etc/shadow.bak`, `/etc/passwd.bak`) i uobičajene backup direktorijume
(`/backup`, `/backups`, `/var/backups`). Svaki backup fajl čitljiv za sve — ili
fajl čitljiv za sve *unutar* backup direktorijuma — prijavljuje se kao
`[CRITICAL]`, čime se reprodukuje primer iz materijala u kojem napadač čita
`etc.tgz` iz `/backup` da bi povratio shadow fajl.

## Napomene i ograničenja

- Skeniranje celog fajl sistema i čitanje nekih fajlova zahtevaju root; bez
  root-a su rezultati delimični i skripta to navodi.
- Bazna lista poznatih setuid fajlova je polazna za uobičajene Debian/Ubuntu
  sisteme; `[WARN]` znači „proveri", a ne „sigurno je zlonamerno".
- Alat prijavljuje samo probleme u konfiguraciji. Ne menja permisije, ne
  uređuje `/etc/fstab` i ne briše fajlove — ispravljanje je prepušteno
  administratoru.