import { app, BrowserWindow, Menu, ipcMain, dialog, shell, protocol } from 'electron';
import { autoUpdater } from 'electron-updater';
import * as path from 'path';
import * as isDev from 'electron-is-dev';

// Security: Disable node integration in renderer
process.env.ELECTRON_DISABLE_SECURITY_WARNINGS = 'true';

class CloistrVaultApp {
  private mainWindow: BrowserWindow | null = null;
  private isQuitting = false;

  constructor() {
    this.initializeApp();
  }

  private async initializeApp() {
    // Handle app events
    app.whenReady().then(() => this.createMainWindow());
    app.on('window-all-closed', () => this.onWindowAllClosed());
    app.on('activate', () => this.onActivate());
    app.on('before-quit', () => this.onBeforeQuit());

    // Security protocols
    app.on('web-contents-created', (event, contents) => {
      this.setupWebContentsSecurityHandlers(contents);
    });

    // IPC handlers
    this.setupIpcHandlers();

    // Auto updater (for production)
    if (!isDev) {
      this.setupAutoUpdater();
    }
  }

  private createMainWindow(): void {
    // Create the browser window
    this.mainWindow = new BrowserWindow({
      width: 1200,
      height: 800,
      minWidth: 800,
      minHeight: 600,
      show: false,
      webPreferences: {
        nodeIntegration: false,
        contextIsolation: true,
        enableRemoteModule: false,
        allowRunningInsecureContent: false,
        experimentalFeatures: false,
        preload: path.join(__dirname, 'preload.js'),
      },
      titleBarStyle: process.platform === 'darwin' ? 'hiddenInset' : 'default',
      icon: path.join(__dirname, '../../assets/icon.png'),
    });

    // Load the app
    const startUrl = isDev 
      ? 'http://localhost:3000' 
      : `file://${path.join(__dirname, '../renderer/index.html')}`;

    this.mainWindow.loadURL(startUrl);

    // Show when ready to prevent visual flash
    this.mainWindow.once('ready-to-show', () => {
      this.mainWindow?.show();
      
      if (isDev) {
        this.mainWindow?.webContents.openDevTools();
      }
    });

    // Handle window events
    this.mainWindow.on('closed', () => {
      this.mainWindow = null;
    });

    // Handle external links
    this.mainWindow.webContents.setWindowOpenHandler(({ url }) => {
      shell.openExternal(url);
      return { action: 'deny' };
    });

    // Create menu
    this.createMenu();
  }

  private createMenu(): void {
    const template: Electron.MenuItemConstructorOptions[] = [
      {
        label: 'File',
        submenu: [
          {
            label: 'New Item',
            accelerator: 'CmdOrCtrl+N',
            click: () => this.sendToRenderer('menu-new-item'),
          },
          { type: 'separator' },
          {
            label: 'Import',
            click: () => this.handleImport(),
          },
          {
            label: 'Export',
            click: () => this.handleExport(),
          },
          { type: 'separator' },
          {
            label: 'Lock Vault',
            accelerator: 'CmdOrCtrl+L',
            click: () => this.sendToRenderer('menu-lock-vault'),
          },
          { type: 'separator' },
          {
            role: 'quit',
            accelerator: process.platform === 'darwin' ? 'Cmd+Q' : 'Ctrl+Q',
          },
        ],
      },
      {
        label: 'Edit',
        submenu: [
          { role: 'undo' },
          { role: 'redo' },
          { type: 'separator' },
          { role: 'cut' },
          { role: 'copy' },
          { role: 'paste' },
          { role: 'selectall' },
          { type: 'separator' },
          {
            label: 'Search',
            accelerator: 'CmdOrCtrl+F',
            click: () => this.sendToRenderer('menu-search'),
          },
        ],
      },
      {
        label: 'View',
        submenu: [
          { role: 'reload' },
          { role: 'forceReload' },
          { role: 'toggleDevTools' },
          { type: 'separator' },
          { role: 'resetZoom' },
          { role: 'zoomIn' },
          { role: 'zoomOut' },
          { type: 'separator' },
          { role: 'togglefullscreen' },
        ],
      },
      {
        label: 'Security',
        submenu: [
          {
            label: 'Generate Password',
            accelerator: 'CmdOrCtrl+G',
            click: () => this.sendToRenderer('menu-generate-password'),
          },
          {
            label: 'Check Password Strength',
            click: () => this.sendToRenderer('menu-check-password'),
          },
          { type: 'separator' },
          {
            label: 'Security Settings',
            click: () => this.sendToRenderer('menu-security-settings'),
          },
        ],
      },
      {
        role: 'help',
        submenu: [
          {
            label: 'About Cloistr Vault',
            click: () => this.showAbout(),
          },
          {
            label: 'Documentation',
            click: () => shell.openExternal('https://github.com/cloistr/vault/docs'),
          },
          {
            label: 'Report Issue',
            click: () => shell.openExternal('https://github.com/cloistr/vault/issues'),
          },
        ],
      },
    ];

    // macOS specific menu adjustments
    if (process.platform === 'darwin') {
      template.unshift({
        label: 'Cloistr Vault',
        submenu: [
          { role: 'about' },
          { type: 'separator' },
          { role: 'services' },
          { type: 'separator' },
          { role: 'hide' },
          { role: 'hideOthers' },
          { role: 'unhide' },
          { type: 'separator' },
          { role: 'quit' },
        ],
      });
    }

    const menu = Menu.buildFromTemplate(template);
    Menu.setApplicationMenu(menu);
  }

  private setupWebContentsSecurityHandlers(contents: Electron.WebContents): void {
    // Prevent navigation to external protocols
    contents.on('will-navigate', (event, navigationUrl) => {
      const parsedUrl = new URL(navigationUrl);
      
      if (parsedUrl.origin !== 'http://localhost:3000' && !navigationUrl.startsWith('file://')) {
        event.preventDefault();
      }
    });

    // Prevent new window creation
    contents.setWindowOpenHandler(() => {
      return { action: 'deny' };
    });
  }

  private setupIpcHandlers(): void {
    // Secure storage operations
    ipcMain.handle('secure-storage-get', async (event, key: string) => {
      // In production, use keytar or similar for secure storage
      return null;
    });

    ipcMain.handle('secure-storage-set', async (event, key: string, value: string) => {
      // In production, use keytar or similar for secure storage
      return true;
    });

    ipcMain.handle('secure-storage-delete', async (event, key: string) => {
      // In production, use keytar or similar for secure storage
      return true;
    });

    // File operations
    ipcMain.handle('show-save-dialog', async (event, options) => {
      if (!this.mainWindow) return null;
      return await dialog.showSaveDialog(this.mainWindow, options);
    });

    ipcMain.handle('show-open-dialog', async (event, options) => {
      if (!this.mainWindow) return null;
      return await dialog.showOpenDialog(this.mainWindow, options);
    });

    // App info
    ipcMain.handle('get-app-version', () => {
      return app.getVersion();
    });

    ipcMain.handle('get-platform', () => {
      return process.platform;
    });
  }

  private setupAutoUpdater(): void {
    autoUpdater.checkForUpdatesAndNotify();

    autoUpdater.on('update-available', () => {
      dialog.showMessageBox(this.mainWindow!, {
        type: 'info',
        title: 'Update available',
        message: 'A new version is available. It will be downloaded in the background.',
        buttons: ['OK'],
      });
    });

    autoUpdater.on('update-downloaded', () => {
      dialog.showMessageBox(this.mainWindow!, {
        type: 'info',
        title: 'Update ready',
        message: 'Update downloaded. The application will restart to apply the update.',
        buttons: ['Restart Now', 'Later'],
      }).then((result) => {
        if (result.response === 0) {
          autoUpdater.quitAndInstall();
        }
      });
    });
  }

  private sendToRenderer(channel: string, ...args: any[]): void {
    if (this.mainWindow && !this.mainWindow.isDestroyed()) {
      this.mainWindow.webContents.send(channel, ...args);
    }
  }

  private async handleImport(): Promise<void> {
    if (!this.mainWindow) return;

    const result = await dialog.showOpenDialog(this.mainWindow, {
      title: 'Import Vault Data',
      filters: [
        { name: 'JSON Files', extensions: ['json'] },
        { name: 'CSV Files', extensions: ['csv'] },
        { name: 'All Files', extensions: ['*'] },
      ],
      properties: ['openFile'],
    });

    if (!result.canceled && result.filePaths.length > 0) {
      this.sendToRenderer('file-import', result.filePaths[0]);
    }
  }

  private async handleExport(): Promise<void> {
    if (!this.mainWindow) return;

    const result = await dialog.showSaveDialog(this.mainWindow, {
      title: 'Export Vault Data',
      defaultPath: 'vault-export.json',
      filters: [
        { name: 'JSON Files', extensions: ['json'] },
        { name: 'CSV Files', extensions: ['csv'] },
      ],
    });

    if (!result.canceled && result.filePath) {
      this.sendToRenderer('file-export', result.filePath);
    }
  }

  private showAbout(): void {
    dialog.showMessageBox(this.mainWindow!, {
      type: 'info',
      title: 'About Cloistr Vault',
      message: 'Cloistr Vault',
      detail: `Version: ${app.getVersion()}\nZero-knowledge password manager\n\n© 2024 Cloistr Vault Team`,
      buttons: ['OK'],
    });
  }

  private onWindowAllClosed(): void {
    if (process.platform !== 'darwin' || this.isQuitting) {
      app.quit();
    }
  }

  private onActivate(): void {
    if (this.mainWindow === null && !this.isQuitting) {
      this.createMainWindow();
    }
  }

  private onBeforeQuit(): void {
    this.isQuitting = true;
  }
}

// Initialize the application
new CloistrVaultApp();