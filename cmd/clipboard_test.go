package main

import (
	"testing"
)

func TestFromAndroidGuard(t *testing.T) {
	clipMu.Lock()
	lastClipHash = clipHash("from android")
	fromAndroid = true
	clipMu.Unlock()

	// Simulate what clipboardWatcher does
	currentHash := clipHash("from android")

	clipMu.Lock()
	shouldSkip := fromAndroid && currentHash == lastClipHash
	if shouldSkip {
		fromAndroid = false
	}
	clipMu.Unlock()

	if !shouldSkip {
		t.Error("fromAndroid guard should skip echoing text back")
	}
}

func TestFromAndroidGuardDifferentHash(t *testing.T) {
	clipMu.Lock()
	lastClipHash = clipHash("from android")
	fromAndroid = true
	clipMu.Unlock()

	// Simulate a different clipboard change (not from android)
	currentHash := clipHash("different text")

	clipMu.Lock()
	shouldSkip := fromAndroid && currentHash == lastClipHash
	clipMu.Unlock()

	if shouldSkip {
		t.Error("should NOT skip when hashes differ")
	}
}

func TestClipChangedDetection(t *testing.T) {
	clipMu.Lock()
	lastClipHash = clipHash("old text")
	clipChanged = false
	fromAndroid = false
	clipMu.Unlock()

	newHash := clipHash("new text")
	clipMu.Lock()
	if newHash != lastClipHash {
		clipChanged = true
	}
	clipMu.Unlock()

	clipMu.Lock()
	changed := clipChanged
	clipMu.Unlock()

	if !changed {
		t.Error("clipChanged should be true after different hash detected")
	}
}

func TestPendingClip(t *testing.T) {
	clipMu.Lock()
	pendingClip = "test pending"
	clipMu.Unlock()

	clipMu.Lock()
	pending := pendingClip
	pendingClip = ""
	clipMu.Unlock()

	if pending != "test pending" {
		t.Errorf("expected 'test pending', got %q", pending)
	}

	clipMu.Lock()
	remaining := pendingClip
	clipMu.Unlock()

	if remaining != "" {
		t.Error("pendingClip should be cleared after consumption")
	}
}
