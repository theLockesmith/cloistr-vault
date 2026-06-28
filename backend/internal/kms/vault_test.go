package kms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newMockVault returns an httptest server that emulates the subset of the
// Vault KV v2 + sys/health API that VaultKMS uses, backed by an in-memory map.
func newMockVault(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	store := map[string]map[string]interface{}{} // sub-path -> inner data

	h := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"initialized":true,"sealed":false}`))
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/v1/secret/")
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(path, "data/"):
			sub := strings.TrimPrefix(path, "data/")
			var body struct {
				Data map[string]interface{} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			store[sub] = body.Data
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"version":1}}`))

		case r.Method == http.MethodGet && strings.HasPrefix(path, "data/"):
			sub := strings.TrimPrefix(path, "data/")
			d, ok := store[sub]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":[]}`))
				return
			}
			resp := map[string]interface{}{"data": map[string]interface{}{"data": d}}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == "LIST" && strings.HasPrefix(path, "metadata/"):
			sub := strings.TrimPrefix(path, "metadata/")
			prefix := sub + "/"
			seen := map[string]bool{}
			for k := range store {
				if strings.HasPrefix(k, prefix) {
					child := strings.SplitN(strings.TrimPrefix(k, prefix), "/", 2)[0]
					seen[child] = true
				}
			}
			keys := []string{}
			for k := range seen {
				keys = append(keys, k)
			}
			if len(keys) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			resp := map[string]interface{}{"data": map[string]interface{}{"keys": keys}}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	return httptest.NewServer(http.HandlerFunc(h))
}

func newTestVaultKMS(t *testing.T) *VaultKMS {
	t.Helper()
	srv := newMockVault(t)
	t.Cleanup(srv.Close)
	k, err := NewVaultKMS(&Config{
		Provider:  "vault",
		Address:   srv.URL,
		Token:     "test-token",
		MountPath: "secret",
	})
	if err != nil {
		t.Fatalf("NewVaultKMS: %v", err)
	}
	return k
}

func TestVaultKMS_RequiresConfig(t *testing.T) {
	if _, err := NewVaultKMS(&Config{Token: "t"}); err == nil {
		t.Fatal("expected error when address missing")
	}
	if _, err := NewVaultKMS(&Config{Address: "https://v"}); err == nil {
		t.Fatal("expected error when token missing")
	}
}

func TestVaultKMS_GenerateAndGetLatest(t *testing.T) {
	k := newTestVaultKMS(t)
	ctx := context.Background()

	gen, err := k.GenerateKey(ctx, KeyTypeJWT, 256)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if len(gen.KeyMaterial) != 32 {
		t.Fatalf("expected 32 bytes of key material, got %d", len(gen.KeyMaterial))
	}

	got, err := k.GetLatestKey(ctx, KeyTypeJWT)
	if err != nil {
		t.Fatalf("GetLatestKey: %v", err)
	}
	if got.Version != gen.Version {
		t.Fatalf("version mismatch: %s vs %s", got.Version, gen.Version)
	}
	if string(got.KeyMaterial) != string(gen.KeyMaterial) {
		t.Fatal("retrieved key material differs from generated (base64 round-trip broken)")
	}
}

func TestVaultKMS_NotFound(t *testing.T) {
	k := newTestVaultKMS(t)
	if _, err := k.GetLatestKey(context.Background(), KeyTypeDatabase); err == nil {
		t.Fatal("expected key-not-found error")
	}
}

func TestVaultKMS_EncryptDecryptRoundTrip(t *testing.T) {
	k := newTestVaultKMS(t)
	ctx := context.Background()
	if _, err := k.GenerateKey(ctx, KeyTypeDatabase, 256); err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	plaintext := []byte("the quick brown fox jumps over the lazy dog")
	ct, err := k.Encrypt(ctx, KeyTypeDatabase, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(ct) == string(plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}
	pt, err := k.Decrypt(ctx, KeyTypeDatabase, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(pt) != string(plaintext) {
		t.Fatalf("round-trip mismatch: %q != %q", pt, plaintext)
	}

	// Tampered ciphertext must fail (GCM authentication).
	bad := append([]byte{}, ct...)
	bad[len(bad)-1] ^= 0xFF
	if _, err := k.Decrypt(ctx, KeyTypeDatabase, bad); err == nil {
		t.Fatal("expected decryption of tampered ciphertext to fail")
	}
}

func TestVaultKMS_SignVerify(t *testing.T) {
	k := newTestVaultKMS(t)
	ctx := context.Background()
	if _, err := k.GenerateKey(ctx, KeyTypeAPISignature, 256); err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	data := []byte("payload-to-sign")
	sig, err := k.Sign(ctx, KeyTypeAPISignature, data)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := k.Verify(ctx, KeyTypeAPISignature, data, sig); err != nil {
		t.Fatalf("Verify (valid): %v", err)
	}
	if err := k.Verify(ctx, KeyTypeAPISignature, []byte("tampered"), sig); err == nil {
		t.Fatal("expected verify to fail for tampered data")
	}
}

func TestVaultKMS_RotateAndList(t *testing.T) {
	k := newTestVaultKMS(t)
	ctx := context.Background()

	first, err := k.GenerateKey(ctx, KeyTypeJWT, 256)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	second, err := k.RotateKey(ctx, KeyTypeJWT)
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if first.Version == second.Version {
		t.Fatal("rotate did not produce a new version")
	}

	latest, err := k.GetLatestKey(ctx, KeyTypeJWT)
	if err != nil {
		t.Fatalf("GetLatestKey: %v", err)
	}
	if latest.Version != second.Version {
		t.Fatalf("latest should be the rotated key: %s != %s", latest.Version, second.Version)
	}

	keys, err := k.ListKeys(ctx, KeyTypeJWT)
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 key versions (excluding _latest pointer), got %d", len(keys))
	}
}

func TestVaultKMS_HealthAndStatus(t *testing.T) {
	k := newTestVaultKMS(t)
	ctx := context.Background()
	if err := k.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	st, err := k.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !st.Healthy || st.Provider != "vault" {
		t.Fatalf("unexpected status: %+v", st)
	}
}
