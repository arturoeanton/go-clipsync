package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

var (
	pairingMu     sync.Mutex
	pendingToken  string // Token generado por QR, esperando confirmación
	pairedDevice  string // Token confirmado por BLE — solo este habilita sync
)

// initPairing carga el pairing persistente desde SQLite.
func initPairing() {
	if db == nil {
		return
	}

	db.Exec(`CREATE TABLE IF NOT EXISTS pairing (
		id INTEGER PRIMARY KEY,
		device_addr TEXT NOT NULL,
		paired_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	var addr string
	err := db.QueryRow("SELECT device_addr FROM pairing ORDER BY id DESC LIMIT 1").Scan(&addr)
	if err == nil && addr != "" {
		pairingMu.Lock()
		pairedDevice = addr
		pairingMu.Unlock()
		fmt.Printf("[QR] Pairing restaurado: %s...\n", addr[:min(8, len(addr))])
	}
}

// generatePairingToken retorna el token pendiente si existe, o genera uno nuevo.
func generatePairingToken(forceNew bool) string {
	pairingMu.Lock()
	defer pairingMu.Unlock()

	// Reusar token pendiente existente (evita invalidar al refrescar la página)
	if !forceNew && pendingToken != "" {
		return pendingToken
	}

	b := make([]byte, 16)
	rand.Read(b)
	pendingToken = hex.EncodeToString(b)
	fmt.Printf("[QR] Token pendiente generado: %s...\n", pendingToken[:8])
	return pendingToken
}

// confirmPairing marca el pendingToken como confirmado (llamado desde BLE).
func confirmPairing(token string) {
	pairingMu.Lock()
	pairedDevice = token
	pendingToken = ""
	pairingMu.Unlock()

	if db != nil {
		db.Exec("DELETE FROM pairing")
		db.Exec("INSERT INTO pairing (device_addr) VALUES (?)", token)
	}
	fmt.Println("[QR] ✅ Pairing confirmado por BLE")
}

// unpair rompe el enlace.
func unpair() {
	pairingMu.Lock()
	pairedDevice = ""
	pendingToken = ""
	pairingMu.Unlock()

	if db != nil {
		db.Exec("DELETE FROM pairing")
	}
	fmt.Println("[QR] ❌ Dispositivo desvinculado")
}

// isPaired retorna si hay un dispositivo confirmado.
func isPaired() bool {
	pairingMu.Lock()
	defer pairingMu.Unlock()
	return pairedDevice != ""
}

// getPendingOrPairedToken retorna el token que el BLE debe validar.
func getPendingOrPairedToken() string {
	pairingMu.Lock()
	defer pairingMu.Unlock()
	if pairedDevice != "" {
		return pairedDevice
	}
	return pendingToken
}
