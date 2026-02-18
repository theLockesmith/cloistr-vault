package identity

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Bech32 charset for encoding
const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// Bech32 generator constants
var bech32Gen = []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}

// EncodeNpub converts a hex public key to bech32 npub format
// Example: "abc123..." -> "npub1..."
func EncodeNpub(hexPubkey string) (string, error) {
	// Validate hex pubkey
	if len(hexPubkey) != 64 {
		return "", fmt.Errorf("invalid pubkey length: expected 64 hex chars, got %d", len(hexPubkey))
	}

	// Decode hex to bytes
	data, err := hex.DecodeString(hexPubkey)
	if err != nil {
		return "", fmt.Errorf("invalid hex pubkey: %w", err)
	}

	// Convert 8-bit bytes to 5-bit words
	words, err := convertBits(data, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("bit conversion failed: %w", err)
	}

	// Encode with bech32
	return bech32Encode("npub", words)
}

// DecodeNpub converts a bech32 npub to hex public key
// Example: "npub1..." -> "abc123..."
func DecodeNpub(npub string) (string, error) {
	hrp, data, err := bech32Decode(npub)
	if err != nil {
		return "", err
	}

	if hrp != "npub" {
		return "", fmt.Errorf("invalid hrp: expected 'npub', got '%s'", hrp)
	}

	// Convert 5-bit words back to 8-bit bytes
	bytes, err := convertBits(data, 5, 8, false)
	if err != nil {
		return "", fmt.Errorf("bit conversion failed: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}

// EncodeNsec converts a hex private key to bech32 nsec format
func EncodeNsec(hexPrivkey string) (string, error) {
	if len(hexPrivkey) != 64 {
		return "", fmt.Errorf("invalid privkey length: expected 64 hex chars, got %d", len(hexPrivkey))
	}

	data, err := hex.DecodeString(hexPrivkey)
	if err != nil {
		return "", fmt.Errorf("invalid hex privkey: %w", err)
	}

	words, err := convertBits(data, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("bit conversion failed: %w", err)
	}

	return bech32Encode("nsec", words)
}

// FormatNpubShort returns a shortened npub display (npub1abc...xyz)
func FormatNpubShort(hexPubkey string) string {
	npub, err := EncodeNpub(hexPubkey)
	if err != nil {
		// Fallback to hex format
		if len(hexPubkey) >= 16 {
			return hexPubkey[:8] + "..." + hexPubkey[len(hexPubkey)-8:]
		}
		return hexPubkey
	}

	// Return shortened npub: npub1 + first 8 chars + ... + last 8 chars
	if len(npub) > 20 {
		return npub[:12] + "..." + npub[len(npub)-8:]
	}
	return npub
}

// GetDisplayNameForNostrUser returns the best display name for a Nostr user
func GetDisplayNameForNostrUser(hexPubkey string, lightningAddr *string, nip05 *string) string {
	// Priority:
	// 1. NIP-05 address (alice@domain.com)
	// 2. Lightning address (alice@coldforge.xyz)
	// 3. Short npub (npub1abc...xyz)

	if nip05 != nil && *nip05 != "" {
		return *nip05
	}

	if lightningAddr != nil && *lightningAddr != "" {
		return *lightningAddr
	}

	return FormatNpubShort(hexPubkey)
}

// bech32 implementation

func bech32Encode(hrp string, data []byte) (string, error) {
	// Create checksum
	values := append([]byte{}, data...)
	checksum := bech32CreateChecksum(hrp, values)
	combined := append(values, checksum...)

	// Build result
	var result strings.Builder
	result.WriteString(hrp)
	result.WriteByte('1')

	for _, v := range combined {
		if int(v) >= len(bech32Charset) {
			return "", fmt.Errorf("invalid data value: %d", v)
		}
		result.WriteByte(bech32Charset[v])
	}

	return result.String(), nil
}

func bech32Decode(bech string) (string, []byte, error) {
	// Find separator
	pos := strings.LastIndex(bech, "1")
	if pos < 1 || pos+7 > len(bech) {
		return "", nil, fmt.Errorf("invalid bech32 string")
	}

	// Lowercase for processing
	bech = strings.ToLower(bech)
	hrp := bech[:pos]
	dataStr := bech[pos+1:]

	// Decode data characters
	data := make([]byte, 0, len(dataStr))
	for _, c := range dataStr {
		idx := strings.IndexRune(bech32Charset, c)
		if idx < 0 {
			return "", nil, fmt.Errorf("invalid character: %c", c)
		}
		data = append(data, byte(idx))
	}

	// Verify checksum
	if !bech32VerifyChecksum(hrp, data) {
		return "", nil, fmt.Errorf("invalid checksum")
	}

	// Remove checksum (last 6 bytes)
	return hrp, data[:len(data)-6], nil
}

func bech32CreateChecksum(hrp string, data []byte) []byte {
	values := append(bech32HrpExpand(hrp), data...)
	polymod := bech32Polymod(append(values, 0, 0, 0, 0, 0, 0)) ^ 1

	checksum := make([]byte, 6)
	for i := 0; i < 6; i++ {
		checksum[i] = byte((polymod >> (5 * (5 - i))) & 31)
	}
	return checksum
}

func bech32VerifyChecksum(hrp string, data []byte) bool {
	return bech32Polymod(append(bech32HrpExpand(hrp), data...)) == 1
}

func bech32HrpExpand(hrp string) []byte {
	result := make([]byte, len(hrp)*2+1)
	for i, c := range hrp {
		result[i] = byte(c >> 5)
		result[i+len(hrp)+1] = byte(c & 31)
	}
	result[len(hrp)] = 0
	return result
}

func bech32Polymod(values []byte) int {
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ int(v)
		for i := 0; i < 5; i++ {
			if (b>>i)&1 == 1 {
				chk ^= bech32Gen[i]
			}
		}
	}
	return chk
}

func convertBits(data []byte, fromBits, toBits int, pad bool) ([]byte, error) {
	acc := 0
	bits := 0
	maxv := (1 << toBits) - 1
	result := make([]byte, 0, len(data)*fromBits/toBits+1)

	for _, value := range data {
		if int(value)>>fromBits != 0 {
			return nil, fmt.Errorf("invalid value: %d", value)
		}
		acc = (acc << fromBits) | int(value)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			result = append(result, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			result = append(result, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	return result, nil
}
