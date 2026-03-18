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
		fmt.Println("[BLE] >>> Escaneando... (asegurate que el Android tiene la app abierta con servicio activo)")

		ch := make(chan bluetooth.ScanResult, 1)
		deviceCount := 0
		err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			deviceCount++
			name := result.LocalName()
			addr := result.Address.String()
			hasService := result.HasServiceUUID(serviceUUID)

			if name != "" || hasService {
				fmt.Printf("[BLE] #%d Visto: nombre=%q addr=%s RSSI=%d tieneServicio=%v\n",
					deviceCount, name, addr, result.RSSI, hasService)
			} else if deviceCount%20 == 0 {
				fmt.Printf("[BLE] ... %d dispositivos escaneados hasta ahora (sin match)...\n", deviceCount)
			}

			if hasService || name == "ClipSync" {
				fmt.Printf("[BLE] ✅ ¡Match! nombre=%q addr=%s\n", name, addr)
				adapter.StopScan()
				ch <- result
			}
		})
		if err != nil {
			fmt.Printf("[BLE] Error escaneando: %s. Reintentando en 5s...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		result := <-ch

		fmt.Printf("[BLE] Conectando a %s...\n", result.Address.String())
		device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			fmt.Printf("[BLE] Error conectando: %s. Reintentando...\n", err)
			time.Sleep(3 * time.Second)
			continue
		}
		fmt.Println("[BLE] ✅ Conectado")

		err = handleConnection(device, serviceUUID, contentUUID, hashUUID, pairingUUID)
		if err != nil {
			fmt.Printf("[BLE] Conexión perdida: %s. Reconectando...\n", err)
			device.Disconnect()
			time.Sleep(2 * time.Second)
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

	// Suscribirse a notificaciones del contenido (Android → Desktop)
	err = contentChar.EnableNotifications(func(buf []byte) {
		if !isPaired() {
			return
		}
		text := string(buf)
		if text != "" {
			fmt.Printf("[Android → %s] Recibido via notificación (%d chars)\n", osName, len(text))
			clipMu.Lock()
			lastClipContent = text
			lastClipHash = clipHash(text)
			fromAndroid = true
			clipMu.Unlock()

			if err := clipboard.WriteAll(text); err != nil {
				fmt.Printf("[!] Error escribiendo clipboard %s: %s\n", osName, err)
			} else {
				fmt.Printf("[+] Clipboard %s actualizado (%d chars)\n", osName, len(text))
				saveClip(text, "android", clipHash(text))
			}
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
