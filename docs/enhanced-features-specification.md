# Enhanced Features Specification

## 🗂️ **Professional Password Manager Features**

### **📁 Directory/Folder Organization**

**KeePass-style left panel with hierarchical folders:**

```typescript
interface VaultFolder {
  id: string;
  name: string;
  parentId?: string;
  icon: string;          // 📁, 🏢, 🌐, 🔧, etc.
  color: string;         // hex color for visual organization
  position: number;      // for custom ordering
  isShared: boolean;     // team sharing feature
  entryCount: number;    // cached count for performance
  children?: VaultFolder[];
}

// Example folder structure:
const exampleFolders = [
  { name: "🏢 Work", children: [
    { name: "🌐 AWS", icon: "☁️" },
    { name: "🐙 GitHub", icon: "🐙" },
    { name: "💼 Corporate", icon: "🏢" }
  ]},
  { name: "💳 Personal", children: [
    { name: "🏦 Banking", icon: "🏦" },
    { name: "🛒 Shopping", icon: "🛒" },
    { name: "🎮 Gaming", icon: "🎮" }
  ]},
  { name: "🔧 Development", children: [
    { name: "🔑 API Keys", icon: "🔑" },
    { name: "🖥️ Servers", icon: "🖥️" },
    { name: "📜 Certificates", icon: "📜" }
  ]}
];
```

### **🏷️ Advanced Tagging System**

```typescript
interface VaultTag {
  id: string;
  name: string;
  color: string;
  category: 'security' | 'type' | 'custom';
  isSystem: boolean;     // auto-generated vs user-created
  usageCount: number;
}

// System-generated tags:
const systemTags = [
  { name: "weak-password", color: "#ef4444", category: "security" },
  { name: "reused-password", color: "#f59e0b", category: "security" },
  { name: "2fa-enabled", color: "#10b981", category: "security" },
  { name: "breach-detected", color: "#dc2626", category: "security" },
  { name: "password-expired", color: "#f59e0b", category: "security" },
  { name: "api-key", color: "#6366f1", category: "type" },
  { name: "shared-account", color: "#8b5cf6", category: "type" }
];
```

### **🔐 Enhanced Entry Types & Multiple Secrets**

```typescript
interface VaultEntry {
  id: string;
  name: string;
  entryType: EntryType;
  folderId?: string;
  url?: string;
  notes: string;         // markdown notes
  isFavorite: boolean;
  tags: VaultTag[];
  secrets: VaultSecret[];
  attachments: VaultAttachment[];

  // Security analysis
  strengthScore: number;  // 0-100
  hasBreach: boolean;
  lastBreachCheck: Date;

  // Usage tracking
  lastUsed: Date;
  usageCount: number;

  // Organization
  position: number;
}

type EntryType =
  | 'login'           // Standard username/password
  | 'api_key'         // API keys with multiple endpoints
  | 'server'          // Server credentials with SSH keys
  | 'crypto_wallet'   // Crypto wallet seeds/keys
  | 'secure_note'     // Encrypted notes
  | 'credit_card'     // Payment information
  | 'identity'        // Personal identity info
  | 'license'         // Software licenses
  | 'wifi'           // WiFi credentials
  | 'bank_account'   // Banking information
  | 'custom';

interface VaultSecret {
  id: string;
  secretType: SecretType;
  name: string;          // "Login Password", "API Key", "SSH Private Key"
  encryptedValue: string; // client-encrypted

  // Metadata
  expiresAt?: Date;
  lastRotated?: Date;
  strengthScore: number;
  breachStatus: 'safe' | 'warning' | 'compromised';

  // Usage
  lastAccessed: Date;
  accessCount: number;

  // Validation
  isValid: boolean;
  validatedAt?: Date;
  validationError?: string;
}

type SecretType =
  | 'password'
  | 'username'
  | 'api_key'
  | 'app_password'    // App-specific passwords
  | 'recovery_code'
  | 'totp_secret'
  | 'private_key'
  | 'certificate'
  | 'token'
  | 'pin'
  | 'security_question'
  | 'custom';
```

### **🎲 Password Generation System**

```typescript
interface PasswordGenerationSettings {
  length: number;         // 8-128 characters
  includeUppercase: boolean;
  includeLowercase: boolean;
  includeNumbers: boolean;
  includeSymbols: boolean;
  excludeSimilar: boolean;     // avoid 0, O, l, 1, etc.
  excludeAmbiguous: boolean;   // avoid {}[]()\/~,;.<>
  customSymbols?: string;      // user-defined symbol set

  // Advanced options
  pronounceable: boolean;      // generate pronounceable passwords
  pattern?: string;           // custom pattern like "Cvcc-cvcc-99"
  minimumEntropy: number;     // bits of entropy requirement

  // Exclusions
  excludeWords: string[];     // dictionary words to avoid
  excludePatterns: string[];  // regex patterns to avoid
}

interface GeneratedPassword {
  password: string;
  strengthScore: number;      // 0-100
  entropy: number;           // bits of entropy
  timeTooCrack: string;      // human-readable time estimate
  settings: PasswordGenerationSettings;
  createdAt: Date;
  usedForEntry?: string;     // entry ID if used
}

// Pre-defined password types
const passwordPresets = {
  "maximum": { length: 64, all: true, symbols: true },
  "strong": { length: 20, all: true, symbols: true },
  "readable": { length: 16, pronounceable: true },
  "pin": { length: 6, numbersOnly: true },
  "api-key": { length: 32, alphanumeric: true },
  "passphrase": { wordCount: 6, separator: "-" }
};
```

### **📎 File Attachments**

```typescript
interface VaultAttachment {
  id: string;
  name: string;
  fileType: 'image' | 'document' | 'key_file' | 'certificate' | 'backup';
  mimeType: string;
  fileSize: number;
  encryptedData: string;  // base64 encrypted file
  createdAt: Date;
}

// Supported attachment types:
const attachmentTypes = [
  { type: 'image', icon: '🖼️', maxSize: '10MB' },
  { type: 'document', icon: '📄', maxSize: '50MB' },
  { type: 'key_file', icon: '🔑', maxSize: '1MB' },
  { type: 'certificate', icon: '📜', maxSize: '1MB' },
  { type: 'backup', icon: '💾', maxSize: '100MB' }
];
```

## 🎯 **Use Case Examples**

### **🌐 Complex Web Service Entry:**
```typescript
const githubEntry: VaultEntry = {
  name: "GitHub Enterprise",
  entryType: "api_key",
  url: "https://github.com/company",
  notes: "# GitHub Enterprise Setup\n\n- Admin access\n- 2FA enabled\n- Expires quarterly",
  tags: ["work", "development", "critical", "2fa-enabled"],
  secrets: [
    { name: "Login Password", secretType: "password", encryptedValue: "..." },
    { name: "Personal Access Token", secretType: "api_key", expiresAt: "2024-12-31" },
    { name: "SSH Private Key", secretType: "private_key", encryptedValue: "..." },
    { name: "App Password (CI/CD)", secretType: "app_password", encryptedValue: "..." },
    { name: "Recovery Codes", secretType: "recovery_code", encryptedValue: "..." }
  ],
  attachments: [
    { name: "SSH Public Key", fileType: "key_file" },
    { name: "Setup Instructions", fileType: "document" }
  ]
};
```

### **🏦 Banking Entry:**
```typescript
const bankEntry: VaultEntry = {
  name: "Chase Business Account",
  entryType: "bank_account",
  url: "https://secure.chase.com",
  notes: "Business checking account\nMonthly fee: $15\nContact: John Smith",
  tags: ["banking", "business", "high-priority"],
  secrets: [
    { name: "Login Password", secretType: "password" },
    { name: "Phone Banking PIN", secretType: "pin" },
    { name: "Security Question 1", secretType: "security_question" },
    { name: "Security Question 2", secretType: "security_question" }
  ]
};
```

### **☁️ AWS Account Entry:**
```typescript
const awsEntry: VaultEntry = {
  name: "AWS Production Environment",
  entryType: "server",
  url: "https://console.aws.amazon.com",
  notes: "Production AWS account\n\n**Important**: MFA required\n**Budget**: $5000/month",
  tags: ["aws", "production", "critical", "api-key", "2fa-enabled"],
  secrets: [
    { name: "Root Password", secretType: "password" },
    { name: "Access Key ID", secretType: "api_key" },
    { name: "Secret Access Key", secretType: "api_key" },
    { name: "MFA Recovery Codes", secretType: "recovery_code" },
    { name: "IAM Role Token", secretType: "token", expiresAt: "2024-12-31" }
  ],
  attachments: [
    { name: "AWS Architecture Diagram", fileType: "image" },
    { name: "Access Policies", fileType: "document" }
  ]
};
```

## 🎨 **UI/UX Design**

### **Left Panel (Directory Tree):**
```
┌─ Vault Directory ──────────────┐
│ 🔍 Search entries...           │
│ ───────────────────────────────│
│ 📁 All Items (247)            │
│ ⭐ Favorites (12)              │
│ 🗑️ Trash (3)                 │
│ ───────────────────────────────│
│ 📁 🏢 Work (89)               │
│   ├─ 🌐 AWS (23)              │
│   ├─ 🐙 GitHub (15)           │
│   └─ 💼 Corporate (51)        │
│ 📁 💳 Personal (76)           │
│   ├─ 🏦 Banking (8)           │
│   ├─ 🛒 Shopping (45)         │
│   └─ 🎮 Gaming (23)           │
│ 📁 🔧 Development (82)        │
│   ├─ 🔑 API Keys (34)         │
│   ├─ 🖥️ Servers (28)          │
│   └─ 📜 Certificates (20)     │
└────────────────────────────────┘
```

### **Main Panel (Entry Details):**
```
┌─ GitHub Enterprise ─────────────────────────────────────┐
│ 🌐 https://github.com/company                          │
│ 📁 Work > Development                                   │
│ 🏷️ work, development, critical, 2fa-enabled           │
│ ─────────────────────────────────────────────────────── │
│ 🔐 Secrets (5):                                       │
│   📝 Login Password        ••••••••••• [Copy] [Show]  │
│   🔑 Personal Access Token  ghp_xxxx   [Copy] [⚠️Exp] │
│   🔐 SSH Private Key       -----BEGIN  [Copy]         │
│   🔒 App Password (CI/CD)   xxxx-xxxx  [Copy]         │
│   📋 Recovery Codes        code1,code2 [Copy]         │
│ ─────────────────────────────────────────────────────── │
│ 📎 Attachments (2):                                   │
│   📄 SSH Public Key (2KB)              [Download]     │
│   📋 Setup Instructions (15KB)         [Download]     │
│ ─────────────────────────────────────────────────────── │
│ 📝 Notes:                                             │
│ # GitHub Enterprise Setup                             │
│ - Admin access                                        │
│ - 2FA enabled                                         │
│ - Expires quarterly                                   │
└───────────────────────────────────────────────────────┘
```

### **🎲 Password Generator:**
```
┌─ Generate Password ─────────────────────────────────────┐
│ 🎲 Generated: K9#mP$vL2nQ8wXzF                        │
│ 💪 Strength: ████████░░ 89/100 (Very Strong)         │
│ ⏱️ Time to crack: 2.1 million years                  │
│ ─────────────────────────────────────────────────────── │
│ ⚙️ Settings:                                          │
│ Length: [20] ▓▓▓▓▓▓▓▓░░░░░░░░ (8-128)               │
│ ☑️ Uppercase (A-Z)    ☑️ Lowercase (a-z)             │
│ ☑️ Numbers (0-9)      ☑️ Symbols (!@#$...)           │
│ ☐ Exclude similar (0,O,l,1)  ☐ Exclude ambiguous    │
│ ─────────────────────────────────────────────────────── │
│ 📋 Presets:                                          │
│ [Maximum] [Strong] [Readable] [PIN] [API Key]         │
│ ─────────────────────────────────────────────────────── │
│ [🎲 Regenerate] [📋 Copy] [✅ Use This Password]      │
└───────────────────────────────────────────────────────┘
```

## 📋 **Feature Implementation Roadmap**

### **🎯 Phase 2A: Organization Features (Current)**
```typescript
// Priority 1: Basic folder structure
interface BasicFolder {
  id: string;
  name: string;
  parentId?: string;
  entryCount: number;
}

// Priority 2: Entry enhancement
interface EnhancedEntry {
  name: string;
  type: 'login' | 'api_key' | 'secure_note';
  folderId?: string;
  notes: string;
  tags: string[];
  multipleSecrets: {
    password?: string;
    apiKeys?: Array<{name: string, value: string, expiresAt?: Date}>;
    recoveryCode?: string;
  };
}
```

### **🎯 Phase 2B: Advanced Features**
```typescript
// Advanced organization
interface AdvancedVault {
  folders: VaultFolder[];
  tags: VaultTag[];
  entries: VaultEntry[];

  // Smart features
  passwordGenerator: PasswordGenerator;
  securityAnalysis: SecurityAnalysis;
  searchIndex: SearchIndex;

  // Sharing (future)
  sharedFolders: SharedFolder[];
  teamMembers: TeamMember[];
}
```

### **🎯 Phase 3: Intelligence Features**
```typescript
interface SecurityAnalysis {
  weakPasswords: string[];     // entry IDs
  reusedPasswords: Array<{password: string, entries: string[]}>;
  expiredSecrets: Array<{entryId: string, secretType: string}>;
  breachAlerts: Array<{entryId: string, source: string, date: Date}>;
  overallScore: number;        // 0-100 vault security score
}

interface SmartSuggestions {
  duplicateDetection: Array<{entries: string[], confidence: number}>;
  autoTagging: Array<{entryId: string, suggestedTags: string[]}>;
  folderSuggestions: Array<{entryId: string, suggestedFolder: string}>;
  passwordUpgrade: Array<{entryId: string, reason: string}>;
}
```

## 🚀 **Implementation Priority**

### **✅ Currently Working (Phase 1):**
- Email/password authentication
- Basic vault operations
- KMS integration
- Light/dark mode

### **🎯 Next Sprint (Phase 2A):**
1. **Folder system** - Basic hierarchy with drag-drop
2. **Password generator** - Built-in with presets
3. **Multiple secrets per entry** - API keys + passwords
4. **Enhanced entry types** - API, server, note categories

### **🔮 Future Sprints (Phase 2B+):**
1. **Advanced tagging** - Auto-generated security tags
2. **File attachments** - Encrypted file storage
3. **Security analysis** - Breach monitoring, weak password detection
4. **Smart search** - Full-text with filters

## 💡 **Quick Implementation Ideas**

### **Password Generator (Easy Win):**
```typescript
// Add to current App.tsx:
const generatePassword = (length = 20) => {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*';
  return Array.from(crypto.getRandomValues(new Uint32Array(length)))
    .map(x => chars[x % chars.length])
    .join('');
};

// Add button to password fields:
<button onClick={() => setPassword(generatePassword())}>🎲 Generate</button>
```

### **Basic Folders (Quick Start):**
```typescript
// Add to current state:
const [folders, setFolders] = useState([
  { id: '1', name: '🏢 Work', entryCount: 0 },
  { id: '2', name: '💳 Personal', entryCount: 0 },
  { id: '3', name: '🔧 Development', entryCount: 0 }
]);

// Add folder selection to entry form:
<select value={newEntry.folderId} onChange={...}>
  <option value="">No folder</option>
  {folders.map(f => <option key={f.id} value={f.id}>{f.name}</option>)}
</select>
```

**This creates a roadmap for building the most comprehensive, user-friendly password manager with enterprise features while maintaining the zero-knowledge architecture!**

**Which feature should we tackle first - password generation or basic folders?**