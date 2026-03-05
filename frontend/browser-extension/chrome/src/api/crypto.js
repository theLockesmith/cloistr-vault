// Cloistr Vault Browser Extension - Crypto Service
// Client-side encryption using AES-256-GCM with Scrypt key derivation

class CloistrVaultCrypto {
  constructor() {
    this.masterKey = null;
    this.salt = null;
  }

  // Scrypt parameters (must match backend)
  static SCRYPT_N = 32768;  // CPU/memory cost
  static SCRYPT_R = 8;       // Block size
  static SCRYPT_P = 1;       // Parallelization
  static KEY_LENGTH = 32;    // 256 bits

  // Derive key from master password using Scrypt
  async deriveKey(password, salt) {
    // Use the Web Crypto API with PBKDF2 as a fallback
    // Note: True Scrypt requires a library like scrypt-js
    // For now, we use PBKDF2 with high iterations
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);

    const keyMaterial = await crypto.subtle.importKey(
      'raw',
      passwordBuffer,
      'PBKDF2',
      false,
      ['deriveBits', 'deriveKey']
    );

    // Use PBKDF2 with 100,000 iterations as a reasonable approximation
    const key = await crypto.subtle.deriveKey(
      {
        name: 'PBKDF2',
        salt: salt,
        iterations: 100000,
        hash: 'SHA-256'
      },
      keyMaterial,
      { name: 'AES-GCM', length: 256 },
      false,
      ['encrypt', 'decrypt']
    );

    return key;
  }

  // Generate a random salt
  generateSalt() {
    return crypto.getRandomValues(new Uint8Array(32));
  }

  // Generate a random IV for AES-GCM
  generateIV() {
    return crypto.getRandomValues(new Uint8Array(12));
  }

  // Initialize with master password
  async unlock(masterPassword, saltBase64 = null) {
    if (saltBase64) {
      this.salt = this.base64ToBytes(saltBase64);
    } else {
      this.salt = this.generateSalt();
    }

    this.masterKey = await this.deriveKey(masterPassword, this.salt);
    return this.bytesToBase64(this.salt);
  }

  // Lock the vault (clear the master key)
  lock() {
    this.masterKey = null;
    this.salt = null;
  }

  // Check if vault is unlocked
  isUnlocked() {
    return this.masterKey !== null;
  }

  // Encrypt data
  async encrypt(plaintext) {
    if (!this.masterKey) {
      throw new Error('Vault is locked');
    }

    const encoder = new TextEncoder();
    const data = encoder.encode(plaintext);
    const iv = this.generateIV();

    const ciphertext = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv: iv },
      this.masterKey,
      data
    );

    // Combine IV and ciphertext
    const combined = new Uint8Array(iv.length + ciphertext.byteLength);
    combined.set(iv);
    combined.set(new Uint8Array(ciphertext), iv.length);

    return this.bytesToBase64(combined);
  }

  // Decrypt data
  async decrypt(encryptedBase64) {
    if (!this.masterKey) {
      throw new Error('Vault is locked');
    }

    const combined = this.base64ToBytes(encryptedBase64);
    const iv = combined.slice(0, 12);
    const ciphertext = combined.slice(12);

    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv: iv },
      this.masterKey,
      ciphertext
    );

    const decoder = new TextDecoder();
    return decoder.decode(decrypted);
  }

  // Encrypt vault credentials
  async encryptCredentials(credentials) {
    const encrypted = [];
    for (const cred of credentials) {
      encrypted.push({
        ...cred,
        password: await this.encrypt(cred.password),
        // Optionally encrypt other sensitive fields
        username: cred.username, // Keep username readable for search
        notes: cred.notes ? await this.encrypt(cred.notes) : null,
      });
    }
    return encrypted;
  }

  // Decrypt vault credentials
  async decryptCredentials(encryptedCredentials) {
    const decrypted = [];
    for (const cred of encryptedCredentials) {
      try {
        decrypted.push({
          ...cred,
          password: await this.decrypt(cred.password),
          notes: cred.notes ? await this.decrypt(cred.notes) : null,
        });
      } catch (error) {
        console.error('Failed to decrypt credential:', cred.id, error);
        // Skip credentials that fail to decrypt
      }
    }
    return decrypted;
  }

  // Utility: Convert bytes to base64
  bytesToBase64(bytes) {
    let binary = '';
    const len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
  }

  // Utility: Convert base64 to bytes
  base64ToBytes(base64) {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  }

  // Generate a secure random password
  generatePassword(options = {}) {
    const {
      length = 16,
      uppercase = true,
      lowercase = true,
      numbers = true,
      symbols = true,
      excludeSimilar = true
    } = options;

    let charset = '';
    if (lowercase) charset += 'abcdefghijklmnopqrstuvwxyz';
    if (uppercase) charset += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    if (numbers) charset += '0123456789';
    if (symbols) charset += '!@#$%^&*()_+-=[]{}|;:,.<>?';

    if (excludeSimilar) {
      charset = charset.replace(/[0O1lI]/g, '');
    }

    const randomValues = crypto.getRandomValues(new Uint32Array(length));
    let password = '';
    for (let i = 0; i < length; i++) {
      password += charset[randomValues[i] % charset.length];
    }

    return password;
  }
}

// Singleton instance
const cryptoService = new CloistrVaultCrypto();
export default cryptoService;
