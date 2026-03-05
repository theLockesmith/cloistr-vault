// Cloistr Vault Browser Extension - Background Script
// Connects to vault.cloistr.xyz backend

const API_BASE_URL = 'https://vault.cloistr.xyz/api/v1';

class CloistrVaultBackground {
  constructor() {
    this.token = null;
    this.masterKey = null;
    this.salt = null;
    this.credentials = [];
    this.initializeExtension();
  }

  async initializeExtension() {
    // Load saved state
    await this.loadState();

    // Handle extension installation
    chrome.runtime.onInstalled.addListener((details) => {
      this.onInstalled(details);
    });

    // Handle messages from content scripts and popup
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      this.handleMessage(message, sender, sendResponse);
      return true; // Keep message channel open for async responses
    });

    // Handle context menu
    this.createContextMenus();

    // Handle tab updates for form detection
    chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
      this.onTabUpdated(tabId, changeInfo, tab);
    });

    // Handle alarms for auto-lock
    chrome.alarms.onAlarm.addListener((alarm) => {
      if (alarm.name === 'autoLock') {
        this.lockVault();
      }
    });

    console.log('Cloistr Vault background script initialized');
  }

  async loadState() {
    const result = await chrome.storage.local.get(['authToken', 'salt']);
    this.token = result.authToken || null;
    this.salt = result.salt || null;
  }

  async saveToken(token) {
    this.token = token;
    await chrome.storage.local.set({ authToken: token });
  }

  async clearToken() {
    this.token = null;
    await chrome.storage.local.remove(['authToken']);
  }

  onInstalled(details) {
    if (details.reason === 'install') {
      this.setDefaultSettings();
      // Don't open welcome page - the popup handles first-time setup
    } else if (details.reason === 'update') {
      console.log('Cloistr Vault updated to version', chrome.runtime.getManifest().version);
    }
  }

  setDefaultSettings() {
    const defaultSettings = {
      autoFill: true,
      autoSave: true,
      showNotifications: true,
      lockTimeout: 15, // minutes
      theme: 'auto'
    };

    chrome.storage.sync.set({ settings: defaultSettings }, () => {
      console.log('Default settings saved');
    });
  }

  createContextMenus() {
    chrome.contextMenus.create({
      id: 'fill-password',
      title: 'Fill password',
      contexts: ['editable'],
    });

    chrome.contextMenus.create({
      id: 'generate-password',
      title: 'Generate password',
      contexts: ['editable'],
    });

    chrome.contextMenus.create({
      id: 'open-vault',
      title: 'Open Cloistr Vault',
      contexts: ['page'],
    });

    chrome.contextMenus.onClicked.addListener((info, tab) => {
      this.handleContextMenuClick(info, tab);
    });
  }

  handleContextMenuClick(info, tab) {
    switch (info.menuItemId) {
      case 'fill-password':
        this.fillPassword(tab);
        break;
      case 'generate-password':
        this.generateAndFillPassword(tab);
        break;
      case 'open-vault':
        chrome.action.openPopup();
        break;
    }
  }

  async handleMessage(message, sender, sendResponse) {
    try {
      switch (message.type) {
        case 'LOGIN':
          const loginResult = await this.login(message.email, message.password);
          sendResponse(loginResult);
          break;

        case 'UNLOCK_VAULT':
          const unlocked = await this.unlockVault(message.masterPassword);
          sendResponse({ success: unlocked });
          break;

        case 'LOCK_VAULT':
          await this.lockVault();
          sendResponse({ success: true });
          break;

        case 'GET_VAULT_STATUS':
          const status = await this.getVaultStatus();
          sendResponse({ success: true, status });
          break;

        case 'SYNC_VAULT':
          await this.syncVault();
          sendResponse({ success: true });
          break;

        case 'GET_CREDENTIALS':
          const credentials = await this.getCredentials(message.domain);
          sendResponse({ success: true, data: credentials });
          break;

        case 'SAVE_CREDENTIALS':
          await this.saveCredentials(message.data);
          sendResponse({ success: true });
          break;

        case 'GENERATE_PASSWORD':
          const password = this.generatePassword(message.options || {});
          sendResponse({ success: true, password });
          break;

        case 'DETECT_FORMS':
          await this.detectForms(sender.tab);
          sendResponse({ success: true });
          break;

        default:
          sendResponse({ success: false, error: 'Unknown message type' });
      }
    } catch (error) {
      console.error('Background script error:', error);
      sendResponse({ success: false, error: error.message });
    }
  }

  // API Request helper
  async apiRequest(endpoint, options = {}) {
    const url = `${API_BASE_URL}${endpoint}`;
    const headers = {
      'Content-Type': 'application/json',
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(url, {
      ...options,
      headers: {
        ...headers,
        ...options.headers,
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Request failed' }));
      throw new Error(error.message || `HTTP ${response.status}`);
    }

    return response.json();
  }

  // Authentication
  async login(email, password) {
    try {
      const response = await this.apiRequest('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });

      if (response.token) {
        await this.saveToken(response.token);
        return { success: true, user: response.user };
      }

      return { success: false, error: 'No token received' };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  async logout() {
    try {
      if (this.token) {
        await this.apiRequest('/auth/logout', { method: 'POST' });
      }
    } finally {
      await this.clearToken();
      await this.lockVault();
    }
  }

  // Vault operations
  async unlockVault(masterPassword) {
    if (!this.token) {
      throw new Error('Not authenticated');
    }

    try {
      // Derive key from master password
      const salt = this.salt || this.generateSalt();
      this.masterKey = await this.deriveKey(masterPassword, salt);

      if (!this.salt) {
        this.salt = salt;
        await chrome.storage.local.set({ salt: this.bytesToBase64(salt) });
      }

      // Try to fetch and decrypt vault to verify password
      await this.syncVault();

      // Store unlock state
      await chrome.storage.session.set({ vaultUnlocked: true, unlockedAt: Date.now() });
      this.scheduleAutoLock();

      return true;
    } catch (error) {
      console.error('Unlock failed:', error);
      this.masterKey = null;
      return false;
    }
  }

  async lockVault() {
    this.masterKey = null;
    this.credentials = [];
    await chrome.storage.session.set({ vaultUnlocked: false });
    chrome.alarms.clear('autoLock');
  }

  async getVaultStatus() {
    const result = await chrome.storage.session.get(['vaultUnlocked']);
    return {
      unlocked: result.vaultUnlocked || false,
      authenticated: !!this.token,
      hasData: this.credentials.length > 0
    };
  }

  async syncVault() {
    if (!this.token || !this.masterKey) {
      throw new Error('Vault is locked');
    }

    try {
      const response = await this.apiRequest('/vault');

      if (response.data) {
        // Decrypt vault data
        this.credentials = await this.decryptVaultData(response.data);
        // Cache credentials locally
        await chrome.storage.local.set({ cachedCredentials: this.credentials });
      }
    } catch (error) {
      console.error('Sync failed:', error);
      // Fall back to cached credentials
      const cached = await chrome.storage.local.get(['cachedCredentials']);
      this.credentials = cached.cachedCredentials || [];
    }
  }

  async getCredentials(domain) {
    if (!this.masterKey) {
      return [];
    }

    return this.credentials.filter(c =>
      c.domain === domain ||
      c.domain.endsWith('.' + domain) ||
      domain.endsWith('.' + c.domain)
    );
  }

  async saveCredentials(data) {
    const { domain, username, password, url } = data;

    const entry = {
      id: this.generateId(),
      domain,
      username,
      password: await this.encrypt(password),
      url,
      createdAt: Date.now(),
      updatedAt: Date.now()
    };

    // Check if entry already exists
    const existingIndex = this.credentials.findIndex(
      c => c.domain === domain && c.username === username
    );

    if (existingIndex !== -1) {
      entry.id = this.credentials[existingIndex].id;
      entry.createdAt = this.credentials[existingIndex].createdAt;
      this.credentials[existingIndex] = entry;
    } else {
      this.credentials.push(entry);
    }

    // Sync to backend
    await this.pushVault();

    this.showNotification('Credentials saved', `Saved login for ${domain}`);
  }

  async pushVault() {
    if (!this.token || !this.masterKey) {
      throw new Error('Vault is locked');
    }

    const encryptedData = await this.encryptVaultData(this.credentials);

    await this.apiRequest('/vault', {
      method: 'PUT',
      body: JSON.stringify({ data: encryptedData }),
    });
  }

  // Encryption helpers
  async deriveKey(password, salt) {
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);

    const keyMaterial = await crypto.subtle.importKey(
      'raw',
      passwordBuffer,
      'PBKDF2',
      false,
      ['deriveBits', 'deriveKey']
    );

    return crypto.subtle.deriveKey(
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
  }

  generateSalt() {
    return crypto.getRandomValues(new Uint8Array(32));
  }

  generateIV() {
    return crypto.getRandomValues(new Uint8Array(12));
  }

  async encrypt(plaintext) {
    if (!this.masterKey) throw new Error('Vault is locked');

    const encoder = new TextEncoder();
    const data = encoder.encode(plaintext);
    const iv = this.generateIV();

    const ciphertext = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv: iv },
      this.masterKey,
      data
    );

    const combined = new Uint8Array(iv.length + ciphertext.byteLength);
    combined.set(iv);
    combined.set(new Uint8Array(ciphertext), iv.length);

    return this.bytesToBase64(combined);
  }

  async decrypt(encryptedBase64) {
    if (!this.masterKey) throw new Error('Vault is locked');

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

  async encryptVaultData(credentials) {
    const encrypted = [];
    for (const cred of credentials) {
      encrypted.push({
        ...cred,
        password: typeof cred.password === 'string' && !cred.password.startsWith('eyJ')
          ? await this.encrypt(cred.password)
          : cred.password,
      });
    }
    return JSON.stringify(encrypted);
  }

  async decryptVaultData(encryptedData) {
    const credentials = JSON.parse(encryptedData);
    const decrypted = [];

    for (const cred of credentials) {
      try {
        decrypted.push({
          ...cred,
          password: await this.decrypt(cred.password),
        });
      } catch (error) {
        console.error('Failed to decrypt credential:', cred.id);
      }
    }

    return decrypted;
  }

  bytesToBase64(bytes) {
    let binary = '';
    const len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
  }

  base64ToBytes(base64) {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  }

  // Password generation
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

  // Auto-lock scheduling
  scheduleAutoLock() {
    chrome.storage.sync.get(['settings'], (result) => {
      const settings = result.settings || {};
      const lockTimeout = settings.lockTimeout || 15;
      chrome.alarms.create('autoLock', { delayInMinutes: lockTimeout });
    });
  }

  // Form detection
  async detectForms(tab) {
    if (!tab || !tab.id) return;

    try {
      await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => {
          const forms = document.querySelectorAll('form');
          const loginForms = [];

          forms.forEach((form, index) => {
            const passwordFields = form.querySelectorAll('input[type="password"]');
            const emailFields = form.querySelectorAll(
              'input[type="email"], input[type="text"][name*="email"], input[type="text"][name*="username"]'
            );

            if (passwordFields.length > 0) {
              loginForms.push({
                formIndex: index,
                hasEmail: emailFields.length > 0,
                passwordFields: passwordFields.length,
                action: form.action || window.location.href
              });
            }
          });

          if (loginForms.length > 0) {
            chrome.runtime.sendMessage({
              type: 'FORMS_DETECTED',
              forms: loginForms,
              domain: window.location.hostname
            });
          }
        }
      });
    } catch (error) {
      console.error('Error detecting forms:', error);
    }
  }

  onTabUpdated(tabId, changeInfo, tab) {
    if (changeInfo.status === 'complete' && tab.url) {
      this.detectForms(tab);
    }
  }

  // Utilities
  generateId() {
    return 'id_' + crypto.getRandomValues(new Uint32Array(4))
      .reduce((acc, val) => acc + val.toString(36), '');
  }

  showNotification(title, message) {
    chrome.storage.sync.get(['settings'], (result) => {
      const settings = result.settings || {};
      if (settings.showNotifications) {
        chrome.notifications.create({
          type: 'basic',
          iconUrl: 'icons/icon48.png',
          title,
          message
        });
      }
    });
  }
}

// Initialize the background script
new CloistrVaultBackground();
