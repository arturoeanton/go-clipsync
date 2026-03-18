# ClipSync — Estado del Proyecto

> Última actualización: 2026-03-18

## Resumen

Sincronización universal de portapapeles entre desktops macOS/Linux y Android via Bluetooth Low Energy. El Android actúa como hub central de relay para N desktops.

## Estado Actual: **Beta — Funcional**

| Componente | Estado | Notas |
|---|---|---|
| macOS Desktop | ✅ Funciona | BLE Central, polling de clipboard, auto-reconexión |
| Linux Desktop | ✅ Funciona | Mismo codebase, servicio systemd |
| App Android | ✅ Funciona | BLE Peripheral, GATT server, UI flat |
| Multi-Desktop | ✅ Funciona | N desktops via relay Android |
| Dashboard Web | ✅ Funciona | Historial, búsqueda, QR pairing, estadísticas |
| BLE Chunking | ✅ Funciona | Desktop → Android hasta ~126KB |
| Anti-Echo Loop | ✅ Funciona | Prevención de doble capa en ambos lados |
| Gestión de Tokens | ✅ Funciona | Botón "Borrar vinculaciones" en la app |

## Arquitectura

```
Desktop(s) ←── BLE ──→ Android (relay hub) ←── BLE ──→ Desktop(s)
   Go server              Kotlin service              Go server
   BLE Central            BLE Peripheral              BLE Central
   localhost:8066          GATT Server                 localhost:8066
```

**Flujo de datos:**
- Desktop copia texto → BLE chunked write → Android recibe → relay a otros desktops via notificación
- Android copia texto → BLE notificación → todos los desktops reciben y actualizan clipboard

## Sistema Anti-Echo Loop

Previene el rebote infinito de clipboard entre dispositivos:

| Capa | Lado | Mecanismo |
|---|---|---|
| Flag `fromAndroid` | Desktop (Go) | clipboardWatcher ignora cambios que vinieron de Android |
| `pendingClip` | Desktop (Go) | Escritura al clipboard fuera del callback BLE |
| Flag `fromDesktop` | Android | Clipboard poller ignora cambios que vinieron de desktops |
| Hash check en `sendToMac` | Android | AccessibilityService no reenvía si el hash ya coincide |
| `@Synchronized` | Android | `notifyDesktops()` previene condiciones de carrera |

## Sistema de Tokens Multi-Desktop

- Cada desktop genera su propio token de pairing (via QR)
- El usuario escanea cada QR desde la app Android
- Android almacena los tokens como set (separados por `|`) en SharedPreferences
- El desktop lee todos los tokens y valida que el suyo esté presente
- Botón "Borrar vinculaciones" limpia todos los tokens

## Protocolo BLE

| UUID sufijo | Nombre | Dirección | Propósito |
|---|---|---|---|
| `def0` | Service | — | Identificador del servicio ClipSync |
| `def1` | Content | Bidireccional | Texto del clipboard (write con chunks, lectura via notificación) |
| `def2` | Hash | Android → Desktop | Notificación de cambio CRC32 |
| `def3` | Pairing | Desktop lee | Validación de token de seguridad |

**Propiedades de la characteristic Content:** `READ | WRITE | WRITE_NO_RESPONSE | NOTIFY`

## Limitaciones Conocidas

| Issue | Impacto | Workaround |
|---|---|---|
| Notificación limitada a 512 bytes | Texto Android → Desktop truncado a 512 chars | Desktop → Android usa chunking (sin límite) |
| Rango BLE ~10m | Sync solo funciona cerca | Esperado para BLE |
| Polling de clipboard Android 750ms | Pequeño delay en detección | No se puede evitar (restricción del OS) |

## Estructura de Archivos

| Archivo | Propósito |
|---|---|
| `cmd/main.go` | Entry point, estado global, detección de OS |
| `cmd/ble.go` | BLE Central: scan, connect, sync, anti-echo |
| `cmd/clipboard.go` | Lectura/escritura clipboard, watcher con guard anti-echo |
| `cmd/db.go` | Historial SQLite, estadísticas, búsqueda |
| `cmd/qr.go` | Generación de token, pairing persistente |
| `cmd/web.go` | Dashboard HTTP + API (localhost:8066) |
| `ClipboardService.kt` | BLE Peripheral, GATT server, relay hub, anti-echo |
| `MainActivity.kt` | UI, escáner QR, gestión de tokens |
| `ClipAccessibilityService.kt` | Detección de clipboard en background |
| `ShareReceiverActivity.kt` | Receptor de intent Share de Android |

## Cambios Recientes (2026-03-18)

- ✅ Sync multi-desktop (N desktops via relay Android)
- ✅ Propiedad `WRITE_NO_RESPONSE` en la characteristic de contenido
- ✅ Nuevo API `notifyCharacteristicChanged` (Android 13+ con valor explícito)
- ✅ Sistema anti-echo loop (4 capas)
- ✅ UI de gestión de tokens ("Borrar vinculaciones")
- ✅ Mecanismo `pendingClip` (escritura al clipboard fuera del callback BLE)
- ✅ Estabilidad BLE — timeout 15s, solo logs de matches ClipSync
- ✅ Notificaciones chunked Android → Desktop (sin límite de 512 bytes)
- ✅ Reconexión rápida (500ms vs 2s)
- ✅ UX premium Android — historial de sync con auto-refresh
- ✅ 19 tests unitarios Go (hash, DB, QR, clipboard anti-echo)
- ✅ README bilingüe actualizado

## Roadmap

- [x] Servicio macOS LaunchAgent
- [x] Servicio systemd para Linux
- [x] Sync multi-desktop
- [x] Prevención de echo loop
- [x] UI de gestión de tokens
- [x] Estabilidad de scan BLE
- [x] Notificaciones chunked Android → Desktop
- [x] Reconexión rápida
- [x] UX historial de sync
- [x] Tests unitarios (19 tests Go)
- [ ] Transferencia de archivos via HTTP (misma WiFi)
- [ ] Inicio automático en boot (Android)
- [ ] App companion iOS
