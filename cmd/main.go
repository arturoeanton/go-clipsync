package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"runtime"
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
	fromAndroid     bool   // suppress echo back to Android
	pendingClip     string // clipboard text pending write from BLE callback

	// OS detection
	osName   string // "macOS" or "Linux" (display name)
	osSource string // "mac" or "linux" (DB source identifier)
)

func init() {
	switch runtime.GOOS {
	case "darwin":
		osName = "macOS"
		osSource = "mac"
	case "linux":
		osName = "Linux"
		osSource = "linux"
	default:
		osName = runtime.GOOS
		osSource = runtime.GOOS
	}
}

func clipHash(text string) uint32 {
	return crc32.ChecksumIEEE([]byte(text))
}

func main() {
	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Printf("║   📋 ClipSync — Universal Clipboard       ║\n")
	fmt.Printf("║   %s ↔ Android via Bluetooth LE        ║\n", osName)
	fmt.Println("╚═══════════════════════════════════════════╝")

	// Inicializar SQLite
	if err := initDB(); err != nil {
		fmt.Printf("[!] Error inicializando DB: %s\n", err)
	}
	initPairing()

	// Iniciar Web UI
	startWebServer()

	// Iniciar watcher del clipboard
	go clipboardWatcher()

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
		fmt.Printf("[!] ¿Bluetooth está encendido en %s?\n", osName)
	}
}
