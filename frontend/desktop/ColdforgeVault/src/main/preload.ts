import { contextBridge, ipcRenderer } from 'electron';

// Expose protected methods that allow the renderer process to use
// the ipcRenderer without exposing the entire object
contextBridge.exposeInMainWorld('electronAPI', {
  // Secure storage
  secureStorage: {
    get: (key: string) => ipcRenderer.invoke('secure-storage-get', key),
    set: (key: string, value: string) => ipcRenderer.invoke('secure-storage-set', key, value),
    delete: (key: string) => ipcRenderer.invoke('secure-storage-delete', key),
  },

  // File operations
  showSaveDialog: (options: any) => ipcRenderer.invoke('show-save-dialog', options),
  showOpenDialog: (options: any) => ipcRenderer.invoke('show-open-dialog', options),

  // App info
  getAppVersion: () => ipcRenderer.invoke('get-app-version'),
  getPlatform: () => ipcRenderer.invoke('get-platform'),

  // Menu events
  onMenuAction: (callback: (action: string, ...args: any[]) => void) => {
    ipcRenderer.on('menu-new-item', () => callback('new-item'));
    ipcRenderer.on('menu-lock-vault', () => callback('lock-vault'));
    ipcRenderer.on('menu-search', () => callback('search'));
    ipcRenderer.on('menu-generate-password', () => callback('generate-password'));
    ipcRenderer.on('menu-check-password', () => callback('check-password'));
    ipcRenderer.on('menu-security-settings', () => callback('security-settings'));
    ipcRenderer.on('file-import', (event, filePath) => callback('file-import', filePath));
    ipcRenderer.on('file-export', (event, filePath) => callback('file-export', filePath));
  },

  // Remove listeners
  removeAllListeners: (channel: string) => {
    ipcRenderer.removeAllListeners(channel);
  },
});

// Types for TypeScript
declare global {
  interface Window {
    electronAPI: {
      secureStorage: {
        get: (key: string) => Promise<string | null>;
        set: (key: string, value: string) => Promise<boolean>;
        delete: (key: string) => Promise<boolean>;
      };
      showSaveDialog: (options: any) => Promise<any>;
      showOpenDialog: (options: any) => Promise<any>;
      getAppVersion: () => Promise<string>;
      getPlatform: () => Promise<string>;
      onMenuAction: (callback: (action: string, ...args: any[]) => void) => void;
      removeAllListeners: (channel: string) => void;
    };
  }
}