// Cloistr Vault Browser Extension - API Service

const API_BASE_URL = 'https://vault.cloistr.xyz/api/v1';

class CloistrVaultAPI {
  constructor() {
    this.token = null;
  }

  async init() {
    // Load token from storage
    const result = await chrome.storage.local.get(['authToken']);
    this.token = result.authToken || null;
  }

  async setToken(token) {
    this.token = token;
    await chrome.storage.local.set({ authToken: token });
  }

  async clearToken() {
    this.token = null;
    await chrome.storage.local.remove(['authToken']);
  }

  getHeaders() {
    const headers = {
      'Content-Type': 'application/json',
    };
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }
    return headers;
  }

  async request(endpoint, options = {}) {
    const url = `${API_BASE_URL}${endpoint}`;
    const response = await fetch(url, {
      ...options,
      headers: {
        ...this.getHeaders(),
        ...options.headers,
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Request failed' }));
      throw new Error(error.message || `HTTP ${response.status}`);
    }

    return response.json();
  }

  // Health check
  async healthCheck() {
    return this.request('/health');
  }

  // Authentication
  async login(email, password) {
    const response = await this.request('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });

    if (response.token) {
      await this.setToken(response.token);
    }

    return response;
  }

  async logout() {
    try {
      await this.request('/auth/logout', { method: 'POST' });
    } finally {
      await this.clearToken();
    }
  }

  async register(email, password) {
    return this.request('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  // WebAuthn/Passkey authentication
  async webauthnBeginLogin(email = null) {
    const body = email ? { email } : {};
    return this.request('/auth/webauthn/login/begin', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  async webauthnBeginDiscoverableLogin() {
    return this.request('/auth/webauthn/login/begin/discoverable', {
      method: 'POST',
    });
  }

  async webauthnFinishLogin(credential) {
    const response = await this.request('/auth/webauthn/login/finish', {
      method: 'POST',
      body: JSON.stringify(credential),
    });

    if (response.token) {
      await this.setToken(response.token);
    }

    return response;
  }

  // User profile
  async getProfile() {
    return this.request('/user/profile');
  }

  // Vault operations
  async getVault() {
    return this.request('/vault');
  }

  async updateVault(vaultData) {
    return this.request('/vault', {
      method: 'PUT',
      body: JSON.stringify(vaultData),
    });
  }

  // Recovery
  async getRecoveryStatus() {
    return this.request('/recovery/status');
  }

  async recoverAccount(email, recoveryCode, newPassword) {
    return this.request('/auth/recover', {
      method: 'POST',
      body: JSON.stringify({ email, recovery_code: recoveryCode, new_password: newPassword }),
    });
  }

  // Check if authenticated
  isAuthenticated() {
    return !!this.token;
  }
}

// Singleton instance
const api = new CloistrVaultAPI();
export default api;
