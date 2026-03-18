package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

// startBLECentral escanea, conecta y sincroniza con el Android BLE Peripheral.
func startBLECentral() error {
	fmt.Println("[BLE] Habilitando adaptador Bluetooth...")
	if err := adapter.Enable(); err != nil {
		return fmt.Errorf("no se pudo habilitar Bluetooth: %w", err)
	}

	serviceUUID, _ := bluetooth.ParseUUID(ClipServiceUUID)
	contentUUID, _ := bluetooth.ParseUUID(ClipContentUUID)
	hashUUID, _ := bluetooth.ParseUUID(ClipHashUUID)
	pairingUUID, _ := bluetooth.ParseUUID(ClipPairingUUID)

	for {
		var device bluetooth.Device
		var connected bool

		// === Scan con timeout ===
		fmt.Println("[BLE] >>> Escaneando... (asegurate que el Android tiene la app abierta con servicio activo)")

		ch := make(chan bluetooth.ScanResult, 1)
		scanDone := make(chan struct{})

		go func() {
			err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
				hasService := result.HasServiceUUID(serviceUUID)

				// Solo loguear dispositivos con el servicio ClipSync
				if hasService {
					name := result.LocalName()
					fmt.Printf("[BLE] ✅ ¡Match! nombre=%q addr=%s RSSI=%d\n", name, result.Address.String(), result.RSSI)
					adapter.StopScan()
					ch <- result
				}
			})
			if err != nil {
				fmt.Printf("[BLE] Error escaneando: %s\n", err)
			}
			close(scanDone)
		}()

		// Timeout de 15 segundos
		select {
		case result := <-ch:
			fmt.Printf("[BLE] Conectando a %s...\n", result.Address.String())
			dev, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
			if err != nil {
				fmt.Printf("[BLE] Error conectando: %s. Reintentando...\n", err)
				time.Sleep(1 * time.Second)
				continue
			}
			device = dev
			connected = true
		case <-time.After(15 * time.Second):
			fmt.Println("[BLE] Scan timeout (15s). Reintentando...")
			adapter.StopScan()
			<-scanDone
			continue
		}

		if !connected {
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Println("[BLE] ✅ Conectado")

		// Cachear dirección para reconexión rápida
		cacheDeviceAddress(device.Address.String())

		err := handleConnection(device, serviceUUID, contentUUID, hashUUID, pairingUUID)
		if err != nil {
			fmt.Printf("[BLE] Conexión perdida: %s. Reconectando...\n", err)
			device.Disconnect()
			time.Sleep(500 * time.Millisecond) // Reconexión rápida
			continue
		}
	}
}

// handleConnection maneja la comunicación BLE con el Android una vez conectado.
func handleConnection(device bluetooth.Device, serviceUUID, contentUUID, hashUUID, pairingUUID bluetooth.UUID) error {
	fmt.Println("[BLE] Descubriendo servicios...")
	srvcs, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil {
		return fmt.Errorf("discover services: %w", err)
	}

	if len(srvcs) == 0 {
		return fmt.Errorf("servicio ClipSync no encontrado")
	}

	srvc := srvcs[0]
	fmt.Println("[BLE] Servicio ClipSync encontrado")

	// Descubrir características (incluyendo pairing)
	chars, err := srvc.DiscoverCharacteristics([]bluetooth.UUID{contentUUID, hashUUID, pairingUUID})
	if err != nil {
		return fmt.Errorf("discover characteristics: %w", err)
	}

	var contentChar, hashChar, pairingChar bluetooth.DeviceCharacteristic
	hasPairingChar := false
	for _, c := range chars {
		switch c.UUID().String() {
		case contentUUID.String():
			contentChar = c
			fmt.Println("[BLE] Característica clipboard_content encontrada")
		case hashUUID.String():
			hashChar = c
			fmt.Println("[BLE] Característica clipboard_hash encontrada")
		case pairingUUID.String():
			pairingChar = c
			hasPairingChar = true
			fmt.Println("[BLE] Característica pairing_token encontrada")
		}
	}

	// === VALIDACIÓN DE PAIRING ===
	expectedToken := getPendingOrPairedToken()

	if expectedToken == "" {
		fmt.Println("[BLE] ⛔ No hay token. Generá un QR en http://localhost:8066 primero.")
		return fmt.Errorf("no pairing token — generate QR first")
	}

	if !hasPairingChar {
		fmt.Println("[BLE] ⛔ Dispositivo no tiene característica de pairing.")
		return fmt.Errorf("device has no pairing characteristic")
	}

	buf := make([]byte, 512)
	n, err := pairingChar.Read(buf)
	if err != nil {
		fmt.Printf("[BLE] ⛔ Error leyendo token: %s\n", err)
		return fmt.Errorf("cannot read pairing token: %w", err)
	}

	remoteTokens := string(buf[:n])
	if remoteTokens == "" {
		fmt.Println("[BLE] ⛔ Android sin token — debe escanear el QR primero.")
		return fmt.Errorf("android has no pairing token")
	}

	// Multi-token: Android returns pipe-separated tokens
	tokenFound := false
	for _, t := range strings.Split(remoteTokens, "|") {
		if t == expectedToken {
			tokenFound = true
			break
		}
	}

	if !tokenFound {
		fmt.Println("[BLE] ❌ Token NO coincide. Rechazando.")
		fmt.Println("[BLE] El Android debe escanear el QR de este desktop.")
		return fmt.Errorf("pairing token mismatch")
	}

	// Si era pendiente, confirmar el pairing
	if !isPaired() {
		confirmPairing(expectedToken)
	}
	fmt.Println("[BLE] ✅ Token verificado — sync autorizado")

	// Chunk buffer for reassembling multi-chunk notifications from Android
	notifyChunks := make(map[int][]byte)
	notifyExpectedChunks := 0

	// Suscribirse a notificaciones del contenido (Android → Desktop)
	err = contentChar.EnableNotifications(func(buf []byte) {
		if !isPaired() {
			return
		}

		// Detect chunked header: [index, total, data...]
		if len(buf) >= 2 {
			chunkIndex := int(buf[0])
			totalChunks := int(buf[1])

			if totalChunks > 0 && totalChunks <= 255 && chunkIndex < totalChunks {
				data := buf[2:]

				if totalChunks == 1 {
					// Single chunk — process directly
					text := string(data)
					if text != "" {
						fmt.Printf("[Android → %s] Recibido (%d chars)\n", osName, len(text))
						clipMu.Lock()
						pendingClip = text
						clipMu.Unlock()
					}
					return
				}

				// Multi-chunk: buffer and reassemble
				notifyChunks[chunkIndex] = make([]byte, len(data))
				copy(notifyChunks[chunkIndex], data)
				notifyExpectedChunks = totalChunks
				fmt.Printf("[Android → %s] Chunk %d/%d recibido (%d bytes)\n", osName, chunkIndex+1, totalChunks, len(data))

				if len(notifyChunks) >= notifyExpectedChunks {
					// All chunks received — reassemble
					totalSize := 0
					for _, chunk := range notifyChunks {
						totalSize += len(chunk)
					}
					full := make([]byte, 0, totalSize)
					for i := 0; i < notifyExpectedChunks; i++ {
						if chunk, ok := notifyChunks[i]; ok {
							full = append(full, chunk...)
						}
					}
					// Clear buffer
					for k := range notifyChunks {
						delete(notifyChunks, k)
					}
					notifyExpectedChunks = 0

					text := string(full)
					fmt.Printf("[Android → %s] Recibido completo (%d chars, %d chunks)\n", osName, len(text), totalChunks)
					clipMu.Lock()
					pendingClip = text
					clipMu.Unlock()
				}
				return
			}
		}

		// Fallback: raw text (no chunking header)
		text := string(buf)
		if text != "" {
			fmt.Printf("[Android → %s] Recibido raw (%d chars)\n", osName, len(text))
			clipMu.Lock()
			pendingClip = text
			clipMu.Unlock()
		}
	})
	if err != nil {
		fmt.Printf("[BLE] Warn: no se pudo habilitar content notifications: %s\n", err)
	}

	// Suscribirse a notificaciones del hash (backup si content notification no lleva datos)
	err = hashChar.EnableNotifications(func(buf []byte) {
		if !isPaired() {
			return
		}
		// Hash notifications ya no necesitan hacer GATT read
		// El contenido llega via content notification
	})
	if err != nil {
		return fmt.Errorf("enable hash notifications: %w", err)
	}

	fmt.Println("[BLE] 🔄 Sincronización bidireccional activa")
	bleReady = true

	// Forzar envío del clipboard actual de Mac al Android inmediatamente
	clipMu.Lock()
	if lastClipContent != "" {
		clipChanged = true
		fmt.Printf("[BLE] Clipboard %s actual: %d chars — enviando...\n", osName, len(lastClipContent))
	}
	clipMu.Unlock()

	// Loop de polling: enviar clipboard de Mac al Android
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	pollCount := 0
	for range ticker.C {
		pollCount++

		// Verificar pairing activo
		if !isPaired() {
			fmt.Println("[BLE] ⛔ Desvinculado — cortando conexión BLE")
			bleReady = false
			return fmt.Errorf("unpaired — disconnecting")
		}

		// Procesar clipboard pendiente del Android (fuera del callback BLE)
		clipMu.Lock()
		pending := pendingClip
		pendingClip = ""
		clipMu.Unlock()

		if pending != "" {
			if err := clipboard.WriteAll(pending); err != nil {
				fmt.Printf("[!] Error escribiendo clipboard %s: %s\n", osName, err)
			} else {
				fmt.Printf("[+] Clipboard %s actualizado (%d chars)\n", osName, len(pending))
				saveClip(pending, "android", clipHash(pending))
			}
			// Actualizar estado DESPUÉS de escribir al clipboard
			clipMu.Lock()
			lastClipContent = pending
			lastClipHash = clipHash(pending)
			fromAndroid = true
			clipChanged = false
			clipMu.Unlock()
		}

		clipMu.Lock()
		content := lastClipContent
		hash := lastClipHash
		changed := clipChanged
		clipMu.Unlock()

		if pollCount%30 == 0 {
			fmt.Printf("[BLE] Polling #%d — contenido=%d chars, hash=%08x, cambió=%v\n",
				pollCount, len(content), hash, changed)
		}

		if content == "" || !changed {
			continue
		}

		// Escribir al Android (con chunking)
		clipMu.Lock()
		clipChanged = false
		clipMu.Unlock()

		contentBytes := []byte(content)
		chunkSize := 498 // 500 - 2 bytes header
		totalChunks := (len(contentBytes) + chunkSize - 1) / chunkSize
		if totalChunks > 255 {
			totalChunks = 255
			contentBytes = contentBytes[:255*chunkSize]
		}

		fmt.Printf("[BLE] Enviando %d bytes en %d chunk(s)...\n", len(contentBytes), totalChunks)

		for i := 0; i < totalChunks; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if end > len(contentBytes) {
				end = len(contentBytes)
			}

			// Header: [chunkIndex, totalChunks] + data
			chunk := make([]byte, 2+end-start)
			chunk[0] = byte(i)
			chunk[1] = byte(totalChunks)
			copy(chunk[2:], contentBytes[start:end])

			_, err := contentChar.WriteWithoutResponse(chunk)
			if err != nil {
				fmt.Printf("[BLE] Error escribiendo chunk %d/%d: %s\n", i+1, totalChunks, err)
				return fmt.Errorf("write chunk: %w", err)
			}

			if totalChunks > 1 {
				time.Sleep(50 * time.Millisecond)
			}
		}
		fmt.Printf("[%s → Android] Enviado (%d bytes, %d chunks)\n", osName, len(contentBytes), totalChunks)
	}

	return nil
}
