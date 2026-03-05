// Cloistr Vault Browser Extension - Background Script (Firefox)

class CloistrVaultBackground {
  constructor() {
    this.initializeExtension();
  }

  initializeExtension() {
    // Handle extension installation
    browser.runtime.onInstalled.addListener((details) => {
      this.onInstalled(details);
    });

    // Handle messages from content scripts and popup
    browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
      this.handleMessage(message, sender, sendResponse);
      return true; // Keep message channel open for async responses
    });

    // Handle context menu
    this.createContextMenus();

    // Handle tab updates for form detection
    browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
      this.onTabUpdated(tabId, changeInfo, tab);
    });

    console.log('Cloistr Vault background script initialized');
  }

  onInstalled(details) {
    if (details.reason === 'install') {
      // First time installation
      this.setDefaultSettings();
      browser.tabs.create({ url: browser.runtime.getURL('src/popup/welcome.html') });
    } else if (details.reason === 'update') {
      // Extension updated
      console.log('Cloistr Vault updated to version', browser.runtime.getManifest().version);
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

    browser.storage.sync.set({ settings: defaultSettings }).then(() => {
      console.log('Default settings saved');
    });
  }

  createContextMenus() {
    browser.contextMenus.create({
      id: 'fill-password',
      title: 'Fill password',
      contexts: ['editable'],
    });

    browser.contextMenus.create({
      id: 'generate-password',
      title: 'Generate password',
      contexts: ['editable'],
    });

    browser.contextMenus.create({
      id: 'open-vault',
      title: 'Open Cloistr Vault',
      contexts: ['page'],
    });

    browser.contextMenus.onClicked.addListener((info, tab) => {
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
        browser.browserAction.openPopup();
        break;
    }
  }

  async handleMessage(message, sender, sendResponse) {
    try {
      switch (message.type) {
        case 'DETECT_FORMS':
          await this.detectForms(sender.tab);
          sendResponse({ success: true });
          break;

        case 'SAVE_CREDENTIALS':
          await this.saveCredentials(message.data);
          sendResponse({ success: true });
          break;

        case 'GET_CREDENTIALS':
          const credentials = await this.getCredentials(message.domain);
          sendResponse({ success: true, data: credentials });
          break;

        case 'GENERATE_PASSWORD':
          const password = this.generatePassword(message.options || {});
          sendResponse({ success: true, password });
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

        default:
          sendResponse({ success: false, error: 'Unknown message type' });
      }
    } catch (error) {
      console.error('Background script error:', error);
      sendResponse({ success: false, error: error.message });
    }
  }

  async detectForms(tab) {
    if (!tab || !tab.id) return;

    try {
      await browser.tabs.executeScript(tab.id, {
        code: `
          (function() {
            const forms = document.querySelectorAll('form');
            const loginForms = [];

            forms.forEach((form, index) => {
              const passwordFields = form.querySelectorAll('input[type="password"]');
              const emailFields = form.querySelectorAll('input[type="email"], input[type="text"][name*="email"], input[type="text"][name*="username"]');

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
              browser.runtime.sendMessage({
                type: 'FORMS_DETECTED',
                forms: loginForms,
                domain: window.location.hostname
              });
            }
          })();
        `
      });
    } catch (error) {
      console.error('Error detecting forms:', error);
    }
  }

  async saveCredentials(data) {
    const { domain, username, password, url } = data;

    // In a real implementation, this would encrypt the data
    const entry = {
      id: this.generateId(),
      domain,
      username,
      password: await this.encryptPassword(password),
      url,
      createdAt: Date.now(),
      updatedAt: Date.now()
    };

    // Get existing credentials
    const result = await browser.storage.local.get(['credentials']);
    const credentials = result.credentials || [];

    // Check if entry already exists
    const existingIndex = credentials.findIndex(c => c.domain === domain && c.username === username);

    if (existingIndex !== -1) {
      credentials[existingIndex] = entry;
    } else {
      credentials.push(entry);
    }

    await browser.storage.local.set({ credentials });

    // Show notification
    this.showNotification('Credentials saved', `Saved login for ${domain}`);
  }

  async getCredentials(domain) {
    const result = await browser.storage.local.get(['credentials']);
    const credentials = result.credentials || [];

    return credentials.filter(c => c.domain === domain);
  }

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

    let password = '';
    for (let i = 0; i < length; i++) {
      password += charset.charAt(Math.floor(Math.random() * charset.length));
    }

    return password;
  }

  async encryptPassword(password) {
    // In a real implementation, this would use proper encryption
    // For demo purposes, we'll just base64 encode
    return btoa(password);
  }

  async decryptPassword(encryptedPassword) {
    // In a real implementation, this would use proper decryption
    try {
      return atob(encryptedPassword);
    } catch (error) {
      return '';
    }
  }

  async unlockVault(masterPassword) {
    // In a real implementation, this would verify the master password
    // and decrypt the vault
    const isValid = masterPassword === 'demo123'; // Demo password

    if (isValid) {
      await browser.storage.local.set({ vaultUnlocked: true, unlockedAt: Date.now() });
      this.scheduleAutoLock();
    }

    return isValid;
  }

  async lockVault() {
    await browser.storage.local.set({ vaultUnlocked: false });
    browser.alarms.clear('autoLock');
  }

  async getVaultStatus() {
    const result = await browser.storage.local.get(['vaultUnlocked']);
    return {
      unlocked: result.vaultUnlocked || false,
      hasData: true // In real app, check if vault has data
    };
  }

  scheduleAutoLock() {
    browser.storage.sync.get(['settings']).then((result) => {
      const settings = result.settings || {};
      const lockTimeout = settings.lockTimeout || 15; // minutes

      browser.alarms.create('autoLock', { delayInMinutes: lockTimeout });
    });
  }

  onTabUpdated(tabId, changeInfo, tab) {
    if (changeInfo.status === 'complete' && tab.url) {
      // Inject content script if needed
      this.detectForms(tab);
    }
  }

  generateId() {
    return 'id_' + Math.random().toString(36).substr(2, 9);
  }

  showNotification(title, message) {
    browser.storage.sync.get(['settings']).then((result) => {
      const settings = result.settings || {};
      if (settings.showNotifications) {
        browser.notifications.create({
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

// Handle alarms
browser.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'autoLock') {
    browser.storage.local.set({ vaultUnlocked: false });
  }
});
