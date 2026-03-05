# Cloistr Vault Browser Extension

Browser extensions for Cloistr Vault - a zero-knowledge password manager.

## Features

- Autofill credentials on login forms
- Generate secure passwords
- Save new credentials
- Quick access popup with password search
- Context menu integration
- Auto-lock after inactivity

## Structure

```
browser-extension/
├── chrome/           # Chrome/Chromium extension (Manifest V3)
│   ├── manifest.json
│   ├── icons/
│   └── src/
│       ├── background/
│       ├── content/
│       └── popup/
└── firefox/          # Firefox extension (Manifest V2)
    ├── manifest.json
    ├── icons/
    └── src/
        ├── background/
        ├── content/
        └── popup/
```

## Development

### Chrome

1. Open `chrome://extensions/` in Chrome
2. Enable "Developer mode"
3. Click "Load unpacked"
4. Select the `chrome/` directory

### Firefox

1. Open `about:debugging` in Firefox
2. Click "This Firefox"
3. Click "Load Temporary Add-on"
4. Select `firefox/manifest.json`

## Building for Production

### Chrome Web Store

```bash
cd chrome
zip -r ../cloistr-vault-chrome.zip . -x "*.git*"
```

Upload the zip file to the Chrome Web Store Developer Dashboard.

### Firefox Add-ons (AMO)

```bash
cd firefox
zip -r ../cloistr-vault-firefox.zip . -x "*.git*"
```

Upload the zip file to Firefox Add-ons Developer Hub.

## API Integration

The extension communicates with the Cloistr Vault backend at `vault.cloistr.xyz`:

- `/api/v1/auth/login` - Authenticate
- `/api/v1/vault` - Get/update vault data
- `/api/v1/user/webauthn/*` - Passkey management

## Security Notes

- All encryption happens client-side
- Master password never leaves the device
- Vault data is encrypted with AES-256-GCM
- Keys derived using Scrypt (N=32768, r=8, p=1)

## Permissions

| Permission | Purpose |
|------------|---------|
| `activeTab` | Access current tab for autofill |
| `storage` | Store encrypted credentials locally |
| `tabs` | Query tab URL for domain matching |
| `contextMenus` | Right-click menu integration |
| `<all_urls>` | Content script for form detection |

## Testing

### Manual Testing

1. Load the extension in developer mode
2. Navigate to a login page (e.g., github.com)
3. Verify the key icon appears in password fields
4. Test unlock, fill, generate, and save flows

### Demo Mode

For testing, use the demo master password: `demo123`

This is hardcoded in the background script for development purposes and should be replaced with proper authentication in production.

## Future Improvements

- [ ] Sync with vault.cloistr.xyz backend
- [ ] Passkey authentication in extension
- [ ] Biometric unlock (where supported)
- [ ] Import/export functionality
- [ ] Safari extension (WebExtension)
- [ ] Edge extension (same as Chrome)
