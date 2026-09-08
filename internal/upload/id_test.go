package upload

import (
	"encoding/hex"
	"testing"
)

func TestRandomID(t *testing.T) {
	first, err := RandomID()
	if err != nil {
		t.Fatalf("RandomID() error: %v", err)
	}
	second, err := RandomID()
	if err != nil {
		t.Fatalf("RandomID() second error: %v", err)
	}

	if first == second {
		t.Errorf("RandomID() returned the same ID twice: %q", first)
	}
	if len(first) != randomIDBytes*2 {
		t.Errorf("RandomID() length = %d, want %d", len(first), randomIDBytes*2)
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Errorf("RandomID() = %q, want hexadecimal text: %v", first, err)
	}
}
