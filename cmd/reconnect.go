package main

import (
	"fmt"
	"runtime"
	"sync"
)

var (
	cachedAddrMu sync.Mutex
	cachedAddr   string // Last known Android BLE address
)

// cacheDeviceAddress stores the last successful Android address.
func cacheDeviceAddress(addr string) {
	cachedAddrMu.Lock()
	cachedAddr = addr
	cachedAddrMu.Unlock()

	if db != nil {
		db.Exec("CREATE TABLE IF NOT EXISTS ble_cache (id INTEGER PRIMARY KEY, address TEXT NOT NULL)")
		db.Exec("DELETE FROM ble_cache")
		db.Exec("INSERT INTO ble_cache (address) VALUES (?)", addr)
	}
	fmt.Printf("[BLE] Dirección cacheada: %s\n", addr)
}

// getCachedDeviceAddress retrieves the cached Android address.
func getCachedDeviceAddress() string {
	cachedAddrMu.Lock()
	addr := cachedAddr
	cachedAddrMu.Unlock()

	if addr != "" {
		return addr
	}

	if db != nil {
		db.Exec("CREATE TABLE IF NOT EXISTS ble_cache (id INTEGER PRIMARY KEY, address TEXT NOT NULL)")
		var dbAddr string
		err := db.QueryRow("SELECT address FROM ble_cache LIMIT 1").Scan(&dbAddr)
		if err == nil && dbAddr != "" {
			cachedAddrMu.Lock()
			cachedAddr = dbAddr
			cachedAddrMu.Unlock()
			return dbAddr
		}
	}
	return ""
}

// canDirectConnect returns true if the OS supports direct BLE connect by address.
// macOS CoreBluetooth uses random UUIDs that change, so direct connect is unreliable.
func canDirectConnect() bool {
	return runtime.GOOS == "linux"
}
