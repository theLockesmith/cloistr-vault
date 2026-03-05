# Native Mobile Setup for Passkeys

This document describes how to configure iOS and Android for passkey (WebAuthn) support.

## Prerequisites

1. Initialize the React Native project (if not done):
   ```bash
   cd frontend/mobile/ColdforgeVault
   npx react-native init CloistrVault --template react-native-template-typescript
   ```

2. Install dependencies:
   ```bash
   npm install
   cd ios && pod install && cd ..
   ```

## iOS Setup

### 1. Associated Domains Capability

In Xcode:
1. Open `ios/CloistrVault.xcodeproj`
2. Select the project in the navigator
3. Select the main target
4. Go to "Signing & Capabilities"
5. Click "+ Capability"
6. Add "Associated Domains"
7. Add the following domain:
   ```
   webcredentials:vault.cloistr.xyz
   ```

### 2. Info.plist (optional)

No additional Info.plist changes required for basic passkey support.

### 3. Backend Configuration

Set the `IOS_APP_IDS` environment variable on the server:
```
IOS_APP_IDS=TEAMID.xyz.cloistr.vault
```

Replace `TEAMID` with your Apple Developer Team ID.

The server will serve the Apple App Site Association file at:
```
https://vault.cloistr.xyz/.well-known/apple-app-site-association
```

### 4. Verify AASA

Test that the AASA file is correctly served:
```bash
curl -H "Accept: application/json" https://vault.cloistr.xyz/.well-known/apple-app-site-association
```

Expected response:
```json
{
  "webcredentials": {
    "apps": ["TEAMID.xyz.cloistr.vault"]
  }
}
```

## Android Setup

### 1. Digital Asset Links

The backend serves the asset links file. Configure it with environment variables:

```
ANDROID_PACKAGE_NAME=xyz.cloistr.vault
ANDROID_CERT_FINGERPRINTS=XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX:XX
```

### 2. Get SHA-256 Fingerprint

For debug builds:
```bash
keytool -list -v -keystore ~/.android/debug.keystore -alias androiddebugkey -storepass android -keypass android
```

For release builds, use your release keystore:
```bash
keytool -list -v -keystore your-release-key.keystore -alias your-alias
```

Copy the SHA-256 fingerprint (format: `XX:XX:XX:...`).

### 3. Verify Asset Links

Test that the asset links file is correctly served:
```bash
curl https://vault.cloistr.xyz/.well-known/assetlinks.json
```

Expected response:
```json
[
  {
    "relation": [
      "delegate_permission/common.handle_all_urls",
      "delegate_permission/common.get_login_creds"
    ],
    "target": {
      "namespace": "android_app",
      "package_name": "xyz.cloistr.vault",
      "sha256_cert_fingerprints": ["XX:XX:XX:..."]
    }
  }
]
```

### 4. AndroidManifest.xml

Add the following to `android/app/src/main/AndroidManifest.xml` inside the `<application>` tag:

```xml
<meta-data
    android:name="asset_statements"
    android:resource="@string/asset_statements" />
```

And in `android/app/src/main/res/values/strings.xml`:

```xml
<string name="asset_statements">
    [{
        \"include\": \"https://vault.cloistr.xyz/.well-known/assetlinks.json\"
    }]
</string>
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `IOS_APP_IDS` | Comma-separated iOS app IDs | `ABCD1234.xyz.cloistr.vault` |
| `ANDROID_PACKAGE_NAME` | Android package name | `xyz.cloistr.vault` |
| `ANDROID_CERT_FINGERPRINTS` | Comma-separated SHA-256 fingerprints | `XX:XX:XX:...` |

## Testing

### iOS Simulator
Passkeys work in iOS Simulator 15.0+ but may require some additional setup.

### Android Emulator
Passkeys require Android API 28+ and may require Google Play Services.

### Physical Devices
Recommended for full testing of passkey functionality.

## Troubleshooting

### "Invalid domain association" (iOS)
- Verify the AASA file is served with `Content-Type: application/json`
- Ensure the domain matches exactly (no trailing slash)
- Clear the app and reinstall to force re-fetching of AASA

### "No matching credentials" (Android)
- Verify the SHA-256 fingerprint matches your signing certificate
- Ensure the package name matches exactly
- Check that the assetlinks.json is accessible over HTTPS

### General
- Passkeys require HTTPS in production
- Both endpoints must be accessible without authentication
- The domain association files must be at the root `.well-known` path
