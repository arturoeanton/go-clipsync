package main

import (
	"testing"
)

func TestClipHash(t *testing.T) {
	hash := clipHash("hello world")
	if hash == 0 {
		t.Error("clipHash should not return 0 for non-empty input")
	}
}

func TestClipHashDeterministic(t *testing.T) {
	hash1 := clipHash("test string")
	hash2 := clipHash("test string")
	if hash1 != hash2 {
		t.Errorf("clipHash should be deterministic: got %d and %d", hash1, hash2)
	}
}

func TestClipHashDifferent(t *testing.T) {
	hash1 := clipHash("hello")
	hash2 := clipHash("world")
	if hash1 == hash2 {
		t.Error("clipHash should return different values for different inputs")
	}
}

func TestClipHashEmpty(t *testing.T) {
	hash := clipHash("")
	// CRC32 of empty string is 0
	if hash != 0 {
		t.Errorf("clipHash of empty string should be 0, got %d", hash)
	}
}
