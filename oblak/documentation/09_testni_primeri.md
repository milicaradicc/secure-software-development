# 9. Testni primeri

Testovi se nalaze u `tests/benign/` i `tests/malicious/`.

## 9.1 Benigni testovi (prolaze verifikaciju i izvršavaju se)

| Fajl | Šta radi |
|------|----------|
| `hello.py` | Štampa pozdravnu poruku |
| `fibonacci.py` | Računa prvih 10 Fibonačijevih brojeva |
| `data_processing.py` | Prosek, min, max, sortiranje liste |
| `primes.py` | Prosti brojevi do 50 |

Ovi primeri koriste samo bezbedne operacije (matematika, stringovi, štampanje) i
treba da prođu sve provere i uspešno se izvrše u microVM-u.

## 9.2 Maliciozni testovi (treba da budu blokirani)

| Fajl | Napad | Detektuje |
|------|-------|-----------|
| `fork_bomb.py` | `os.fork()` u petlji - iscrpljivanje procesa | pattern matching |
| `reverse_shell.py` | socket + subprocess - udaljena kontrola | Bandit |
| `delete_files.py` | `shutil.rmtree("/")` - brisanje fajlova | pattern matching |
| `exfiltration.py` | čita `/etc/passwd` + mreža - krađa podataka | pattern matching |
| `cpu_bomb.py` | `while True: pass` - CPU iscrpljivanje | pattern matching |
| `read_secrets.py` | čita `/etc/shadow` - krađa hash-eva lozinki | pattern matching |

## 9.3 Očekivani rezultati

| Test | Verifikacija | Izvršavanje |
|------|--------------|-------------|
| hello.py | ✓ prolazi | ✓ izvršava se, vraća output |
| fibonacci.py | ✓ prolazi | ✓ izvršava se |
| data_processing.py | ✓ prolazi | ✓ izvršava se |
| primes.py | ✓ prolazi | ✓ izvršava se |
| fork_bomb.py | ✗ blokiran | - ne dolazi do izvršavanja |
| reverse_shell.py | ✗ blokiran | - |
| delete_files.py | ✗ blokiran | - |
| exfiltration.py | ✗ blokiran | - |
| cpu_bomb.py | ✗ blokiran | - |
| read_secrets.py | ✗ blokiran | - |

Rezultat: **10/10** testova se ponaša po očekivanju - benigni prolaze, maliciozni
su blokirani.

## 9.4 Pokretanje svih testova

```bash
./oblak deploy tests/benign/hello.py
./oblak deploy tests/benign/fibonacci.py
./oblak deploy tests/benign/data_processing.py
./oblak deploy tests/benign/primes.py

./oblak deploy tests/malicious/fork_bomb.py
./oblak deploy tests/malicious/reverse_shell.py
./oblak deploy tests/malicious/delete_files.py
./oblak deploy tests/malicious/exfiltration.py
./oblak deploy tests/malicious/cpu_bomb.py
./oblak deploy tests/malicious/read_secrets.py

#primer komande za isvršavanje sa kodom funkcije
./oblak invoke https://localhost:8000/invoke/2a2a98ed-85b2-417d-8b26-93ce9d59338a
```

## 9.5 Demonstracija odbrane u dubinu

Maliciozni testovi pokazuju da sistem ima više slojeva odbrane:

1. **Pattern matching** hvata očigledne obrasce (`os.fork`, `shutil.rmtree`,
   pristup `/etc/passwd`).
2. **Bandit** hvata složenije obrasce (subprocess + socket kod reverse shell-a).
3. Čak i da kod prođe verifikaciju, **Firecracker izolacija** bi sprečila stvarnu
   štetu (bez mreže, neprivilegovan korisnik, timeout, efemerni rootfs).
