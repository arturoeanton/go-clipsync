package main

import (
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token := generatePairingToken(true)
	if len(token) != 32 {
		t.Errorf("expected 32 hex chars, got %d: %q", len(token), token)
	}
}

func TestTokenReuse(t *testing.T) {
	// Reset state
	pairingMu.Lock()
	pendingToken = ""
	pairedDevice = ""
	pairingMu.Unlock()

	token1 := generatePairingToken(false)
	token2 := generatePairingToken(false)
	if token1 != token2 {
		t.Errorf("expected same token without forceNew, got %q and %q", token1, token2)
	}
}

func TestForceNewToken(t *testing.T) {
	token1 := generatePairingToken(true)
	token2 := generatePairingToken(true)
	if token1 == token2 {
		t.Error("forceNew should generate different tokens")
	}
}

func TestConfirmPairing(t *testing.T) {
	origDB := db
	db = nil // no DB for this test
	defer func() { db = origDB }()

	pairingMu.Lock()
	pendingToken = "testtoken123"
	pairedDevice = ""
	pairingMu.Unlock()

	confirmPairing("testtoken123")

	if !isPaired() {
		t.Error("should be paired after confirmPairing")
	}

	pairingMu.Lock()
	if pairedDevice != "testtoken123" {
		t.Errorf("expected pairedDevice 'testtoken123', got %q", pairedDevice)
	}
	if pendingToken != "" {
		t.Error("pendingToken should be cleared after confirm")
	}
	pairingMu.Unlock()
}

func TestUnpair(t *testing.T) {
	origDB := db
	db = nil
	defer func() { db = origDB }()

	pairingMu.Lock()
	pairedDevice = "somedevice"
	pairingMu.Unlock()

	unpair()

	if isPaired() {
		t.Error("should not be paired after unpair")
	}
}

func TestGetPendingOrPairedToken(t *testing.T) {
	pairingMu.Lock()
	pairedDevice = ""
	pendingToken = "pending123"
	pairingMu.Unlock()

	token := getPendingOrPairedToken()
	if token != "pending123" {
		t.Errorf("expected pending token, got %q", token)
	}

	pairingMu.Lock()
	pairedDevice = "paired456"
	pairingMu.Unlock()

	token = getPendingOrPairedToken()
	if token != "paired456" {
		t.Errorf("expected paired token, got %q", token)
	}
}
