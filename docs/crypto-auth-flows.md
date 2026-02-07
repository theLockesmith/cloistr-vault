# Crypto Authentication Flows - Design Document

## 🎯 **Philosophy: Signature-Based Identity**

For cryptographic authentication methods (Nostr, Lightning), we use **signature-based identity creation** rather than traditional registration flows. The signature itself proves identity and creates the user account.

## 🔐 **Authentication Flow Design**

### **📧 Email/Password (Traditional)**
```
Registration Required: YES
Flow: Register → Email verification → Login
Identity: Email address
```

### **⚡ Lightning Address (Signature-Based)**
```
Registration Required: NO
Flow: Sign challenge → Auto-create user → Login
Identity: Lightning Address (e.g., alice@coldforge.xyz)
User ID: Derived from Lightning Address + domain verification
```

### **🔑 Nostr (Signature-Based)**
```
Registration Required: NO
Flow: Sign challenge → Auto-create user → Login
Identity: Nostr public key (npub...)
User ID: Derived from Nostr public key
```

### **🆔 NIP-05 (Signature-Based)**
```
Registration Required: NO
Flow: NIP-05 verification + signature → Auto-create user → Login
Identity: NIP-05 address (e.g., alice@coldforge.xyz)
User ID: Derived from verified Nostr key + NIP-05
```

## 🏗️ **Implementation Architecture**

### **Unified Authentication Interface:**
```typescript
interface AuthMethod {
  type: 'email' | 'nostr' | 'lightning' | 'nip05';
  requiresRegistration: boolean;
  identityDerivation: 'user_provided' | 'signature_based' | 'domain_verified';
}
```

### **Auth Flow Examples:**

**🔑 Nostr Authentication:**
```typescript
// 1. User clicks "Sign in with Nostr"
// 2. Browser extension/app generates signature for challenge
// 3. Backend verifies signature and derives user ID
// 4. Auto-create user if first time, login if existing

const nostrFlow = {
  challenge: "Sign this message to authenticate: coldforge-auth-12345",
  signature: "signed_by_nostr_private_key",
  publicKey: "npub1...",
  // No email/password needed - signature proves identity
}
```

**⚡ Lightning Address Authentication:**
```typescript
// 1. User enters alice@coldforge.xyz (or external domain)
// 2. System verifies Lightning Address exists via LNURL
// 3. Generate LNURL-auth challenge
// 4. User signs with Lightning node
// 5. Auto-create user based on Lightning Address

const lightningFlow = {
  lightningAddress: "alice@coldforge.xyz",
  lnurlAuth: "lnurl1...",
  signature: "signed_by_lightning_node",
  // Auto-derive user ID from Lightning Address
}
```

## 🎨 **UI/UX Design**

### **Authentication Selection Screen:**
```
┌─────────────────────────────────────┐
│  🛡️ Coldforge Vault                │
│                                     │
│  Choose your authentication:        │
│                                     │
│  📧 Email & Password                │
│  [Sign In] [Register]               │
│                                     │
│  🔑 Sign in with Nostr              │
│  [Connect Wallet/Extension]         │
│                                     │
│  ⚡ Sign in with Lightning          │
│  [Lightning Address: @coldforge.xyz]│
│                                     │
│  🆔 Sign in with NIP-05             │
│  [Verify Identity: @coldforge.xyz] │
└─────────────────────────────────────┘
```

### **Crypto Auth Benefits Messaging:**
```
🔑 Nostr: "No passwords needed - your cryptographic identity is your login"
⚡ Lightning: "One address for payments and authentication"
🆔 NIP-05: "Verified identity across the Nostr network"
```

## 🔧 **Backend Implementation**

### **Auto-User Creation Logic:**
```go
func (p *NostrProvider) AuthenticateOrCreate(signature, challenge, pubkey string) (*User, error) {
    // 1. Verify signature
    if !crypto.VerifyNostrSignature(signature, challenge, pubkey) {
        return nil, ErrInvalidSignature
    }

    // 2. Derive user ID from public key
    userID := crypto.DeriveUserIDFromNostrKey(pubkey)

    // 3. Check if user exists
    user, err := p.getUserByNostrKey(pubkey)
    if err == ErrUserNotFound {
        // 4. Auto-create user
        user = &User{
            ID:        userID,
            Email:     fmt.Sprintf("%s@nostr.local", pubkey[:16]),
            CreatedAt: time.Now(),
        }
        if err := p.createUser(user, pubkey); err != nil {
            return nil, err
        }
    }

    return user, nil
}
```

### **Lightning Address Auto-Creation:**
```go
func (p *LightningProvider) AuthenticateOrCreate(address, signature string) (*User, error) {
    // 1. Parse and verify Lightning Address
    lnAddr, err := ParseLightningAddress(address)
    if err != nil {
        return nil, err
    }

    // 2. For our domain (@coldforge.xyz)
    if lnAddr.Domain == "coldforge.xyz" {
        // Auto-create with username reservation
        userID := uuid.New()
        user := &User{
            ID:    userID,
            Email: address, // Lightning Address as email
        }

        // Reserve the Lightning Address
        if err := p.reserveLightningAddress(address, userID); err != nil {
            return nil, err
        }

        return user, p.createUser(user, address)
    }

    // 3. For external domains, verify via LNURL-auth
    return p.verifyExternalLightningAuth(address, signature)
}
```

## 🎯 **Recommended Implementation Priority**

### **Phase 1: Enhanced Email Auth (Current)**
- ✅ Working email/password with registration
- ✅ Dark/light mode implemented
- ✅ Improved text contrast

### **Phase 2: Nostr Signature Auth**
```typescript
// Add to frontend:
const connectNostr = async () => {
  if (window.nostr) {
    const pubkey = await window.nostr.getPublicKey();
    const challenge = await generateChallenge();
    const signature = await window.nostr.signEvent(challenge);

    // No registration needed - signature proves identity
    await authenticateWithSignature(pubkey, signature, challenge);
  }
};
```

### **Phase 3: Lightning Address Integration**
```typescript
// Add to frontend:
const connectLightning = async (lightningAddress: string) => {
  // 1. Verify Lightning Address exists
  // 2. Generate LNURL-auth challenge
  // 3. User signs with Lightning wallet
  // 4. Auto-create account based on Lightning Address
  // 5. For @coldforge.xyz addresses, become their identity provider
};
```

## 🔄 **User Experience Flow**

### **First-Time Crypto User:**
1. **Click "Sign in with Nostr"**
2. **Browser extension opens** (or shows QR code for mobile)
3. **Sign challenge** → **Account automatically created**
4. **Immediately logged in** → No separate registration step
5. **Lightning Address assigned**: `npub123...@coldforge.xyz` (optional)

### **Returning User:**
1. **Click "Sign in with Nostr"**
2. **Same signature process**
3. **Recognized by public key** → Instant login
4. **Vault restored** with all saved passwords

### **Lightning Integration:**
1. **User requests**: `alice@coldforge.xyz`
2. **If available**: Auto-create account + Lightning Address
3. **Future logins**: Use Lightning Address OR Nostr signature
4. **Payment integration**: Same address receives payments

## 🎁 **Benefits of This Approach:**

### **For Users:**
- **No passwords to remember** for crypto auth
- **Instant onboarding** - no registration forms
- **True ownership** - control your own keys
- **Unified identity** - same address for auth + payments

### **For Coldforge:**
- **Better UX** than traditional crypto apps
- **Identity provider** for @coldforge.xyz addresses
- **Lightning integration** comes naturally
- **Future-proof** for Web3 trends

This approach makes Coldforge Vault feel native to crypto users while maintaining excellent traditional auth for mainstream users!

**Want me to implement the Nostr signature flow next?**