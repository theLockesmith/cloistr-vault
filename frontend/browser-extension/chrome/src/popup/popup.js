// Cloistr Vault Browser Extension - Popup Script

class CloistrVaultPopup {
  constructor() {
    this.isUnlocked = false;
    this.passwords = [];
    this.filteredPasswords = [];
    this.selectedPassword = null;

    this.init();
  }

  async init() {
    await this.checkVaultStatus();
    this.bindEvents();

    if (this.isUnlocked) {
      await this.loadPasswords();
    }
  }

  async checkVaultStatus() {
    try {
      const response = await this.sendMessage({ type: 'GET_VAULT_STATUS' });

      if (response.success) {
        this.isUnlocked = response.status.unlocked;
        this.updateUI();
      }
    } catch (error) {
      console.error('Error checking vault status:', error);
    }
  }

  updateUI() {
    const unlockScreen = document.getElementById('unlockScreen');
    const mainScreen = document.getElementById('mainScreen');
    const statusIndicator = document.getElementById('statusIndicator');

    if (this.isUnlocked) {
      unlockScreen.classList.add('hidden');
      mainScreen.classList.remove('hidden');
      statusIndicator.classList.add('unlocked');
    } else {
      unlockScreen.classList.remove('hidden');
      mainScreen.classList.add('hidden');
      statusIndicator.classList.remove('unlocked');
    }
  }

  bindEvents() {
    // Unlock form
    document.getElementById('unlockForm').addEventListener('submit', (e) => {
      e.preventDefault();
      this.handleUnlock();
    });

    // Toggle password visibility
    document.getElementById('togglePassword').addEventListener('click', () => {
      const input = document.getElementById('masterPassword');
      input.type = input.type === 'password' ? 'text' : 'password';
    });

    // Open web app
    document.getElementById('openWebApp').addEventListener('click', (e) => {
      e.preventDefault();
      chrome.tabs.create({ url: 'https://vault.cloistr.xyz' });
    });

    // Forgot password
    document.getElementById('forgotPassword').addEventListener('click', (e) => {
      e.preventDefault();
      chrome.tabs.create({ url: 'https://vault.cloistr.xyz/recovery' });
    });

    // Search
    document.getElementById('searchInput').addEventListener('input', (e) => {
      this.filterPasswords(e.target.value);
    });

    // Quick actions
    document.getElementById('fillBtn').addEventListener('click', () => this.fillCurrentPage());
    document.getElementById('generateBtn').addEventListener('click', () => this.showGenerateModal());
    document.getElementById('addBtn').addEventListener('click', () => this.showAddModal());

    // Footer
    document.getElementById('settingsBtn').addEventListener('click', () => this.openSettings());
    document.getElementById('lockBtn').addEventListener('click', () => this.lockVault());

    // Modal
    document.getElementById('closeModal').addEventListener('click', () => this.closeModal());
    document.getElementById('fillFromModal').addEventListener('click', () => this.fillFromModal());
    document.getElementById('toggleModalPassword').addEventListener('click', () => this.toggleModalPassword());

    // Copy buttons
    document.querySelectorAll('.copy-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        this.copyField(e.target.dataset.field);
      });
    });
  }

  async handleUnlock() {
    const masterPassword = document.getElementById('masterPassword').value;

    if (!masterPassword) {
      this.showToast('Please enter your master password', 'error');
      return;
    }

    this.showLoading(true);

    try {
      const response = await this.sendMessage({
        type: 'UNLOCK_VAULT',
        masterPassword
      });

      if (response.success) {
        this.isUnlocked = true;
        this.updateUI();
        await this.loadPasswords();
        this.showToast('Vault unlocked', 'success');
      } else {
        this.showToast('Invalid master password', 'error');
        document.getElementById('masterPassword').value = '';
        document.getElementById('masterPassword').focus();
      }
    } catch (error) {
      this.showToast('Failed to unlock vault', 'error');
      console.error('Unlock error:', error);
    } finally {
      this.showLoading(false);
    }
  }

  async loadPasswords() {
    try {
      // Get current tab domain
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const currentDomain = tab?.url ? new URL(tab.url).hostname : '';

      // Get all credentials
      const result = await chrome.storage.local.get(['credentials']);
      this.passwords = result.credentials || [];

      // Sort: matching domain first, then alphabetically
      this.passwords.sort((a, b) => {
        const aMatches = a.domain === currentDomain;
        const bMatches = b.domain === currentDomain;

        if (aMatches && !bMatches) return -1;
        if (!aMatches && bMatches) return 1;

        return a.domain.localeCompare(b.domain);
      });

      this.filteredPasswords = [...this.passwords];
      this.renderPasswordList();
    } catch (error) {
      console.error('Error loading passwords:', error);
    }
  }

  filterPasswords(query) {
    const lowerQuery = query.toLowerCase();

    this.filteredPasswords = this.passwords.filter(p =>
      p.domain.toLowerCase().includes(lowerQuery) ||
      p.username.toLowerCase().includes(lowerQuery)
    );

    this.renderPasswordList();
  }

  renderPasswordList() {
    const list = document.getElementById('passwordList');
    const noResults = document.getElementById('noResults');

    if (this.filteredPasswords.length === 0) {
      list.innerHTML = '';
      noResults.classList.remove('hidden');
      return;
    }

    noResults.classList.add('hidden');

    list.innerHTML = this.filteredPasswords.map(password => `
      <div class="password-item" data-id="${password.id}">
        <div class="favicon">${this.getFavicon(password.domain)}</div>
        <div class="details">
          <div class="site-name">${this.escapeHtml(password.domain)}</div>
          <div class="username">${this.escapeHtml(password.username)}</div>
        </div>
        <div class="actions">
          <button class="action-icon copy-password" title="Copy password">📋</button>
          <button class="action-icon fill-password" title="Fill password">🔑</button>
        </div>
      </div>
    `).join('');

    // Bind click events
    list.querySelectorAll('.password-item').forEach(item => {
      item.addEventListener('click', (e) => {
        // Ignore if clicking action buttons
        if (e.target.closest('.actions')) return;

        const id = item.dataset.id;
        const password = this.passwords.find(p => p.id === id);
        if (password) {
          this.showPasswordDetail(password);
        }
      });
    });

    list.querySelectorAll('.copy-password').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const id = btn.closest('.password-item').dataset.id;
        const password = this.passwords.find(p => p.id === id);
        if (password) {
          await this.copyToClipboard(atob(password.password));
          this.showToast('Password copied', 'success');
        }
      });
    });

    list.querySelectorAll('.fill-password').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const id = btn.closest('.password-item').dataset.id;
        const password = this.passwords.find(p => p.id === id);
        if (password) {
          await this.fillPassword(password);
        }
      });
    });
  }

  getFavicon(domain) {
    // Return first letter as fallback
    return domain.charAt(0).toUpperCase();
  }

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  showPasswordDetail(password) {
    this.selectedPassword = password;

    document.getElementById('modalTitle').textContent = password.domain;
    document.getElementById('modalWebsite').textContent = password.url || password.domain;
    document.getElementById('modalUsername').textContent = password.username;
    document.getElementById('modalPassword').textContent = '••••••••';
    document.getElementById('modalPassword').dataset.password = password.password;
    document.getElementById('modalPassword').classList.add('password-hidden');

    document.getElementById('passwordModal').classList.remove('hidden');
  }

  closeModal() {
    document.getElementById('passwordModal').classList.add('hidden');
    this.selectedPassword = null;
  }

  toggleModalPassword() {
    const passwordSpan = document.getElementById('modalPassword');
    const isHidden = passwordSpan.classList.contains('password-hidden');

    if (isHidden) {
      passwordSpan.textContent = atob(passwordSpan.dataset.password);
      passwordSpan.classList.remove('password-hidden');
    } else {
      passwordSpan.textContent = '••••••••';
      passwordSpan.classList.add('password-hidden');
    }
  }

  async fillFromModal() {
    if (this.selectedPassword) {
      await this.fillPassword(this.selectedPassword);
      this.closeModal();
    }
  }

  async fillPassword(credential) {
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

      if (!tab?.id) {
        this.showToast('Cannot access this page', 'error');
        return;
      }

      await chrome.tabs.sendMessage(tab.id, {
        type: 'FILL_PASSWORD',
        data: {
          username: credential.username,
          password: atob(credential.password)
        }
      });

      // Close popup after filling
      window.close();
    } catch (error) {
      this.showToast('Failed to fill password', 'error');
      console.error('Fill error:', error);
    }
  }

  async fillCurrentPage() {
    try {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

      if (!tab?.url) {
        this.showToast('Cannot access this page', 'error');
        return;
      }

      const domain = new URL(tab.url).hostname;
      const credentials = this.passwords.filter(p => p.domain === domain);

      if (credentials.length === 0) {
        this.showToast('No saved passwords for this site', 'info');
        return;
      }

      // Fill with first matching credential
      await this.fillPassword(credentials[0]);
    } catch (error) {
      this.showToast('Failed to fill password', 'error');
    }
  }

  async showGenerateModal() {
    // Generate a password and copy to clipboard
    try {
      const response = await this.sendMessage({
        type: 'GENERATE_PASSWORD',
        options: { length: 16, symbols: true }
      });

      if (response.success) {
        await this.copyToClipboard(response.password);
        this.showToast('Password generated and copied', 'success');
      }
    } catch (error) {
      this.showToast('Failed to generate password', 'error');
    }
  }

  showAddModal() {
    // Open web app to add new password
    chrome.tabs.create({ url: 'https://vault.cloistr.xyz/vault/add' });
  }

  openSettings() {
    chrome.tabs.create({ url: 'https://vault.cloistr.xyz/settings' });
  }

  async lockVault() {
    try {
      await this.sendMessage({ type: 'LOCK_VAULT' });
      this.isUnlocked = false;
      this.passwords = [];
      this.filteredPasswords = [];
      this.updateUI();
      document.getElementById('masterPassword').value = '';
      this.showToast('Vault locked', 'success');
    } catch (error) {
      this.showToast('Failed to lock vault', 'error');
    }
  }

  async copyField(field) {
    let value = '';

    switch (field) {
      case 'website':
        value = document.getElementById('modalWebsite').textContent;
        break;
      case 'username':
        value = document.getElementById('modalUsername').textContent;
        break;
      case 'password':
        const passwordSpan = document.getElementById('modalPassword');
        value = atob(passwordSpan.dataset.password);
        break;
    }

    if (value) {
      await this.copyToClipboard(value);
      this.showToast(`${field.charAt(0).toUpperCase() + field.slice(1)} copied`, 'success');
    }
  }

  async copyToClipboard(text) {
    try {
      await navigator.clipboard.writeText(text);
    } catch (error) {
      // Fallback for older browsers
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  }

  showLoading(show) {
    document.getElementById('loading').classList.toggle('hidden', !show);
  }

  showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type}`;

    // Show toast
    setTimeout(() => {
      toast.classList.add('hidden');
    }, 3000);
  }

  sendMessage(message) {
    return new Promise((resolve, reject) => {
      chrome.runtime.sendMessage(message, (response) => {
        if (chrome.runtime.lastError) {
          reject(chrome.runtime.lastError);
        } else {
          resolve(response);
        }
      });
    });
  }
}

// Initialize popup
document.addEventListener('DOMContentLoaded', () => {
  new CloistrVaultPopup();
});
