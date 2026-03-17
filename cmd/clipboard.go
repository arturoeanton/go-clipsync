package main

import (
	"fmt"
	"time"

	"github.com/atotto/clipboard"
)

// macClipboardWatcher vigila cambios en el clipboard de macOS
// y notifica via BLE cuando detecta un cambio.
func macClipboardWatcher() {
	fmt.Println("[Clipboard] Watcher de macOS iniciado")

	// Leer contenido inicial
	initialText, err := clipboard.ReadAll()
	if err == nil && initialText != "" {
		clipMu.Lock()
		lastClipContent = initialText
		lastClipHash = clipHash(initialText)
		clipMu.Unlock()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		currentText, err := clipboard.ReadAll()
		if err != nil || currentText == "" {
			continue
		}

		currentHash := clipHash(currentText)

		clipMu.Lock()
		if currentHash != lastClipHash {
			fmt.Printf("[Mac → Android] Cambio detectado (%d chars)\n", len(currentText))
			lastClipContent = currentText
			lastClipHash = currentHash
			clipChanged = true
			clipMu.Unlock()
			saveClip(currentText, "mac", currentHash)
		} else {
			clipMu.Unlock()
		}
	}
}

// setMacClipboard escribe texto al clipboard de macOS.
func setMacClipboard(text string) {
	clipMu.Lock()
	lastClipContent = text
	lastClipHash = clipHash(text)
	clipMu.Unlock()

	if err := clipboard.WriteAll(text); err != nil {
		fmt.Printf("[!] Error escribiendo clipboard Mac: %s\n", err)
	} else {
		fmt.Printf("[+] Clipboard Mac actualizado (%d chars)\n", len(text))
		saveClip(text, "android", clipHash(text))
	}
}
