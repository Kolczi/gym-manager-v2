# Gym Manager v2

System zarządzania siłownią — klienci, karnety, wejścia, płatności, RFID.

## Stack

- **Backend:** Go 1.25, Chi router
- **Desktop:** Wails v3 (GTK4 + WebKitGTK 6.0)
- **Frontend:** HTMX + vanilla JS
- **Baza danych:** SQLite (mattn/go-sqlite3)
- **SQL:** sqlc (generowanie kodu), goose (migracje)

## Wymagania

### Minimalne (build web / serwer)

- Go 1.25+
- GCC (CGO wymagane przez go-sqlite3)
- make

### Desktop (build Wails)

Dodatkowo:
- GTK4 + WebKitGTK 6.0 dev headers
  ```
  # Debian/Ubuntu
  sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev
  ```
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

### Narzędzia dev (opcjonalne)

- [sqlc](https://sqlc.dev/) — regeneracja kodu store (`make sqlc`)
- [goose](https://github.com/pressly/goose) — ręczne migracje (`make migrate-up`)
- [nfpm](https://nfpm.goreleaser.com/) — budowanie .deb (`make deb`)
- Node.js/npm — zmiany w frontendzie (`frontend/`)

## Build

```bash
# Headless web server (deploy na VPS)
make web        # → bin/gym-web

# Desktop app (Wails + GTK4)
make build      # → bin/gym-manager-v2

# Oba
make all
```

## Uruchamianie

### Dev (lokalnie)

```bash
make dev
# Startuje serwer HTTP na :8080 (GYM_DEV_HTTP=1)
```

### Produkcja (VPS)

Pobierz binarkę z GitHub Releases:
```bash
gh release download v0.1.0 -R Kolczi/gym-manager-v2 -p "gym-web"
```

Struktura na serwerze:
```
/opt/gym-manager/
├── bin/gym-web
├── .env
├── data/gym.db
├── internal/templates/     # szablony HTML
└── frontend/dist/          # pliki statyczne
```

### Zmienne środowiskowe

| Zmienna         | Opis                          | Domyślnie          |
|-----------------|-------------------------------|---------------------|
| `DATABASE_PATH` | Ścieżka do pliku SQLite       | `data/gym.db`       |
| `GYM_DEV_HTTP`  | `1` = tryb dev (HTTP, no Wails) | — |

Plik `.env` w katalogu roboczym jest ładowany automatycznie (godotenv).

### Systemd

```bash
sudo cp deploy/gym-manager.service /etc/systemd/system/
sudo systemctl enable --now gym-manager
```

Service czyta `/opt/gym-manager/.env` (dyrektywa `EnvironmentFile`).

## Migracja z MongoDB

Jednorazowy import danych ze starej wersji (Node.js/MongoDB):

```bash
DATABASE_PATH=data/gym.db MONGO_URI=mongodb://127.0.0.1:27017 make migrate-mongo
```

| Zmienna     | Opis                    | Domyślnie                    |
|-------------|-------------------------|-------------------------------|
| `MONGO_URI` | Connection string Mongo | `mongodb://127.0.0.1:27017`  |
| `MONGO_DB`  | Nazwa bazy Mongo        | `gymManager`                  |

## Baza danych

SQLite z WAL mode. Migracje wbudowane — aplikacja automatycznie wykonuje migracje przy starcie (`internal/dbinit/`). Pierwszy start tworzy domyślnego użytkownika admin.

Ręczne migracje (goose):
```bash
DATABASE_PATH=data/gym.db make migrate-up
DATABASE_PATH=data/gym.db make migrate-down
```

## Release

```bash
make web
gh release create v0.x.x bin/gym-web --title "v0.x.x" --notes "opis zmian"
```

## Backup

```bash
make backup
# albo ręcznie: scripts/backup.sh
```

## Licencja

Prywatne repozytorium.
