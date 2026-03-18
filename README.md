# ClipSync — Universal Clipboard via Bluetooth

> Sync your clipboard between macOS/Linux and Android over Bluetooth Low Energy. No cloud, no WiFi required.

---

## Features

| Feature | Description |
|---|---|
| **Bidirectional Sync** | Copy on desktop → paste on Android, and vice versa |
| **Multi-Desktop** | N desktops sync through one Android (relay hub) |
| **QR Pairing** | Secure token-based pairing via QR code scan |
| **BLE Only** | Works over Bluetooth LE — no WiFi or internet needed |
| **Chunked Transfer** | Supports large text (code, articles) up to ~126KB |
| **Web Dashboard** | Clean flat UI at `localhost:8066` with history, search, stats |
| **SQLite History** | Persistent clipboard history with search |
| **Persistent Pairing** | Pair once, stays paired across restarts |
| **Cross-platform** | Runs on macOS and Linux (auto-detected) |
| **Accessibility Service** | Automatic Android → desktop sync (no manual action) |

## Architecture

```
┌──────────────────┐                        ┌──────────────────┐
│  Desktop 1 (Go)  │──►┐                    │  Android (Kt)    │
│  macOS           │   │       BLE          │  BLE Peripheral  │
├──────────────────┤   ├──────────────────►  │  + Relay Hub     │
│  Desktop 2 (Go)  │──►┘  Clipboard sync    │                  │
│  Linux           │      Chunked transfer  │  ClipboardService│
├──────────────────┤      Token validation  │  MainActivity    │
│  Desktop N (Go)  │──►                     │  AccessibilitySvc│
│  macOS / Linux   │                        │                  │
└──────────────────┘                        └──────────────────┘

Copy on any desktop → Android relays to all other desktops
Copy on Android    → all desktops receive it
```

## Quick Start

### Prerequisites

**macOS:**
- Go 1.21+
- Bluetooth enabled

**Linux (Ubuntu/Debian):**
- Go 1.21+
- Bluetooth enabled
- System dependencies:
  ```bash
  sudo apt install -y bluez libbluetooth-dev libdbus-1-dev xclip
  sudo systemctl enable --now bluetooth
  ```

**Android:**
- Android 10+ (API 29)
- ADB installed

### 1. Install the desktop service

**macOS:**
```bash
bash run.sh
```

**Linux (Ubuntu/Debian):**
```bash
bash run-ubuntu.sh
```

The Ubuntu script automatically installs all required packages (`bluez`, `libbluetooth-dev`, `libdbus-1-dev`, `xclip`), compiles the binary, and registers a systemd user service.

| OS | Script | Service type | Auto-start |
|---|---|---|---|
| macOS | `run.sh` | LaunchAgent | On login |
| Linux | `run-ubuntu.sh` | systemd user service | On login |

**macOS commands:**
```bash
launchctl unload ~/Library/LaunchAgents/com.clipsync.server.plist  # stop
launchctl load ~/Library/LaunchAgents/com.clipsync.server.plist    # start
tail -f ~/Library/Logs/clipsync-server.out.log                     # logs
```

**Linux commands:**
```bash
systemctl --user stop clipsync      # stop
systemctl --user start clipsync     # start
journalctl --user -u clipsync -f    # logs
```

**Manual build (alternative):**
```bash
cd cmd
go mod tidy
CGO_ENABLED=1 go build -o clipsync-server .
./clipsync-server
```

### 2. Install the Android app

```bash
bash install-android.sh
```

Detects the connected device and installs via ADB. Compiles the APK if needed.

### 3. Pair

1. Open **http://localhost:8066** on your first desktop — the QR code is ready
2. Open **ClipSync** on Android
3. Tap **"Escanear QR"** — scan the QR
4. Done — clipboard syncs automatically

**Multi-desktop:** Repeat for each desktop — open its dashboard, scan its QR from the Android app. Each scan adds a token. The Android relays clipboard between all paired desktops.

## Security Model

- **No cloud**: All data stays local
- **No WiFi**: BLE is direct point-to-point
- **Token-gated**: No clipboard data flows without matching tokens
- **Persistent pairing**: Survives restarts on both sides
- **Unpair**: From the dashboard, sync stops instantly

## BLE Protocol

| UUID | Name | Direction | Purpose |
|---|---|---|---|
| `...def0` | Service | — | ClipSync service identifier |
| `...def1` | Content | Bidirectional | Clipboard text (chunked) |
| `...def2` | Hash | Android → Desktop | CRC32 change notification |
| `...def3` | Pairing | Desktop reads | Security token |

### Chunked Transfer

```
[chunkIndex (1 byte)] [totalChunks (1 byte)] [data (up to 498 bytes)]
```

Max 255 chunks × 498 bytes = ~126KB per transfer.

## Web Dashboard API

| Endpoint | Method | Description |
|---|---|---|
| `/` | GET | Dashboard UI |
| `/api/status` | GET | BLE status + stats |
| `/api/history` | GET | Clipboard history (`?q=search`) |
| `/api/qr` | GET | Get pairing QR token |
| `/api/qr/new` | GET | Force new QR token |
| `/api/pair` | GET | Check pairing status |
| `/api/unpair` | GET | Break the pairing |
| `/api/cleardb` | GET | Delete all history |
| `/api/copy` | POST | Copy text to desktop clipboard |

## Project Structure

```
go-clipsync/
├── run.sh                  # Install service (macOS LaunchAgent)
├── run-ubuntu.sh           # Install service (Linux systemd + apt dependencies)
├── install-android.sh      # Install Android app via ADB
├── cmd/                    # Go server (macOS + Linux)
│   ├── main.go             # Entry point, OS detection, UUIDs
│   ├── ble.go              # BLE Central: scan, connect, sync
│   ├── clipboard.go        # Clipboard read/write (cross-platform)
│   ├── db.go               # SQLite history, stats, search
│   ├── qr.go               # Token generation, pairing persistence
│   ├── web.go              # HTTP dashboard + API (localhost:8066)
│   ├── go.mod
│   └── go.sum
└── client/                 # Android app (Kotlin)
    └── app/src/main/
        ├── java/.../
        │   ├── MainActivity.kt
        │   ├── ClipboardService.kt
        │   ├── ClipAccessibilityService.kt
        │   ├── ShareReceiverActivity.kt
        │   └── SendClipboardActivity.kt
        ├── res/
        └── AndroidManifest.xml
```

## Battery Impact

| | Android | Desktop |
|---|---|---|
| BLE | ~1-2mA (LE advertising) | Negligible |
| Clipboard polling | ~0.5%/hour | ~0.1% CPU |
| RAM | ~25MB | ~35MB |
| **Estimated total** | **~1-2% battery/hour** | **Imperceptible** |

## Roadmap

- [x] macOS LaunchAgent service — `run.sh`
- [x] Linux systemd user service — `run-ubuntu.sh`
- [x] ADB install script — `install-android.sh`
- [x] OS auto-detection (macOS / Linux)
- [x] Multi-desktop sync (N desktops via Android relay)
- [ ] File transfer via HTTP (same WiFi)
- [ ] Auto-start on boot (Android)
- [ ] iOS companion app

## License

MIT

---

# ClipSync — Portapapeles Universal via Bluetooth

> Sincronizá tu portapapeles entre macOS/Linux y Android via Bluetooth Low Energy. Sin nube, sin WiFi.

## Inicio Rápido

### Requisitos

**macOS:** Go 1.21+ y Bluetooth activado.

**Linux (Ubuntu/Debian):**
```bash
sudo apt install -y bluez libbluetooth-dev libdbus-1-dev xclip
sudo systemctl enable --now bluetooth
```

**Android:** Android 10+ y ADB instalado.

### 1. Instalar el servicio en desktop

**macOS:**
```bash
bash run.sh
```

**Linux (Ubuntu/Debian):**
```bash
bash run-ubuntu.sh
```

El script de Ubuntu instala automáticamente todos los paquetes necesarios (`bluez`, `libbluetooth-dev`, `libdbus-1-dev`, `xclip`), compila el binario y crea un servicio systemd.

| OS | Script | Tipo de servicio |
|---|---|---|
| macOS | `run.sh` | LaunchAgent (arranca al login) |
| Linux | `run-ubuntu.sh` | systemd user service (arranca al login) |

**Comandos macOS:**
```bash
launchctl unload ~/Library/LaunchAgents/com.clipsync.server.plist  # detener
launchctl load ~/Library/LaunchAgents/com.clipsync.server.plist    # arrancar
tail -f ~/Library/Logs/clipsync-server.out.log                     # logs
```

**Comandos Linux:**
```bash
systemctl --user stop clipsync      # detener
systemctl --user start clipsync     # arrancar
journalctl --user -u clipsync -f    # logs
```

### 2. Instalar la app Android

```bash
bash install-android.sh
```

Detecta el dispositivo conectado por USB y lo instala via ADB.

### 3. Vincular

1. Abrí **http://localhost:8066** en tu primer desktop — el QR ya está listo
2. Abrí **ClipSync** en tu Android
3. Tocá **"Escanear QR"**
4. ¡Listo! El portapapeles se sincroniza automáticamente

**Multi-desktop:** Repetí para cada desktop — abrí su dashboard, escaneá su QR desde la app Android. Cada escaneo agrega un token. El Android hace relay del clipboard entre todos los desktops vinculados.

### Seguridad

- **Sin nube**: Todos los datos quedan locales
- **Sin WiFi**: BLE es punto a punto directo
- **Token de seguridad**: Sin tokens coincidentes, no pasa ningún dato
- **Pairing persistente**: Vinculás una vez, sobrevive reinicios
- **Desvincular**: Desde el dashboard, el sync se corta al instante

### Sync Automático (Android → Desktop)

1. En el Android: Ajustes → Accesibilidad → ClipSync → Activar
2. Todo lo que copies en el Android aparece automáticamente en el desktop

### Dashboard Web

- Estado de conexión BLE en vivo
- Historial de clipboard con búsqueda
- QR para vincular/revincular
- Botones para desvincular y limpiar historial
