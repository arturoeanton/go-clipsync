package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// UUIDs del servicio y características BLE
const (
	ClipServiceUUID = "12345678-1234-5678-1234-56789abcdef0"
	ClipContentUUID = "12345678-1234-5678-1234-56789abcdef1"
	ClipHashUUID    = "12345678-1234-5678-1234-56789abcdef2"
	ClipPairingUUID = "12345678-1234-5678-1234-56789abcdef3"
)

var (
	clipMu          sync.Mutex
	lastClipContent string
	lastClipHash    uint32
	bleReady        bool
	clipChanged     bool
)

func clipHash(text string) uint32 {
	return crc32.ChecksumIEEE([]byte(text))
}

func main() {
	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║   📋 ClipSync — Universal Clipboard       ║")
	fmt.Println("║   Mac ↔ Android via Bluetooth LE          ║")
	fmt.Println("╚═══════════════════════════════════════════╝")

	// Inicializar SQLite
	if err := initDB(); err != nil {
		fmt.Printf("[!] Error inicializando DB: %s\n", err)
	}
	initPairing()

	// Iniciar Web UI
	startWebServer()

	// Iniciar watcher del clipboard macOS
	go macClipboardWatcher()

	// Graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\n[*] Cerrando ClipSync...")
		if db != nil {
			db.Close()
		}
		os.Exit(0)
	}()

	// Iniciar BLE Central
	if err := startBLECentral(); err != nil {
		fmt.Printf("[!] Error BLE: %s\n", err)
		fmt.Println("[!] ¿Bluetooth está encendido en la Mac?")
	}
}
