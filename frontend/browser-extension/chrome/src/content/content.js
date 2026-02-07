// Coldforge Vault Browser Extension - Content Script

class ColdforgeVaultContent {
  constructor() {
    this.isInitialized = false;
    this.formDetector = null;
    this.fillIndicators = new Map();
    
    this.initialize();
  }

  initialize() {
    if (this.isInitialized) return;
    
    // Wait for DOM to be ready
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', () => this.setup());
    } else {
      this.setup();
    }
  }

  setup() {
    this.isInitialized = true;
    
    // Detect forms on page load
    this.detectForms();
    
    // Monitor for dynamically added forms
    this.observeDOM();
    
    // Listen for messages from background script
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      this.handleMessage(message, sender, sendResponse);
    });
    
    console.log('Coldforge Vault content script initialized on', window.location.hostname);
  }

  detectForms() {
    const forms = document.querySelectorAll('form');
    const loginForms = [];

    forms.forEach((form, index) => {
      const passwordFields = form.querySelectorAll('input[type="password"]');
      const usernameFields = form.querySelectorAll(
        'input[type="email"], input[type="text"][name*="email"], input[type="text"][name*="username"], input[type="text"][name*="user"]'
      );

      if (passwordFields.length > 0) {
        this.addFillIndicators(form, usernameFields, passwordFields);
        
        loginForms.push({
          formIndex: index,
          hasUsername: usernameFields.length > 0,
          passwordFields: passwordFields.length,
          action: form.action || window.location.href,
          method: form.method || 'POST'
        });
      }
    });

    if (loginForms.length > 0) {
      // Notify background script
      chrome.runtime.sendMessage({
        type: 'FORMS_DETECTED',
        forms: loginForms,
        domain: window.location.hostname,
        url: window.location.href
      });
    }
  }

  addFillIndicators(form, usernameFields, passwordFields) {
    // Add fill buttons next to password fields
    passwordFields.forEach(field => {
      if (this.fillIndicators.has(field)) return;
      
      const indicator = this.createFillIndicator();
      this.positionIndicator(indicator, field);
      
      indicator.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        this.showPasswordOptions(field, usernameFields[0]);
      });
      
      document.body.appendChild(indicator);
      this.fillIndicators.set(field, indicator);
    });

    // Monitor field position changes
    this.monitorFieldPositions();
  }

  createFillIndicator() {
    const indicator = document.createElement('div');
    indicator.className = 'cv-fill-indicator';
    indicator.innerHTML = '🔑';
    indicator.title = 'Fill with Coldforge Vault';
    
    // Styles
    Object.assign(indicator.style, {
      position: 'absolute',
      width: '24px',
      height: '24px',
      backgroundColor: '#2563eb',
      color: 'white',
      borderRadius: '12px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: '12px',
      cursor: 'pointer',
      zIndex: '2147483647',
      boxShadow: '0 2px 8px rgba(0,0,0,0.3)',
      border: '2px solid white',
      transition: 'transform 0.2s ease'
    });

    indicator.addEventListener('mouseenter', () => {
      indicator.style.transform = 'scale(1.1)';
    });

    indicator.addEventListener('mouseleave', () => {
      indicator.style.transform = 'scale(1)';
    });

    return indicator;
  }

  positionIndicator(indicator, field) {
    const rect = field.getBoundingClientRect();
    const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
    const scrollLeft = window.pageXOffset || document.documentElement.scrollLeft;
    
    indicator.style.top = `${rect.top + scrollTop + rect.height / 2 - 12}px`;
    indicator.style.left = `${rect.right + scrollLeft - 30}px`;
  }

  monitorFieldPositions() {
    // Reposition indicators when page layout changes
    const repositionAll = () => {
      this.fillIndicators.forEach((indicator, field) => {
        if (document.contains(field)) {
          this.positionIndicator(indicator, field);
        } else {
          // Field no longer exists, remove indicator
          indicator.remove();
          this.fillIndicators.delete(field);
        }
      });
    };

    let resizeTimeout;
    window.addEventListener('resize', () => {
      clearTimeout(resizeTimeout);
      resizeTimeout = setTimeout(repositionAll, 100);
    });

    window.addEventListener('scroll', repositionAll, { passive: true });
  }

  showPasswordOptions(passwordField, usernameField) {
    // Create floating menu
    const menu = document.createElement('div');
    menu.className = 'cv-password-menu';
    
    menu.innerHTML = `
      <div class="cv-menu-item" data-action="fill">🔑 Fill Password</div>
      <div class="cv-menu-item" data-action="generate">⚡ Generate & Fill</div>
      <div class="cv-menu-item" data-action="save">💾 Save Current</div>
    `;

    // Position menu
    const rect = passwordField.getBoundingClientRect();
    Object.assign(menu.style, {
      position: 'absolute',
      top: `${rect.bottom + window.pageYOffset + 5}px`,
      left: `${rect.left + window.pageXOffset}px`,
      backgroundColor: 'white',
      border: '1px solid #e2e8f0',
      borderRadius: '8px',
      boxShadow: '0 4px 16px rgba(0,0,0,0.15)',
      zIndex: '2147483647',
      minWidth: '160px',
      overflow: 'hidden'
    });

    // Add event listeners
    menu.addEventListener('click', (e) => {
      const target = e.target as HTMLElement;
      const action = target.dataset.action;
      
      if (action) {
        this.handlePasswordAction(action, passwordField, usernameField);
        menu.remove();
      }
    });

    // Remove menu on outside click
    setTimeout(() => {
      document.addEventListener('click', (e) => {
        if (!menu.contains(e.target as Node)) {
          menu.remove();
        }
      }, { once: true });
    }, 100);

    document.body.appendChild(menu);

    // Add menu styles if not present
    this.addMenuStyles();
  }

  addMenuStyles() {
    if (document.getElementById('cv-styles')) return;
    
    const styles = document.createElement('style');
    styles.id = 'cv-styles';
    styles.textContent = `
      .cv-password-menu {
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        font-size: 14px;
      }
      .cv-menu-item {
        padding: 12px 16px;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 8px;
        transition: background-color 0.2s ease;
      }
      .cv-menu-item:hover {
        background-color: #f1f5f9;
      }
      .cv-fill-indicator:hover {
        transform: scale(1.1) !important;
      }
    `;
    
    document.head.appendChild(styles);
  }

  async handlePasswordAction(action, passwordField, usernameField) {
    switch (action) {
      case 'fill':
        await this.fillCredentials(passwordField, usernameField);
        break;
      case 'generate':
        await this.generateAndFill(passwordField);
        break;
      case 'save':
        await this.saveCurrentCredentials(passwordField, usernameField);
        break;
    }
  }

  async fillCredentials(passwordField, usernameField) {
    try {
      const credentials = await this.getStoredCredentials();
      
      if (credentials.length === 0) {
        this.showToast('No saved passwords for this site', 'info');
        return;
      }

      // For demo, use first credential
      const cred = credentials[0];
      
      if (usernameField) {
        usernameField.value = cred.username;
        usernameField.dispatchEvent(new Event('input', { bubbles: true }));
      }
      
      passwordField.value = await this.decryptPassword(cred.password);
      passwordField.dispatchEvent(new Event('input', { bubbles: true }));
      
      this.showToast('Password filled successfully', 'success');
    } catch (error) {
      this.showToast('Failed to fill password', 'error');
      console.error('Fill error:', error);
    }
  }

  async generateAndFill(passwordField) {
    try {
      const response = await chrome.runtime.sendMessage({
        type: 'GENERATE_PASSWORD',
        options: { length: 16, symbols: true }
      });
      
      if (response.success) {
        passwordField.value = response.password;
        passwordField.dispatchEvent(new Event('input', { bubbles: true }));
        this.showToast('Generated password filled', 'success');
      }
    } catch (error) {
      this.showToast('Failed to generate password', 'error');
      console.error('Generate error:', error);
    }
  }

  async saveCurrentCredentials(passwordField, usernameField) {
    try {
      const username = usernameField ? usernameField.value : '';
      const password = passwordField.value;
      
      if (!password) {
        this.showToast('No password to save', 'warning');
        return;
      }

      await chrome.runtime.sendMessage({
        type: 'SAVE_CREDENTIALS',
        data: {
          domain: window.location.hostname,
          username,
          password,
          url: window.location.href
        }
      });

      this.showToast('Credentials saved to vault', 'success');
    } catch (error) {
      this.showToast('Failed to save credentials', 'error');
      console.error('Save error:', error);
    }
  }

  async getStoredCredentials() {
    try {
      const response = await chrome.runtime.sendMessage({
        type: 'GET_CREDENTIALS',
        domain: window.location.hostname
      });
      
      return response.success ? response.data : [];
    } catch (error) {
      console.error('Error getting credentials:', error);
      return [];
    }
  }

  async decryptPassword(encryptedPassword) {
    // In a real implementation, this would decrypt the password
    // For demo, we'll just decode base64
    try {
      return atob(encryptedPassword);
    } catch (error) {
      return encryptedPassword;
    }
  }

  observeDOM() {
    const observer = new MutationObserver((mutations) => {
      let shouldRecheck = false;
      
      mutations.forEach((mutation) => {
        if (mutation.type === 'childList') {
          mutation.addedNodes.forEach((node) => {
            if (node.nodeType === Node.ELEMENT_NODE) {
              const element = node as Element;
              if (element.tagName === 'FORM' || element.querySelector('form')) {
                shouldRecheck = true;
              }
            }
          });
        }
      });

      if (shouldRecheck) {
        setTimeout(() => this.detectForms(), 500);
      }
    });

    observer.observe(document.body, {
      childList: true,
      subtree: true
    });
  }

  showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `cv-toast cv-toast-${type}`;
    toast.textContent = message;
    
    Object.assign(toast.style, {
      position: 'fixed',
      top: '20px',
      right: '20px',
      padding: '12px 16px',
      borderRadius: '8px',
      color: 'white',
      fontSize: '14px',
      fontWeight: '500',
      zIndex: '2147483647',
      opacity: '0',
      transform: 'translateX(100%)',
      transition: 'all 0.3s ease',
      backgroundColor: type === 'success' ? '#10b981' : 
                      type === 'error' ? '#ef4444' : 
                      type === 'warning' ? '#f59e0b' : '#3b82f6'
    });

    document.body.appendChild(toast);
    
    // Animate in
    setTimeout(() => {
      toast.style.opacity = '1';
      toast.style.transform = 'translateX(0)';
    }, 100);

    // Remove after delay
    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateX(100%)';
      setTimeout(() => toast.remove(), 300);
    }, 3000);
  }

  handleMessage(message, sender, sendResponse) {
    switch (message.type) {
      case 'FILL_PASSWORD':
        this.fillPasswordFromBackground(message.data);
        sendResponse({ success: true });
        break;
        
      case 'HIGHLIGHT_FIELDS':
        this.highlightPasswordFields();
        sendResponse({ success: true });
        break;
        
      case 'GET_PAGE_INFO':
        sendResponse({
          success: true,
          data: {
            url: window.location.href,
            domain: window.location.hostname,
            title: document.title,
            hasPasswordFields: document.querySelectorAll('input[type="password"]').length > 0
          }
        });
        break;
        
      default:
        sendResponse({ success: false, error: 'Unknown message type' });
    }
  }

  fillPasswordFromBackground(data) {
    const { username, password } = data;
    
    // Find and fill username field
    if (username) {
      const usernameField = document.querySelector(
        'input[type="email"], input[type="text"][name*="email"], input[type="text"][name*="username"], input[type="text"][name*="user"]'
      ) as HTMLInputElement;
      
      if (usernameField) {
        usernameField.value = username;
        usernameField.dispatchEvent(new Event('input', { bubbles: true }));
      }
    }

    // Find and fill password field
    const passwordField = document.querySelector('input[type="password"]') as HTMLInputElement;
    if (passwordField && password) {
      passwordField.value = password;
      passwordField.dispatchEvent(new Event('input', { bubbles: true }));
    }

    this.showToast('Credentials filled successfully', 'success');
  }

  highlightPasswordFields() {
    const passwordFields = document.querySelectorAll('input[type="password"]');
    
    passwordFields.forEach(field => {
      const originalBorder = field.style.border;
      field.style.border = '2px solid #2563eb';
      field.style.boxShadow = '0 0 0 3px rgba(37, 99, 235, 0.2)';
      
      setTimeout(() => {
        field.style.border = originalBorder;
        field.style.boxShadow = '';
      }, 2000);
    });
  }

  // Cleanup on page unload
  cleanup() {
    this.fillIndicators.forEach(indicator => indicator.remove());
    this.fillIndicators.clear();
  }
}

// Initialize content script
const coldforgeVault = new ColdforgeVaultContent();

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
  coldforgeVault.cleanup();
});