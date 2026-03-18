package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

type ClipEntry struct {
	ID        int64  `json:"id"`
	Content   string `json:"content"`
	Source    string `json:"source"` // "mac" o "android"
	Hash      uint32 `json:"hash"`
	Timestamp string `json:"timestamp"`
	Length    int    `json:"length"`
}

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", "clipsync.db")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS clipboard_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'mac',
			hash INTEGER NOT NULL,
			length INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_hash ON clipboard_history(hash);
		CREATE INDEX IF NOT EXISTS idx_created ON clipboard_history(created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// Limpiar entradas viejas (mantener últimas 500)
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			cleanOldEntries()
		}
	}()

	fmt.Println("[DB] SQLite inicializado (clipsync.db)")
	return nil
}

func saveClip(content, source string, hash uint32) {
	if db == nil || content == "" {
		return
	}

	// Evitar duplicados consecutivos
	var lastHash int
	err := db.QueryRow("SELECT hash FROM clipboard_history ORDER BY id DESC LIMIT 1").Scan(&lastHash)
	if err == nil && uint32(lastHash) == hash {
		return
	}

	_, err = db.Exec(
		"INSERT INTO clipboard_history (content, source, hash, length) VALUES (?, ?, ?, ?)",
		content, source, hash, len(content),
	)
	if err != nil {
		log.Printf("[DB] Error guardando clip: %s\n", err)
	}
}

func getHistory(limit int) []ClipEntry {
	if db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(
		"SELECT id, content, source, hash, length, created_at FROM clipboard_history ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		log.Printf("[DB] Error leyendo historial: %s\n", err)
		return nil
	}
	defer rows.Close()

	var entries []ClipEntry
	for rows.Next() {
		var e ClipEntry
		if err := rows.Scan(&e.ID, &e.Content, &e.Source, &e.Hash, &e.Length, &e.Timestamp); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func searchHistory(query string, limit int) []ClipEntry {
	if db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(
		"SELECT id, content, source, hash, length, created_at FROM clipboard_history WHERE content LIKE ? ORDER BY id DESC LIMIT ?",
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []ClipEntry
	for rows.Next() {
		var e ClipEntry
		if err := rows.Scan(&e.ID, &e.Content, &e.Source, &e.Hash, &e.Length, &e.Timestamp); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func getStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total":        0,
		"from_mac":     0,
		"from_linux":   0,
		"from_android": 0,
	}
	if db == nil {
		return stats
	}

	var total, fromMac, fromLinux, fromAndroid int
	db.QueryRow("SELECT COUNT(*) FROM clipboard_history").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM clipboard_history WHERE source='mac'").Scan(&fromMac)
	db.QueryRow("SELECT COUNT(*) FROM clipboard_history WHERE source='linux'").Scan(&fromLinux)
	db.QueryRow("SELECT COUNT(*) FROM clipboard_history WHERE source='android'").Scan(&fromAndroid)
	stats["total"] = total
	stats["from_mac"] = fromMac
	stats["from_linux"] = fromLinux
	stats["from_android"] = fromAndroid
	return stats
}

func cleanOldEntries() {
	if db == nil {
		return
	}
	db.Exec("DELETE FROM clipboard_history WHERE id NOT IN (SELECT id FROM clipboard_history ORDER BY id DESC LIMIT 500)")
}
