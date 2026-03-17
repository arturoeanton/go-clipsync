# 📋 ClipSync — Universal Clipboard via Bluetooth

> Sync your clipboard between macOS and Android over Bluetooth Low Energy. No cloud, no WiFi required.

> Sincronizá tu portapapeles entre macOS y Android via Bluetooth Low Energy. Sin nube, sin WiFi.

---

## ✨ Features / Características

| Feature | Description |
|---|---|
| 🔄 **Bidirectional Sync** | Copy on Mac → paste on Android, and vice versa |
| 🔒 **QR Pairing** | Secure token-based pairing via QR code scan |
| 📡 **BLE Only** | Works over Bluetooth LE — no WiFi or internet needed |
| 📦 **Chunked Transfer** | Supports large text (code, articles) up to ~126KB |
| 🌐 **Web Dashboard** | Premium dark UI at `localhost:8066` with history, search, stats |
| 💾 **SQLite History** | Persistent clipboard history with search |
| 🔗 **Persistent Pairing** | Pair once, stays paired across restarts |
| ⚙ **Accessibility Service** | Automatic Android → Mac sync (no manual action) |

## 🏗 Architecture / Arquitectura

```
┌──────────────────┐          BLE           ┌──────────────────┐
│                  │ ◄────────────────────► │                  │
│   macOS (Go)     │   Clipboard sync       │   Android (Kt)   │
│   BLE Central    │   Chunked transfer     │   BLE Peripheral │
│                  │   Token validation     │                  │
│   ├── ble.go     │                        │   ├── ClipboardService.kt
│   ├── clipboard  │                        │   ├── MainActivity.kt
│   ├── db.go      │                        │   ├── ClipAccessibilityService.kt
│   ├── web.go     │                        │   └── ShareReceiverActivity.kt
│   └── qr.go      │                        │
│                  │                        │
│   localhost:8066 │                        │   📷 QR Scanner
│   Dashboard      │                        │   One-button UX
└──────────────────┘                        └──────────────────┘
```

## 📱 Screenshots

### Web Dashboard
- Live BLE connection status
- Clipboard history with search
- QR code for secure pairing
- Unpair & clear database buttons

### Android App
- One-button experience: scan QR → syncing
- Premium dark theme
- Automatic BLE service

## 🚀 Quick Start

### Prerequisites / Requisitos

**macOS:**
- Go 1.21+
- Bluetooth enabled

**Android:**
- Android 10+ (API 29)
- ADB (`brew install android-platform-tools`)

### 1. Install as macOS Service (recommended)

```bash
bash run.sh
```

This will:
- Compile the Go server
- Install the binary to `~/bin/clipsync-server`
- Register a **LaunchAgent** that starts on login and auto-restarts
- Open the dashboard at **http://localhost:8066**

> The server **only listens on localhost** — it's not accessible from the network.

**Useful commands after install:**

```bash
# Stop service
launchctl unload ~/Library/LaunchAgents/com.clipsync.server.plist

# Start service
launchctl load ~/Library/LaunchAgents/com.clipsync.server.plist

# View logs
tail -f ~/Library/Logs/clipsync-server.out.log
```

#### Manual build (alternative)

```bash
cd cmd
go mod tidy
CGO_ENABLED=1 go build -o clipsync-ble .
./clipsync-ble
```

### 2. Install the Android App

```bash
bash install-android.sh
```

This will detect your connected Android device and install the APK via ADB. If the APK hasn't been built yet, it will compile it automatically.

**Manual install (alternative):**

```bash
cd client
ANDROID_HOME=~/Library/Android/sdk ./gradlew assembleDebug
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

### 3. Pair

1. Open **http://localhost:8066** — the QR code is ready
2. Open **ClipSync** on your Android
3. Tap **"📷 Escanear QR"** — scan the QR from the dashboard
4. Done! Clipboard syncs automatically

## 🔒 Security Model

```
1. Mac generates random token → QR code
2. Android scans QR → saves token locally
3. On BLE connect: Mac reads token from Android
4. If tokens match → sync enabled ✅
5. If not → connection rejected ❌
6. Token persists on both sides (survives restarts)
7. "Unpair" on dashboard breaks the link instantly
```

- **No cloud**: All data stays local
- **No WiFi**: BLE is direct point-to-point
- **Token-gated**: No clipboard data flows without matching tokens

## 📡 BLE Protocol

| UUID | Name | Direction | Purpose |
|---|---|---|---|
| `...def0` | Service | — | ClipSync service identifier |
| `...def1` | Content | Bidirectional | Clipboard text (chunked for large text) |
| `...def2` | Hash | Android → Mac | CRC32 change notification |
| `...def3` | Pairing | Mac reads | Security token for authentication |

### Chunked Transfer Protocol

Large text is split into chunks with a 2-byte header:

```
[chunkIndex (1 byte)] [totalChunks (1 byte)] [data (up to 498 bytes)]
```

- Max 255 chunks × 498 bytes = ~126KB per transfer
- 50ms delay between chunks for reliability

## 🌐 Web Dashboard API

| Endpoint | Method | Description |
|---|---|---|
| `/` | GET | Dashboard UI |
| `/api/status` | GET | BLE connection status + stats |
| `/api/history` | GET | Clipboard history (supports `?q=search`) |
| `/api/qr` | GET | Get/reuse pairing QR token |
| `/api/qr/new` | GET | Force generate new QR token |
| `/api/pair` | GET | Check pairing status |
| `/api/unpair` | GET | Break the pairing |
| `/api/cleardb` | GET | Delete all clipboard history |
| `/api/copy` | POST | Copy text to Mac clipboard |

## 🗂 Project Structure

```
go-clipsync/
├── run.sh                  # Install as macOS LaunchAgent service
├── install-android.sh      # Install Android app via ADB
├── cmd/                    # Go server (macOS)
│   ├── main.go             # Entry point, UUIDs, signal handling
│   ├── ble.go              # BLE Central: scan, connect, token validation, sync
│   ├── clipboard.go        # macOS clipboard read/write (pbcopy/pbpaste)
│   ├── db.go               # SQLite history, stats, search
│   ├── qr.go               # Token generation, pairing persistence
│   ├── web.go              # HTTP dashboard + API endpoints (localhost:8066)
│   ├── go.mod
│   └── go.sum
│
└── client/                 # Android app (Kotlin)
    └── app/src/main/
        ├── java/.../
        │   ├── MainActivity.kt              # One-button QR scan UI
        │   ├── ClipboardService.kt           # BLE GATT server + clipboard
        │   ├── ClipAccessibilityService.kt   # Auto-sync via Accessibility
        │   ├── ShareReceiverActivity.kt      # Share intent receiver
        │   └── SendClipboardActivity.kt      # Notification action sender
        ├── res/
        │   ├── xml/accessibility_service_config.xml
        │   └── values/strings.xml
        └── AndroidManifest.xml
```

## 🐧 Ubuntu Support

The Go server is 95% cross-platform. To run on Ubuntu:

```bash
# Install dependencies
sudo apt install libdbus-dev libbluetooth-dev xclip

# Change clipboard commands in clipboard.go:
#   pbcopy  → xclip -selection clipboard
#   pbpaste → xclip -selection clipboard -o

# Build
CGO_ENABLED=1 go build -o clipsync-ble .
```

## ⚡ Battery Impact

| | Android | macOS |
|---|---|---|
| BLE | ~1-2mA (LE advertising) | Negligible |
| Clipboard polling | ~0.5%/hour | ~0.1% CPU |
| RAM | ~25MB | ~35MB |
| **Estimated total** | **~1-2% battery/hour** | **Imperceptible** |

## 📋 Roadmap

- [ ] File transfer via HTTP (same WiFi)
- [ ] Auto-start on boot (Android)
- [x] macOS LaunchAgent (auto-start on login) — `run.sh`
- [x] ADB install script — `install-android.sh`
- [ ] Ubuntu/Linux support
- [ ] Swift helper for Bluetooth Classic file transfer (no WiFi)
- [ ] iOS companion app

## 📝 License

MIT

---

# 📋 ClipSync — Portapapeles Universal via Bluetooth

> Sincronizá tu portapapeles entre macOS y Android via Bluetooth Low Energy. Sin nube, sin WiFi.

## 🚀 Inicio Rápido

### 1. Instalar como servicio macOS (recomendado)

```bash
bash run.sh
```

Compila el servidor, lo instala como **LaunchAgent** (arranca al login, se reinicia solo) y abre el dashboard en **http://localhost:8066**.

> El servidor solo escucha en **localhost** — no es accesible desde la red.

**Comandos útiles:**

```bash
# Detener servicio
launchctl unload ~/Library/LaunchAgents/com.clipsync.server.plist

# Arrancar servicio
launchctl load ~/Library/LaunchAgents/com.clipsync.server.plist

# Ver logs
tail -f ~/Library/Logs/clipsync-server.out.log
```

### 2. Instalar la app Android

```bash
bash install-android.sh
```

Detecta el dispositivo conectado por USB y lo instala via ADB. Si el APK no existe, lo compila automáticamente.

### 3. Vincular

1. Abrí **http://localhost:8066** — el QR ya está listo
2. Abrí **ClipSync** en tu Android
3. Tocá **"📷 Escanear QR"** — escaneá el QR del dashboard
4. ¡Listo! El portapapeles se sincroniza automáticamente

### Seguridad

- **Sin nube**: Todos los datos quedan locales
- **Sin WiFi**: BLE es punto a punto directo
- **Token de seguridad**: Sin tokens coincidentes, no pasa ningún dato
- **Pairing persistente**: Vinculás una vez, sobrevive reinicios
- **Desvincular**: Desde el dashboard, el sync se corta al instante

### Sync Automático (Android → Mac)

Para sincronizar automáticamente sin tocar botones:

1. En el Android: Ajustes → Accesibilidad → ClipSync → Activar
2. Ahora todo lo que copies en el Android aparece en la Mac

### Dashboard Web

- Estado de conexión BLE en vivo
- Historial de clipboard con búsqueda
- QR para vincular/revincular
- Botones para desvincular y limpiar historial
