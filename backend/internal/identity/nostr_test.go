package identity

import (
	"strings"
	"testing"
)

func TestEncodeNpub(t *testing.T) {
	tests := []struct {
		name       string
		hexPubkey  string
		wantPrefix string
		wantErr    bool
	}{
		{
			name:       "valid pubkey",
			hexPubkey:  "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			wantPrefix: "npub1",
			wantErr:    false,
		},
		{
			name:       "another valid pubkey",
			hexPubkey:  "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2",
			wantPrefix: "npub1",
			wantErr:    false,
		},
		{
			name:       "invalid hex",
			hexPubkey:  "invalid_hex_string",
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "too short",
			hexPubkey:  "3bf0c63fcb93463407af97a5",
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "too long",
			hexPubkey:  "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d00",
			wantPrefix: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeNpub(tt.hexPubkey)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeNpub() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("EncodeNpub() = %v, want prefix %v", got, tt.wantPrefix)
			}
		})
	}
}

func TestDecodeNpub(t *testing.T) {
	// First encode a known pubkey
	hexPubkey := "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	npub, err := EncodeNpub(hexPubkey)
	if err != nil {
		t.Fatalf("EncodeNpub failed: %v", err)
	}

	// Then decode it back
	decoded, err := DecodeNpub(npub)
	if err != nil {
		t.Fatalf("DecodeNpub failed: %v", err)
	}

	if decoded != hexPubkey {
		t.Errorf("Round-trip failed: got %v, want %v", decoded, hexPubkey)
	}
}

func TestFormatNpubShort(t *testing.T) {
	tests := []struct {
		name      string
		hexPubkey string
		wantHas   string
	}{
		{
			name:      "valid pubkey",
			hexPubkey: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			wantHas:   "npub1",
		},
		{
			name:      "another valid pubkey",
			hexPubkey: "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2",
			wantHas:   "npub1",
		},
		{
			name:      "short pubkey fallback",
			hexPubkey: "abcdef1234567890",
			wantHas:   "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNpubShort(tt.hexPubkey)
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("FormatNpubShort() = %v, want to contain %v", got, tt.wantHas)
			}
			// Should contain "..." for truncation
			if len(tt.hexPubkey) == 64 && !strings.Contains(got, "...") {
				t.Errorf("FormatNpubShort() = %v, expected truncation with ...", got)
			}
		})
	}
}

func TestGetDisplayNameForNostrUser(t *testing.T) {
	hexPubkey := "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	lightningAddr := "alice@coldforge.xyz"
	nip05 := "alice@nostr.domain"

	// Test priority: NIP-05 > Lightning > npub
	t.Run("nip05 has highest priority", func(t *testing.T) {
		got := GetDisplayNameForNostrUser(hexPubkey, &lightningAddr, &nip05)
		if got != nip05 {
			t.Errorf("GetDisplayNameForNostrUser() = %v, want %v", got, nip05)
		}
	})

	t.Run("lightning is second priority", func(t *testing.T) {
		got := GetDisplayNameForNostrUser(hexPubkey, &lightningAddr, nil)
		if got != lightningAddr {
			t.Errorf("GetDisplayNameForNostrUser() = %v, want %v", got, lightningAddr)
		}
	})

	t.Run("npub is fallback", func(t *testing.T) {
		got := GetDisplayNameForNostrUser(hexPubkey, nil, nil)
		if !strings.HasPrefix(got, "npub1") {
			t.Errorf("GetDisplayNameForNostrUser() = %v, want npub prefix", got)
		}
	})
}

func TestKnownNpubVector(t *testing.T) {
	// Test vector from NIP-19
	// This is a well-known test pubkey
	hexPubkey := "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	expectedNpub := "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"

	npub, err := EncodeNpub(hexPubkey)
	if err != nil {
		t.Fatalf("EncodeNpub failed: %v", err)
	}

	if npub != expectedNpub {
		t.Errorf("EncodeNpub() = %v, want %v", npub, expectedNpub)
	}

	// Decode back
	decoded, err := DecodeNpub(expectedNpub)
	if err != nil {
		t.Fatalf("DecodeNpub failed: %v", err)
	}

	if decoded != hexPubkey {
		t.Errorf("DecodeNpub() = %v, want %v", decoded, hexPubkey)
	}
}
