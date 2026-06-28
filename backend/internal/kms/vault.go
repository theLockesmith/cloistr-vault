package kms

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VaultKMS implements the KMS interface backed by a HashiCorp Vault KV v2
// secrets engine. Key material is stored in Vault (persistent and shared
// across replicas, unlike the file provider's per-pod ephemeral keys), and
// cryptographic operations are performed locally with the retrieved key.
//
// Keys are laid out under the mount as:
//
//	<mount>/data/cloistr-vault/keys/<keyType>/<version>   # one doc per key version
//	<mount>/data/cloistr-vault/keys/<keyType>/_latest     # { "version": "<version>" }
//
// Key material is base64-encoded/decoded by us on both sides, avoiding the
// transit-engine base64 ambiguity that previously corrupted keys elsewhere.
type VaultKMS struct {
	client    *http.Client
	address   string // e.g. https://vault.coldforge.xyz:8200
	token     string
	mountPath string // KV v2 mount, e.g. "secret"
	config    *Config
}

// Compile-time assertion that VaultKMS satisfies the KMS interface.
var _ KMS = (*VaultKMS)(nil)

const vaultKeyPrefix = "cloistr-vault/keys"

// NewVaultKMS constructs a Vault-backed KMS. It validates configuration only;
// connectivity is exercised lazily (and via HealthCheck). On missing config it
// returns an error so the caller can fall back to the file provider.
func NewVaultKMS(cfg *Config) (*VaultKMS, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("vault kms: address is required (set KMS_ADDRESS)")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("vault kms: token is required (set KMS_TOKEN)")
	}
	mount := cfg.MountPath
	if mount == "" {
		mount = "secret"
	}

	transport := &http.Transport{}
	if cfg.Options != nil && cfg.Options["tls_skip_verify"] == "true" {
		// Vault uses a self-signed CA in this environment.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &VaultKMS{
		client:    &http.Client{Timeout: 15 * time.Second, Transport: transport},
		address:   strings.TrimRight(cfg.Address, "/"),
		token:     cfg.Token,
		mountPath: strings.Trim(mount, "/"),
		config:    cfg,
	}, nil
}

// --- Vault HTTP plumbing -------------------------------------------------

// vaultDo issues a request against the Vault API and returns the parsed JSON
// body (may be nil for empty responses) plus the HTTP status code.
func (v *VaultKMS) vaultDo(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(b)
	}

	url := fmt.Sprintf("%s/v1/%s", v.address, strings.TrimLeft(path, "/"))
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Vault-Token", v.token)
	req.Header.Set("X-Vault-Request", "true")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if len(raw) == 0 {
		return nil, resp.StatusCode, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("vault response parse error (status %d): %w", resp.StatusCode, err)
	}
	return parsed, resp.StatusCode, nil
}

func (v *VaultKMS) dataPath(sub string) string     { return fmt.Sprintf("%s/data/%s", v.mountPath, sub) }
func (v *VaultKMS) metadataPath(sub string) string { return fmt.Sprintf("%s/metadata/%s", v.mountPath, sub) }

// kvPut writes a KV v2 secret at the given sub-path under the mount.
func (v *VaultKMS) kvPut(ctx context.Context, sub string, data map[string]interface{}) error {
	_, code, err := v.vaultDo(ctx, http.MethodPost, v.dataPath(sub), map[string]interface{}{"data": data})
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return NewProviderError(fmt.Sprintf("vault write %s returned status %d", sub, code))
	}
	return nil
}

// kvGet reads a KV v2 secret. Returns NewKeyNotFoundError on 404.
func (v *VaultKMS) kvGet(ctx context.Context, sub string, keyType KeyType) (map[string]interface{}, error) {
	parsed, code, err := v.vaultDo(ctx, http.MethodGet, v.dataPath(sub), nil)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, NewKeyNotFoundError(keyType)
	}
	if code != http.StatusOK {
		return nil, NewProviderError(fmt.Sprintf("vault read %s returned status %d", sub, code))
	}
	// KV v2 wraps the secret as { "data": { "data": {...}, "metadata": {...} } }.
	outer, ok := parsed["data"].(map[string]interface{})
	if !ok {
		return nil, NewProviderError("vault read: missing data envelope")
	}
	inner, ok := outer["data"].(map[string]interface{})
	if !ok {
		return nil, NewKeyNotFoundError(keyType)
	}
	return inner, nil
}

// kvList lists child keys under a KV v2 metadata path. Returns nil on 404.
func (v *VaultKMS) kvList(ctx context.Context, sub string) ([]string, error) {
	parsed, code, err := v.vaultDo(ctx, "LIST", v.metadataPath(sub), nil)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, nil
	}
	if code != http.StatusOK {
		return nil, NewProviderError(fmt.Sprintf("vault list %s returned status %d", sub, code))
	}
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	rawKeys, ok := data["keys"].([]interface{})
	if !ok {
		return nil, nil
	}
	keys := make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, ok := k.(string); ok {
			keys = append(keys, s)
		}
	}
	return keys, nil
}

// --- KMS interface -------------------------------------------------------

func (v *VaultKMS) GenerateKey(ctx context.Context, keyType KeyType, keySize int) (*KeyInfo, error) {
	if keySize <= 0 {
		keySize = v.getDefaultKeySize(keyType)
	}
	keyBytes := make([]byte, keySize/8)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	now := time.Now()
	version := fmt.Sprintf("v%d", now.UnixNano())
	info := &KeyInfo{
		ID:        fmt.Sprintf("%s-%s", keyType, version),
		Type:      keyType,
		Version:   version,
		Algorithm: v.getAlgorithmForKeyType(keyType),
		KeySize:   keySize,
		CreatedAt: now,
		Status:    KeyStatusActive,
		Metadata: map[string]string{
			"created_by": "coldforge-vault",
			"key_usage":  string(keyType),
		},
		KeyMaterial: keyBytes,
	}

	doc := map[string]interface{}{
		"id":           info.ID,
		"type":         string(info.Type),
		"version":      info.Version,
		"algorithm":    info.Algorithm,
		"key_size":     info.KeySize,
		"created_at":   info.CreatedAt.Format(time.RFC3339),
		"status":       string(info.Status),
		"metadata":     info.Metadata,
		"key_material": base64.StdEncoding.EncodeToString(keyBytes),
	}

	keyBase := vaultKeyPrefix + "/" + string(keyType)
	if err := v.kvPut(ctx, keyBase+"/"+version, doc); err != nil {
		return nil, err
	}
	if err := v.kvPut(ctx, keyBase+"/_latest", map[string]interface{}{"version": version}); err != nil {
		return nil, err
	}
	return info, nil
}

func (v *VaultKMS) GetKey(ctx context.Context, keyType KeyType, version string) (*KeyInfo, error) {
	keyBase := vaultKeyPrefix + "/" + string(keyType)
	data, err := v.kvGet(ctx, keyBase+"/"+version, keyType)
	if err != nil {
		return nil, err
	}
	return v.parseKeyInfo(data, keyType)
}

func (v *VaultKMS) GetLatestKey(ctx context.Context, keyType KeyType) (*KeyInfo, error) {
	keyBase := vaultKeyPrefix + "/" + string(keyType)
	ptr, err := v.kvGet(ctx, keyBase+"/_latest", keyType)
	if err != nil {
		return nil, err
	}
	version, _ := ptr["version"].(string)
	if version == "" {
		return nil, NewKeyNotFoundError(keyType)
	}
	return v.GetKey(ctx, keyType, version)
}

func (v *VaultKMS) Sign(ctx context.Context, keyType KeyType, data []byte) ([]byte, error) {
	key, err := v.GetLatestKey(ctx, keyType)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key.KeyMaterial)
	mac.Write(data)
	return mac.Sum(nil), nil
}

func (v *VaultKMS) Verify(ctx context.Context, keyType KeyType, data []byte, signature []byte) error {
	expected, err := v.Sign(ctx, keyType, data)
	if err != nil {
		return err
	}
	if !hmac.Equal(expected, signature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// aeadFor builds an AES-256-GCM AEAD from the latest key for a key type.
func (v *VaultKMS) aeadFor(ctx context.Context, keyType KeyType) (cipher.AEAD, error) {
	key, err := v.GetLatestKey(ctx, keyType)
	if err != nil {
		return nil, err
	}
	material := key.KeyMaterial
	if len(material) != 32 {
		// Normalize to a 32-byte AES-256 key.
		sum := sha256.Sum256(material)
		material = sum[:]
	}
	block, err := aes.NewCipher(material)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (v *VaultKMS) Encrypt(ctx context.Context, keyType KeyType, plaintext []byte) ([]byte, error) {
	aead, err := v.aeadFor(ctx, keyType)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// Output: nonce || ciphertext(+tag)
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (v *VaultKMS) Decrypt(ctx context.Context, keyType KeyType, ciphertext []byte) ([]byte, error) {
	aead, err := v.aeadFor(ctx, keyType)
	if err != nil {
		return nil, err
	}
	ns := aead.NonceSize()
	if len(ciphertext) < ns {
		return nil, &Error{Code: ErrCodeDecryptionError, Message: "ciphertext too short", KeyType: keyType}
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, &Error{Code: ErrCodeDecryptionError, Message: "decryption failed", KeyType: keyType}
	}
	return plaintext, nil
}

func (v *VaultKMS) RotateKey(ctx context.Context, keyType KeyType) (*KeyInfo, error) {
	current, err := v.GetLatestKey(ctx, keyType)
	if err != nil {
		// No current key — create one with the default size.
		return v.GenerateKey(ctx, keyType, v.getDefaultKeySize(keyType))
	}
	return v.GenerateKey(ctx, keyType, current.KeySize)
}

func (v *VaultKMS) DisableKey(ctx context.Context, keyType KeyType, version string) error {
	keyBase := vaultKeyPrefix + "/" + string(keyType)
	data, err := v.kvGet(ctx, keyBase+"/"+version, keyType)
	if err != nil {
		return err
	}
	data["status"] = string(KeyStatusDisabled)
	return v.kvPut(ctx, keyBase+"/"+version, data)
}

func (v *VaultKMS) ListKeys(ctx context.Context, keyType KeyType) ([]*KeyInfo, error) {
	keyBase := vaultKeyPrefix + "/" + string(keyType)
	versions, err := v.kvList(ctx, keyBase)
	if err != nil {
		return nil, err
	}
	var infos []*KeyInfo
	for _, ver := range versions {
		if ver == "_latest" || strings.HasSuffix(ver, "/") {
			continue
		}
		info, err := v.GetKey(ctx, keyType, ver)
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (v *VaultKMS) HealthCheck(ctx context.Context) error {
	_, code, err := v.vaultDo(ctx, http.MethodGet, "sys/health", nil)
	if err != nil {
		return err
	}
	// 200 active, 429 standby, 473 perf-standby — all reachable + unsealed.
	switch code {
	case http.StatusOK, http.StatusTooManyRequests, 473:
		return nil
	default:
		return NewProviderError(fmt.Sprintf("vault health check returned status %d", code))
	}
}

func (v *VaultKMS) GetStatus(ctx context.Context) (*Status, error) {
	status := &Status{
		Provider:     "vault",
		Healthy:      true,
		KeyCount:     make(map[KeyType]int),
		LastRotation: make(map[KeyType]time.Time),
		NextRotation: make(map[KeyType]time.Time),
	}
	if err := v.HealthCheck(ctx); err != nil {
		status.Healthy = false
		status.Errors = append(status.Errors, err.Error())
	}
	for _, kt := range []KeyType{KeyTypeJWT, KeyTypeDatabase, KeyTypeRedis, KeyTypeAPISignature} {
		if keys, err := v.ListKeys(ctx, kt); err == nil {
			status.KeyCount[kt] = len(keys)
		}
	}
	return status, nil
}

// --- helpers -------------------------------------------------------------

func (v *VaultKMS) parseKeyInfo(data map[string]interface{}, keyType KeyType) (*KeyInfo, error) {
	info := &KeyInfo{Type: keyType}

	if id, ok := data["id"].(string); ok {
		info.ID = id
	}
	if t, ok := data["type"].(string); ok {
		info.Type = KeyType(t)
	}
	if ver, ok := data["version"].(string); ok {
		info.Version = ver
	}
	if alg, ok := data["algorithm"].(string); ok {
		info.Algorithm = alg
	}
	if ks, ok := data["key_size"].(float64); ok {
		info.KeySize = int(ks)
	}
	if ca, ok := data["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ca); err == nil {
			info.CreatedAt = t
		}
	}
	if st, ok := data["status"].(string); ok {
		info.Status = KeyStatus(st)
	}
	if md, ok := data["metadata"].(map[string]interface{}); ok {
		info.Metadata = make(map[string]string)
		for k, val := range md {
			if s, ok := val.(string); ok {
				info.Metadata[k] = s
			}
		}
	}
	if km, ok := data["key_material"].(string); ok {
		decoded, err := base64.StdEncoding.DecodeString(km)
		if err != nil {
			return nil, NewProviderError("failed to decode key material")
		}
		info.KeyMaterial = decoded
	}
	return info, nil
}

func (v *VaultKMS) getAlgorithmForKeyType(keyType KeyType) string {
	switch keyType {
	case KeyTypeJWT, KeyTypeAPISignature:
		return "HMAC-SHA256"
	case KeyTypeDatabase, KeyTypeRedis:
		return "AES-256-GCM"
	case KeyTypeLightning, KeyTypeNostrRelay:
		return "secp256k1"
	default:
		return "AES-256-GCM"
	}
}

func (v *VaultKMS) getDefaultKeySize(keyType KeyType) int {
	// All managed operational keys are 256-bit.
	return 256
}
