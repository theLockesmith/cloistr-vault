# Security Model

## Zero-Knowledge Architecture

Coldforge Vault implements a **zero-knowledge architecture** where the server never has access to your unencrypted data. This means:

- ✅ **All encryption/decryption happens on your device**
- ✅ **Server only stores encrypted blobs**
- ✅ **Your master key never leaves your device**
- ✅ **Even if servers are compromised, your data remains secure**
- ✅ **We cannot decrypt your data even if legally compelled**

## Encryption Details

### Client-Side Encryption
- **Algorithm**: AES-256-GCM for vault data encryption
- **Key Derivation**: Scrypt with N=32768, r=8, p=1
- **Salt**: 32-byte random salt per user
- **Nonce**: 12-byte random nonce per encryption operation

### Password Hashing
- **Algorithm**: Scrypt followed by SHA-256
- **Parameters**: N=32768, r=8, p=1 (configurable)
- **Salt**: 32-byte random salt per password
- **Storage**: Only the hash is stored, never the plaintext

### Nostr Integration
- **Key Format**: secp256k1 private keys (32 bytes)
- **Signatures**: ECDSA with SHA-256 hashing
- **Challenge**: 32-byte random challenge for each auth attempt
- **Key Derivation**: Scrypt using private key as input

## Authentication Methods

### 1. Email/Password Authentication

**Registration Flow:**
```
1. User provides email + password
2. Client generates random salt
3. Client derives key: scrypt(password, salt)
4. Client encrypts vault with derived key
5. Client hashes password: sha256(scrypt(password, salt))
6. Server stores: email, salt, password_hash, encrypted_vault
```

**Login Flow:**
```
1. User provides email + password
2. Server provides salt for email
3. Client derives key: scrypt(password, salt)
4. Client hashes password: sha256(derived_key)
5. Server compares hash
6. If valid, server returns encrypted vault
7. Client decrypts vault with derived key
```

### 2. Nostr Authentication

**Registration Flow:**
```
1. User provides nostr public key
2. Client derives vault key: scrypt(private_key, salt)
3. Client encrypts vault with derived key
4. Server stores: nostr_pubkey, encrypted_vault
```

**Login Flow:**
```
1. User provides nostr public key
2. Server generates random challenge
3. Client signs challenge with private key
4. Server verifies signature
5. If valid, server returns encrypted vault
6. Client derives key and decrypts vault
```

## Recovery Mechanisms

### Recovery Codes
- Generated during account creation
- 8 recovery codes in format: `xxxx-yyyy-zzzz`
- Each code can only be used once
- Hashed and salted before storage
- Can be used to reset password or recover account

### Trusted Device Recovery
- Devices can be marked as "trusted"
- Trusted devices can authorize new device access
- Uses public key cryptography for device authentication
- Time-locked recovery process for additional security

## Security Features

### Session Management
- JWT tokens with configurable expiration
- Automatic session cleanup
- Secure session storage
- Single sign-on across applications

### Rate Limiting
- Login attempt rate limiting
- API endpoint rate limiting  
- Progressive delays for failed attempts
- IP-based and user-based limits

### Audit Logging
- All authentication events logged
- Vault access logging
- Failed login attempt tracking
- Exportable audit trails

### Data Protection
- Constant-time password comparison
- Secure memory wiping for sensitive data
- Protection against timing attacks
- Input validation and sanitization

## Threat Model

### Protected Against:
- ✅ **Server compromise** - encrypted data remains secure
- ✅ **Database breach** - only encrypted data and hashes stored
- ✅ **Man-in-the-middle attacks** - TLS + client-side encryption
- ✅ **Timing attacks** - constant-time comparisons
- ✅ **Rainbow table attacks** - salted and stretched passwords
- ✅ **Legal data requests** - we cannot decrypt user data

### Considerations:
- ⚠️ **Client-side malware** - could capture master password
- ⚠️ **Phishing attacks** - users must verify authentic domains
- ⚠️ **Lost master password** - no recovery without recovery codes
- ⚠️ **Compromised client device** - local data may be accessible

## Best Practices

### For Users:
1. **Use a strong, unique master password**
2. **Store recovery codes in a safe location**
3. **Enable two-factor authentication when available**
4. **Keep client applications updated**
5. **Use trusted devices for sensitive operations**

### For Deployment:
1. **Use HTTPS everywhere** - no exceptions
2. **Configure proper TLS certificates**
3. **Enable database encryption at rest**
4. **Implement proper network security**
5. **Regular security updates and patches**
6. **Monitor for suspicious activity**

## Compliance

### Security Standards:
- Follows OWASP security guidelines
- Implements defense-in-depth strategy
- Uses industry-standard cryptography
- Regular security audits (recommended)

### Privacy:
- Zero-knowledge by design
- Minimal data collection
- No tracking or analytics on sensitive data
- GDPR compliance ready

## Security Audits

We recommend:
- **Regular penetration testing**
- **Code security reviews**
- **Dependency vulnerability scanning**
- **Infrastructure security assessments**

For security researchers: Please report vulnerabilities responsibly to security@coldforge-vault.com

## Cryptographic Dependencies

- **golang.org/x/crypto** - Scrypt implementation
- **crypto/aes** - AES encryption (Go standard library)
- **crypto/cipher** - GCM mode implementation
- **secp256k1** - Elliptic curve cryptography for Nostr
- **crypto/sha256** - Hashing (Go standard library)

All cryptographic operations use well-established, peer-reviewed implementations.