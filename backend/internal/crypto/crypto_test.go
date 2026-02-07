package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name string
		size int
		want bool
	}{
		{"32 bytes", 32, true},
		{"16 bytes", 16, true},
		{"0 bytes", 0, true},
		{"1 byte", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateRandomBytes(tt.size)
			if (err == nil) != tt.want {
				t.Errorf("GenerateRandomBytes() error = %v, want success = %v", err, tt.want)
				return
			}
			if len(got) != tt.size {
				t.Errorf("GenerateRandomBytes() length = %v, want %v", len(got), tt.size)
			}
			
			// Test that two calls produce different results (with high probability)
			if tt.size > 0 {
				got2, _ := GenerateRandomBytes(tt.size)
				if bytes.Equal(got, got2) {
					t.Error("GenerateRandomBytes() produced identical results, expected different")
				}
			}
		})
	}
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}
	
	if len(salt1) != SaltSize {
		t.Errorf("GenerateSalt() length = %v, want %v", len(salt1), SaltSize)
	}
	
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}
	
	if bytes.Equal(salt1, salt2) {
		t.Error("GenerateSalt() produced identical salts")
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}
	
	if len(nonce1) != NonceSize {
		t.Errorf("GenerateNonce() length = %v, want %v", len(nonce1), NonceSize)
	}
	
	nonce2, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}
	
	if bytes.Equal(nonce1, nonce2) {
		t.Error("GenerateNonce() produced identical nonces")
	}
}

func TestDeriveKey(t *testing.T) {
	password := "test-password-123"
	salt, _ := GenerateSalt()
	
	tests := []struct {
		name     string
		password string
		salt     []byte
		n        int
		r        int
		p        int
		wantErr  bool
	}{
		{"valid parameters", password, salt, 32768, 8, 1, false},
		{"low N parameter", password, salt, 16, 8, 1, false},
		{"invalid salt size", password, []byte("short"), 32768, 8, 1, true},
		{"empty salt", password, []byte{}, 32768, 8, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeriveKey(tt.password, tt.salt, tt.n, tt.r, tt.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeriveKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != KeySize {
					t.Errorf("DeriveKey() key length = %v, want %v", len(got), KeySize)
				}
				
				// Test deterministic behavior
				got2, err2 := DeriveKey(tt.password, tt.salt, tt.n, tt.r, tt.p)
				if err2 != nil {
					t.Errorf("DeriveKey() second call error = %v", err2)
				}
				if !bytes.Equal(got, got2) {
					t.Error("DeriveKey() not deterministic")
				}
				
				// Test different password produces different key
				got3, _ := DeriveKey("different-password", tt.salt, tt.n, tt.r, tt.p)
				if bytes.Equal(got, got3) {
					t.Error("DeriveKey() same key for different passwords")
				}
			}
		})
	}
}

func TestDeriveKeyDefault(t *testing.T) {
	password := "test-password"
	salt, _ := GenerateSalt()
	
	key, err := DeriveKeyDefault(password, salt)
	if err != nil {
		t.Fatalf("DeriveKeyDefault() error = %v", err)
	}
	
	if len(key) != KeySize {
		t.Errorf("DeriveKeyDefault() key length = %v, want %v", len(key), KeySize)
	}
	
	// Should match manual call with default parameters
	expectedKey, _ := DeriveKey(password, salt, ScryptN, ScryptR, ScryptP)
	if !bytes.Equal(key, expectedKey) {
		t.Error("DeriveKeyDefault() doesn't match manual default parameters")
	}
}

func TestHashPassword(t *testing.T) {
	password := "secure-password-123"
	salt, _ := GenerateSalt()
	
	hash, err := HashPassword(password, salt)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	
	if len(hash) != 32 { // SHA256 output size
		t.Errorf("HashPassword() hash length = %v, want 32", len(hash))
	}
	
	// Test deterministic behavior
	hash2, err := HashPassword(password, salt)
	if err != nil {
		t.Fatalf("HashPassword() second call error = %v", err)
	}
	
	if !bytes.Equal(hash, hash2) {
		t.Error("HashPassword() not deterministic")
	}
	
	// Test different password produces different hash
	hash3, _ := HashPassword("different-password", salt)
	if bytes.Equal(hash, hash3) {
		t.Error("HashPassword() same hash for different passwords")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "test-password-verify"
	wrongPassword := "wrong-password"
	salt, _ := GenerateSalt()
	hash, _ := HashPassword(password, salt)
	
	tests := []struct {
		name     string
		password string
		salt     []byte
		hash     []byte
		want     bool
	}{
		{"correct password", password, salt, hash, true},
		{"wrong password", wrongPassword, salt, hash, false},
		{"wrong salt", password, []byte("wrong-salt-32-bytes-long-exactly"), hash, false},
		{"corrupted hash", password, salt, []byte("corrupted-hash-32-bytes-long-exa"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifyPassword(tt.password, tt.salt, tt.hash)
			if got != tt.want {
				t.Errorf("VerifyPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstantTimeEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{"equal bytes", []byte{1, 2, 3, 4}, []byte{1, 2, 3, 4}, true},
		{"different bytes", []byte{1, 2, 3, 4}, []byte{1, 2, 3, 5}, false},
		{"different lengths", []byte{1, 2, 3}, []byte{1, 2, 3, 4}, false},
		{"empty slices", []byte{}, []byte{}, true},
		{"one empty", []byte{1}, []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constantTimeEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("constantTimeEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncryptDecryptAESGCM(t *testing.T) {
	key, _ := GenerateRandomBytes(KeySize)
	
	tests := []struct {
		name      string
		plaintext []byte
		wantErr   bool
	}{
		{"simple text", []byte("Hello, World!"), false},
		{"empty text", []byte(""), true},
		{"large text", bytes.Repeat([]byte("A"), 10000), false},
		{"binary data", []byte{0, 1, 2, 3, 255, 254, 253}, false},
		{"unicode text", []byte("Hello, 世界! 🔒"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test encryption
			ciphertext, nonce, err := EncryptAESGCM(tt.plaintext, key)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncryptAESGCM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr {
				return
			}
			
			// Verify nonce size
			if len(nonce) != NonceSize {
				t.Errorf("EncryptAESGCM() nonce length = %v, want %v", len(nonce), NonceSize)
			}
			
			// Verify ciphertext is different from plaintext
			if bytes.Equal(ciphertext, tt.plaintext) {
				t.Error("EncryptAESGCM() ciphertext equals plaintext")
			}
			
			// Test decryption
			decrypted, err := DecryptAESGCM(ciphertext, key, nonce)
			if err != nil {
				t.Errorf("DecryptAESGCM() error = %v", err)
				return
			}
			
			// Verify decrypted data matches original
			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Errorf("DecryptAESGCM() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptAESGCMErrors(t *testing.T) {
	plaintext := []byte("test data")
	
	tests := []struct {
		name    string
		key     []byte
		wantErr error
	}{
		{"wrong key size - too short", []byte("short"), ErrInvalidKeySize},
		{"wrong key size - too long", bytes.Repeat([]byte("A"), 64), ErrInvalidKeySize},
		{"nil key", nil, ErrInvalidKeySize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := EncryptAESGCM(plaintext, tt.key)
			if err != tt.wantErr {
				t.Errorf("EncryptAESGCM() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecryptAESGCMErrors(t *testing.T) {
	key, _ := GenerateRandomBytes(KeySize)
	plaintext := []byte("test data")
	ciphertext, nonce, _ := EncryptAESGCM(plaintext, key)
	
	tests := []struct {
		name       string
		ciphertext []byte
		key        []byte
		nonce      []byte
		wantErr    error
	}{
		{"wrong key size", ciphertext, []byte("short"), nonce, ErrInvalidKeySize},
		{"wrong nonce size", ciphertext, key, []byte("short"), ErrInvalidNonceSize},
		{"corrupted ciphertext", []byte("corrupted"), key, nonce, ErrDecryptionFailed},
		{"wrong key", ciphertext, bytes.Repeat([]byte("B"), KeySize), nonce, ErrDecryptionFailed},
		{"wrong nonce", ciphertext, key, bytes.Repeat([]byte{1}, NonceSize), ErrDecryptionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptAESGCM(tt.ciphertext, tt.key, tt.nonce)
			if err != tt.wantErr {
				t.Errorf("DecryptAESGCM() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecureWipe(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	original := make([]byte, len(data))
	copy(original, data)
	
	SecureWipe(data)
	
	// Verify all bytes are zeroed
	for i, b := range data {
		if b != 0 {
			t.Errorf("SecureWipe() byte %d = %v, want 0", i, b)
		}
	}
	
	// Verify original data was different
	allZero := true
	for _, b := range original {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("Test data was already all zeros, test invalid")
	}
}

// Benchmark tests for performance monitoring
func BenchmarkDeriveKey(b *testing.B) {
	password := "benchmark-password"
	salt, _ := GenerateSalt()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DeriveKeyDefault(password, salt)
	}
}

func BenchmarkEncryptAESGCM(b *testing.B) {
	key, _ := GenerateRandomBytes(KeySize)
	plaintext := bytes.Repeat([]byte("A"), 1024) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = EncryptAESGCM(plaintext, key)
	}
}

func BenchmarkDecryptAESGCM(b *testing.B) {
	key, _ := GenerateRandomBytes(KeySize)
	plaintext := bytes.Repeat([]byte("A"), 1024) // 1KB
	ciphertext, nonce, _ := EncryptAESGCM(plaintext, key)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecryptAESGCM(ciphertext, key, nonce)
	}
}