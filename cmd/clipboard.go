package main

import (
	"fmt"
	"time"

	"github.com/atotto/clipboard"
)

// clipboardWatcher vigila cambios en el clipboard del sistema
// y notifica via BLE cuando detecta un cambio.
func clipboardWatcher() {
	fmt.Printf("[Clipboard] Watcher de %s iniciado\n", osName)

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
		// Skip if this change came from Android (prevent echo)
		if fromAndroid && currentHash == lastClipHash {
			fromAndroid = false
			clipMu.Unlock()
			continue
		}
		fromAndroid = false

		if currentHash != lastClipHash {
			fmt.Printf("[%s → Android] Cambio detectado (%d chars)\n", osName, len(currentText))
			lastClipContent = currentText
			lastClipHash = currentHash
			clipChanged = true
			clipMu.Unlock()
			saveClip(currentText, osSource, currentHash)
		} else {
			clipMu.Unlock()
		}
	}
}

// setClipboard escribe texto al clipboard del sistema.
func setClipboard(text string) {
	clipMu.Lock()
	lastClipContent = text
	lastClipHash = clipHash(text)
	clipMu.Unlock()

	if err := clipboard.WriteAll(text); err != nil {
		fmt.Printf("[!] Error escribiendo clipboard %s: %s\n", osName, err)
	} else {
		fmt.Printf("[+] Clipboard %s actualizado (%d chars)\n", osName, len(text))
		saveClip(text, "android", clipHash(text))
	}
}
