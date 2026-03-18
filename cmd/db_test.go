package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	origDB := db
	t.Cleanup(func() { db = origDB })

	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

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
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func TestInitDB(t *testing.T) {
	testDB(t) // just verify it doesn't crash
}

func TestSaveAndGetHistory(t *testing.T) {
	testDB(t)

	saveClip("hello world", "mac", clipHash("hello world"))
	history := getHistory(10)

	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}
	if history[0].Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", history[0].Content)
	}
	if history[0].Source != "mac" {
		t.Errorf("expected source 'mac', got %q", history[0].Source)
	}
}

func TestDuplicateSuppression(t *testing.T) {
	testDB(t)

	hash := clipHash("duplicate text")
	saveClip("duplicate text", "mac", hash)
	saveClip("duplicate text", "mac", hash) // same hash — should be suppressed

	history := getHistory(10)
	if len(history) != 1 {
		t.Errorf("expected 1 entry (dedup), got %d", len(history))
	}
}

func TestSearchHistory(t *testing.T) {
	testDB(t)

	saveClip("hello world", "mac", clipHash("hello world"))
	saveClip("goodbye world", "android", clipHash("goodbye world"))
	saveClip("foo bar", "linux", clipHash("foo bar"))

	results := searchHistory("world", 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'world', got %d", len(results))
	}

	results = searchHistory("foo", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'foo', got %d", len(results))
	}
}

func TestGetStats(t *testing.T) {
	testDB(t)

	saveClip("mac clip", "mac", clipHash("mac clip"))
	saveClip("android clip", "android", clipHash("android clip"))
	saveClip("linux clip", "linux", clipHash("linux clip"))

	stats := getStats()
	if stats["total"] != 3 {
		t.Errorf("expected total 3, got %v", stats["total"])
	}
	if stats["from_mac"] != 1 {
		t.Errorf("expected from_mac 1, got %v", stats["from_mac"])
	}
	if stats["from_android"] != 1 {
		t.Errorf("expected from_android 1, got %v", stats["from_android"])
	}
}
