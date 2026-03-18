# ClipSync — Project Status

> Last updated: 2026-03-18

## Overview

Universal clipboard sync between macOS/Linux desktops and Android via Bluetooth Low Energy. Android acts as a central relay hub for N desktops.

## Current Status: **Beta — Functional**

| Component | Status | Notes |
|---|---|---|
| macOS Desktop | ✅ Working | BLE Central, clipboard polling, auto-reconnect |
| Linux Desktop | ✅ Working | Same codebase, systemd service |
| Android App | ✅ Working | BLE Peripheral, GATT server, flat UI |
| Multi-Desktop | ✅ Working | N desktops via shared Android relay |
| Web Dashboard | ✅ Working | History, search, QR pairing, stats |
| BLE Chunking | ✅ Working | Desktop → Android up to ~126KB |
| Anti-Echo Loop | ✅ Working | Dual-layer prevention on both sides |
| Token Management | ✅ Working | Android UI "Borrar vinculaciones" button |

## Architecture

```
Desktop(s) ←── BLE ──→ Android (relay hub) ←── BLE ──→ Desktop(s)
   Go server              Kotlin service              Go server
   BLE Central            BLE Peripheral              BLE Central
   localhost:8066          GATT Server                 localhost:8066
```

**Data flow:**
- Desktop copies text → BLE chunked write → Android receives → relays to other desktops via notification
- Android copies text → BLE notification → all desktops receive and set clipboard

## Anti-Echo Loop System

Prevents infinite clipboard bounce between devices:

| Layer | Side | Mechanism |
|---|---|---|
| `fromAndroid` flag | Desktop (Go) | clipboardWatcher skips changes that came from Android |
| `pendingClip` | Desktop (Go) | Clipboard write happens in polling loop, not BLE callback |
| `fromDesktop` flag | Android | Clipboard poller skips changes that came from desktops |
| `sendToMac` hash check | Android | Accessibility Service skips if hash already matches |
| `@Synchronized` | Android | `notifyDesktops()` prevents race conditions |

## Multi-Desktop Token System

- Each desktop generates its own pairing token (via QR)
- User scans each QR from the Android app
- Android stores tokens as pipe-separated set in SharedPreferences
- Desktop reads all tokens and validates its own is present
- "Borrar vinculaciones" button clears all tokens

## BLE Protocol

| UUID suffix | Name | Direction | Purpose |
|---|---|---|---|
| `def0` | Service | — | ClipSync service identifier |
| `def1` | Content | Bidirectional | Clipboard text (chunked write, notification read) |
| `def2` | Hash | Android → Desktop | CRC32 change notification |
| `def3` | Pairing | Desktop reads | Security token validation |

**Content characteristic properties:** `READ | WRITE | WRITE_NO_RESPONSE | NOTIFY`

## Known Limitations

| Issue | Impact | Workaround |
|---|---|---|
| Notification content limit 512 bytes | Android → Desktop text truncated at 512 chars | Desktop → Android uses chunking (no limit) |
| BLE range ~10m | Sync only works nearby | Expected for BLE |
| Android clipboard polling 750ms | Slight delay in detection | Can't be avoided (OS restriction) |

## File Structure

| File | Purpose |
|---|---|
| `cmd/main.go` | Entry point, global state, OS detection |
| `cmd/ble.go` | BLE Central: scan, connect, sync, anti-echo |
| `cmd/clipboard.go` | Clipboard read/write, watcher with echo guard |
| `cmd/db.go` | SQLite history, stats, search |
| `cmd/qr.go` | Token generation, persistent pairing |
| `cmd/web.go` | HTTP dashboard + API (localhost:8066) |
| `ClipboardService.kt` | BLE Peripheral, GATT server, relay hub, anti-echo |
| `MainActivity.kt` | UI, QR scanner, token management |
| `ClipAccessibilityService.kt` | Background clipboard detection |
| `ShareReceiverActivity.kt` | Android Share intent receiver |

## Recent Changes (2026-03-18)

- ✅ Multi-desktop sync (N desktops via Android relay)
- ✅ `WRITE_NO_RESPONSE` property on content characteristic
- ✅ New `notifyCharacteristicChanged` API (Android 13+ explicit value)
- ✅ Anti-echo loop system (4 layers)
- ✅ Token management UI ("Borrar vinculaciones")
- ✅ `pendingClip` mechanism (clipboard write outside BLE callback)
- ✅ BLE scan stability — 15s timeout, only ClipSync matches logged
- ✅ Chunked Android → Desktop notifications (no 512 byte limit)
- ✅ Fast reconnect (500ms vs 2s)
- ✅ Premium Android UX — sync history with auto-refresh
- ✅ 19 Go unit tests (hash, DB, QR, clipboard anti-echo)
- ✅ README bilingual update

## Roadmap

- [x] macOS LaunchAgent service
- [x] Linux systemd user service
- [x] Multi-desktop sync
- [x] Anti-echo loop prevention
- [x] Token management UI
- [x] BLE scan stability
- [x] Chunked Android → Desktop notifications
- [x] Fast reconnect
- [x] Sync history UX
- [x] Unit tests (19 Go tests)
- [ ] File transfer via HTTP (same WiFi)
- [ ] Auto-start on boot (Android)
- [ ] iOS companion app
