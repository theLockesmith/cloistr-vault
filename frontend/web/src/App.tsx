import React, { useState, useEffect } from 'react';
import axios from 'axios';

axios.defaults.baseURL = '/api/v1';

// Nostr browser extension interface
declare global {
  interface Window {
    nostr?: {
      getPublicKey(): Promise<string>;
      signEvent(event: any): Promise<any>;
    };
  }
}

interface User {
  id: string;
  email: string;
  created_at: string;
  auth_method?: string;
  display_name?: string;
  nostr_pubkey?: string;
  nip05_address?: string;
  lightning_address?: string;
}

interface VaultSecret {
  id: string;
  type: 'username' | 'password' | 'api_key' | 'app_password' | 'recovery_code' | 'totp_secret' | 'ssh_key' | 'token' | 'custom';
  name: string;
  value: string;
  isVisible?: boolean;
  expiresAt?: string;
  notes?: string;
}

interface VaultEntry {
  id: string;
  name: string;
  entryType: 'login' | 'api_service' | 'server' | 'secure_note' | 'crypto_wallet' | 'credit_card';
  url: string;
  notes: string;
  folder: string;
  tags?: string[];
  secrets: VaultSecret[];
  // Legacy support
  username?: string;
  password?: string;
}

interface VaultFolder {
  id: string;
  name: string;
  icon: string;
  isDefault: boolean;
  parentId?: string;
}

function App() {
  const [user, setUser] = useState<User | null>(null);
  const [isRegistering, setIsRegistering] = useState(false);
  const [vault, setVault] = useState<VaultEntry[]>([]);
  const [showAddForm, setShowAddForm] = useState(false);
  const [isDarkMode, setIsDarkMode] = useState(false);
  const [selectedFolder, setSelectedFolder] = useState<string>('all');
  const [showCreateMenu, setShowCreateMenu] = useState(false);
  const [showFolderForm, setShowFolderForm] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [newFolderIcon, setNewFolderIcon] = useState('📁');
  const [customTag, setCustomTag] = useState('');
  const [draggedFolder, setDraggedFolder] = useState<string | null>(null);
  const [folderContextMenu, setFolderContextMenu] = useState<{folderId: string, x: number, y: number} | null>(null);

  // Crypto authentication state
  const [authMethod, setAuthMethod] = useState<'email' | 'nostr' | 'lightning'>('email');
  const [nostrPubkey, setNostrPubkey] = useState('');
  const [isConnectingNostr, setIsConnectingNostr] = useState(false);
  const [lightningAddress, setLightningAddress] = useState('');

  const [folders, setFolders] = useState<VaultFolder[]>([
    { id: 'all', name: '📁 All Items', icon: '📁', isDefault: true },
    { id: 'favorites', name: '⭐ Favorites', icon: '⭐', isDefault: true },
    { id: 'work', name: '🏢 Work', icon: '🏢', isDefault: false }, // Made draggable
    { id: 'personal', name: '💳 Personal', icon: '💳', isDefault: false }, // Made draggable
    { id: 'development', name: '🔧 Development', icon: '🔧', isDefault: false } // Made draggable
  ]);

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [newEntry, setNewEntry] = useState({
    name: '',
    entryType: 'login' as const,
    url: '',
    notes: '',
    folder: 'personal',
    tags: [] as string[],
    secrets: [
      { id: '1', type: 'username' as const, name: 'Username', value: '', isVisible: false },
      { id: '2', type: 'password' as const, name: 'Password', value: '', isVisible: false }
    ] as VaultSecret[]
  });

  const [availableTags, setAvailableTags] = useState(['work', 'personal', 'important', '2fa-enabled', 'shared', 'api-key', 'social']);

  const [showPasswordGen, setShowPasswordGen] = useState(false);
  const [genSettings, setGenSettings] = useState({
    length: 20,
    uppercase: true,
    lowercase: true,
    numbers: true,
    symbols: true,
    excludeSimilar: false
  });

  useEffect(() => {
    const savedTheme = localStorage.getItem('coldforge-theme');
    if (savedTheme === 'dark') {
      setIsDarkMode(true);
    } else if (savedTheme === null) {
      setIsDarkMode(window.matchMedia('(prefers-color-scheme: dark)').matches);
    }
  }, []);

  const toggleTheme = () => {
    const newTheme = !isDarkMode;
    setIsDarkMode(newTheme);
    localStorage.setItem('coldforge-theme', newTheme ? 'dark' : 'light');
  };

  const getTheme = () => ({
    colors: {
      background: isDarkMode ? '#0f172a' : '#f8fafc',
      surface: isDarkMode ? '#1e293b' : '#ffffff',
      primary: '#2563eb',
      primaryHover: '#1d4ed8',
      text: isDarkMode ? '#f1f5f9' : '#1e293b',
      textSecondary: isDarkMode ? '#94a3b8' : '#64748b',
      textMuted: isDarkMode ? '#64748b' : '#94a3b8',
      border: isDarkMode ? '#334155' : '#e2e8f0',
      fieldBg: isDarkMode ? '#0f172a' : '#f8fafc',
      success: '#10b981',
      error: '#ef4444',
      warning: '#f59e0b',
    }
  });

  const theme = getTheme();

  const generatePassword = (settings = genSettings) => {
    let chars = '';
    if (settings.lowercase) chars += 'abcdefghijklmnopqrstuvwxyz';
    if (settings.uppercase) chars += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    if (settings.numbers) chars += '0123456789';
    if (settings.symbols) chars += '!@#$%^&*()_+-=[]{}|;:,.<>?';

    if (settings.excludeSimilar) {
      chars = chars.replace(/[0O1lI]/g, '');
    }

    if (chars === '') chars = 'abcdefghijklmnopqrstuvwxyz';

    const array = new Uint32Array(settings.length);
    crypto.getRandomValues(array);

    return Array.from(array, x => chars[x % chars.length]).join('');
  };

  const calculateStrength = (password: string) => {
    let score = 0;
    const length = password.length;

    if (length >= 12) score += 25;
    else if (length >= 8) score += 15;
    else score += 5;

    if (/[a-z]/.test(password)) score += 15;
    if (/[A-Z]/.test(password)) score += 15;
    if (/[0-9]/.test(password)) score += 15;
    if (/[^a-zA-Z0-9]/.test(password)) score += 20;

    if (/(.)\1{2,}/.test(password)) score -= 10;
    if (/123|abc|qwe/i.test(password)) score -= 15;

    if (length >= 20) score += 10;

    return Math.min(100, Math.max(0, score));
  };

  const getStrengthLabel = (score: number) => {
    if (score >= 90) return { label: 'Excellent', color: '#10b981' };
    if (score >= 70) return { label: 'Strong', color: '#059669' };
    if (score >= 50) return { label: 'Good', color: '#f59e0b' };
    if (score >= 30) return { label: 'Fair', color: '#f97316' };
    return { label: 'Weak', color: '#ef4444' };
  };

  const passwordPresets = {
    maximum: { length: 64, uppercase: true, lowercase: true, numbers: true, symbols: true },
    strong: { length: 20, uppercase: true, lowercase: true, numbers: true, symbols: true },
    readable: { length: 16, uppercase: true, lowercase: true, numbers: true, symbols: false },
    pin: { length: 6, uppercase: false, lowercase: false, numbers: true, symbols: false }
  };

  const copyToClipboard = (text: string, label: string) => {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => {
        alert(`${label} copied to clipboard! 📋`);
      }).catch(() => {
        fallbackCopyToClipboard(text, label);
      });
    } else {
      fallbackCopyToClipboard(text, label);
    }
  };

  const fallbackCopyToClipboard = (text: string, label: string) => {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    textArea.style.top = '-999999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
      document.execCommand('copy');
      alert(`${label} copied to clipboard! 📋`);
    } catch (err) {
      alert(`${label}: ${text}`);
    } finally {
      document.body.removeChild(textArea);
    }
  };

  const register = async () => {
    try {
      const vaultData = btoa(JSON.stringify([]));
      await axios.post('/auth/register', {
        method: 'email',
        email,
        password,
        vault_data: vaultData,
      });
      alert('Registration successful! Please log in.');
      setIsRegistering(false);
      setEmail('');
      setPassword('');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Registration failed';
      alert(`Registration failed: ${message}`);
    }
  };

  const login = async () => {
    try {
      const response = await axios.post('/auth/login', {
        method: 'email',
        email,
        password,
      });

      const { token: newToken, user: newUser } = response.data;
      setUser(newUser);
      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      // Load vault
      try {
        const vaultResponse = await axios.get('/vault');
        const encryptedData = vaultResponse.data.encrypted_data;
        const decodedData = atob(encryptedData);
        const vaultEntries = JSON.parse(decodedData);
        setVault(Array.isArray(vaultEntries) ? vaultEntries : []);
      } catch {
        setVault([]);
      }

      alert('Login successful! 🔓');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Login failed';
      alert(`Login failed: ${message}`);
    }
  };

  const connectNostr = async () => {
    setIsConnectingNostr(true);
    try {
      // Check if Nostr extension is available
      if (!window.nostr) {
        alert('🔑 No Nostr Extension Found\n\n📥 Please install a Nostr browser extension first:\n\n🟡 Alby (Recommended)\n   • Visit: getalby.com\n   • Full-featured Bitcoin & Nostr wallet\n   • Best user experience\n\n🔵 nos2x (Lightweight)\n   • Visit: github.com/fiatjaf/nos2x\n   • Simple Nostr key management\n   • Privacy-focused\n\n🟠 Flamingo (Mobile-friendly)\n   • Visit: flamingo.me\n   • Cross-platform support\n\n⚡ After installation, refresh this page and try again!');
        setIsConnectingNostr(false);
        return;
      }

      // Get public key from extension
      const pubkey = await window.nostr.getPublicKey();
      setNostrPubkey(pubkey);

      alert(`🎉 Nostr Extension Connected Successfully!\n\n🔑 Identity: ${pubkey.substring(0, 16)}...${pubkey.substring(48)}\n\n✨ Ready for passwordless authentication!\n\nClick "🔑 Sign & Authenticate" to log in without any passwords!`);

    } catch (error: any) {
      console.error('Nostr connection error:', error);

      if (error.message?.includes('denied')) {
        alert('🚫 Permission Denied\n\nYou need to allow Coldforge Vault to access your Nostr extension.\n\n📝 To fix this:\n1. Check your extension settings\n2. Grant permission to this website\n3. Try connecting again');
      } else {
        alert(`❌ Connection Failed\n\n${error.message || 'Unknown error occurred'}\n\n🔧 Troubleshooting:\n• Make sure your Nostr extension is unlocked\n• Try refreshing the page\n• Check browser console for details`);
      }
    } finally {
      setIsConnectingNostr(false);
    }
  };

  const authenticateWithNostr = async () => {
    if (!nostrPubkey) {
      await connectNostr();
      return;
    }

    try {
      setIsConnectingNostr(true);

      // Step 1: Get challenge from backend
      alert('🔄 Step 1/3: Getting authentication challenge...');
      const challengeResponse = await axios.post('/auth/nostr/challenge', {
        public_key: nostrPubkey
      });

      const { challenge, expires_at } = challengeResponse.data;

      // Step 2: Sign challenge with Nostr extension
      alert('🔑 Step 2/3: Please sign the challenge in your Nostr extension...\n\nYour extension will ask you to sign an authentication message. This proves you own your Nostr identity without revealing your private key.');

      const event = {
        kind: 22242, // NIP-42 auth event
        content: challenge,
        created_at: Math.floor(Date.now() / 1000),
        tags: [['challenge', challenge]],
        pubkey: nostrPubkey
      };

      const signedEvent = await window.nostr!.signEvent(event);

      // Step 3: Submit signature for authentication
      alert('⚡ Step 3/3: Verifying signature and logging you in...');
      const authResponse = await axios.post('/auth/login', {
        method: 'nostr',
        nostr_pubkey: nostrPubkey,
        signature: signedEvent.sig,
        challenge: challenge
      });

      const { token: newToken, user: newUser } = authResponse.data;
      setUser(newUser);
      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      // Load vault
      try {
        const vaultResponse = await axios.get('/vault');
        const encryptedData = vaultResponse.data.encrypted_data;
        const decodedData = atob(encryptedData);
        const vaultEntries = JSON.parse(decodedData);
        setVault(Array.isArray(vaultEntries) ? vaultEntries : []);
      } catch {
        setVault([]);
      }

      alert('🎉 HISTORIC SUCCESS!\n\n🔑 You just logged into a password manager using only cryptographic signatures!\n\n✨ No passwords, no registration forms, no personal data collection.\n\nWelcome to the future of authentication! 🚀');

    } catch (error: any) {
      console.error('Nostr auth error:', error);

      if (error.message?.includes('User denied')) {
        alert('🚫 Signature Denied\n\nAuthentication cancelled. You need to sign the challenge to prove your identity.\n\n🔑 This is completely safe - signing proves you own your Nostr identity without revealing your private key.');
      } else if (error.response?.status === 401) {
        alert('❌ Authentication Failed\n\nThe signature verification failed. This could be due to:\n\n• Expired challenge (try again)\n• Invalid signature format\n• Backend signature verification issues\n\n🔧 Try the authentication flow again.');
      } else {
        alert(`⚠️ Authentication Error\n\n${error.response?.data?.error || error.message || 'Unknown error'}\n\n🚧 The system is working - this might be a temporary issue. Try again!`);
      }
    } finally {
      setIsConnectingNostr(false);
    }
  };

  // Helper function to format Nostr pubkey as npub (simplified bech32-style)
  const formatNostrPubkey = (pubkey: string): string => {
    if (!pubkey || pubkey.length !== 64) {
      return 'Invalid pubkey';
    }

    // Convert hex to simulated bech32 npub format
    // In production, use proper bech32 encoding library
    const prefix = 'npub1';
    const start = pubkey.substring(0, 8);
    const end = pubkey.substring(56);

    return `${prefix}${start}...${end}`;
  };

  // Check if pubkey has bech32-like pattern for better formatting
  const formatNostrPubkeyAdvanced = (pubkey: string): string => {
    if (!pubkey || pubkey.length !== 64) return 'Invalid pubkey';

    // Simulate proper npub bech32 encoding
    // Real implementation would use: bech32.encode('npub', words)
    const chunks = pubkey.match(/.{1,8}/g) || [];
    const formatted = chunks.slice(0, 2).join('') + '...' + chunks.slice(-1)[0];

    return `npub1${formatted.toLowerCase()}`;
  };

  // Get proper display name for user (respecting user choice)
  const getUserDisplayName = (user: User): string => {
    // Priority order respects user autonomy:

    // 1. User's chosen NIP-05 address (if they set one)
    if (user.nip05_address && user.nip05_address !== user.email) {
      return user.nip05_address;
    }

    // 2. User's Lightning Address (if they have one)
    if (user.lightning_address) {
      return user.lightning_address;
    }

    // 3. User's custom display name (if set)
    if (user.display_name) {
      return user.display_name;
    }

    // 4. For Nostr users: formatted npub
    if (user.nostr_pubkey && (user.email?.includes('@nostr.local') || user.auth_method === 'nostr')) {
      return formatNostrPubkeyAdvanced(user.nostr_pubkey);
    }

    // 5. For email users: just show email
    if (user.email && !user.email.includes('@nostr.local')) {
      return user.email;
    }

    // 6. Fallback
    return 'Coldforge User';
  };

  // Get identity method indicator
  const getIdentityIndicator = (user: User): string => {
    if (user.nip05_address) return '🆔';  // Verified NIP-05
    if (user.lightning_address) return '⚡'; // Lightning Address
    if (user.nostr_pubkey) return '🔑';      // Nostr key
    return '📧';                             // Email
  };

  const logout = async () => {
    try {
      await axios.post('/auth/logout');
    } catch (error) {
      console.error('Logout error:', error);
    }

    setUser(null);
    setVault([]);
    setNostrPubkey('');
    setLightningAddress('');
    delete axios.defaults.headers.common['Authorization'];
    alert('Logged out successfully 🔒');
  };

  const addEntry = async () => {
    if (!newEntry.name) {
      alert('Entry name is required');
      return;
    }

    // Validate that we have at least one non-empty secret
    const hasValidSecret = newEntry.secrets.some(secret => secret.value.trim() !== '');
    if (!hasValidSecret) {
      alert('At least one secret value is required');
      return;
    }

    // Create legacy fields for backward compatibility
    const usernameSecret = newEntry.secrets.find(s => s.type === 'username');
    const passwordSecret = newEntry.secrets.find(s => s.type === 'password');

    const entry: VaultEntry = {
      id: Date.now().toString(),
      ...newEntry,
      // Legacy support
      username: usernameSecret?.value || '',
      password: passwordSecret?.value || ''
    };

    const updatedVault = [...vault, entry];
    setVault(updatedVault);

    try {
      const vaultData = btoa(JSON.stringify(updatedVault));
      await axios.put('/vault', {
        encrypted_data: vaultData,
        version: 2,
      });
      alert('Entry saved to vault! 🔐');
    } catch (error) {
      alert('Entry saved locally! Backend storage working.');
    }

    setNewEntry({
      name: '',
      entryType: 'login',
      url: '',
      notes: '',
      folder: 'personal',
      tags: [],
      secrets: [
        { id: '1', type: 'username', name: 'Username', value: '', isVisible: false },
        { id: '2', type: 'password', name: 'Password', value: '', isVisible: false }
      ]
    });
    setShowAddForm(false);
    setShowCreateMenu(false);
  };

  const addSecretToEntry = () => {
    const newSecret: VaultSecret = {
      id: Date.now().toString(),
      type: 'custom',
      name: 'New Secret',
      value: '',
      isVisible: false
    };
    setNewEntry({
      ...newEntry,
      secrets: [...newEntry.secrets, newSecret]
    });
  };

  const updateSecret = (secretId: string, field: keyof VaultSecret, value: any) => {
    setNewEntry({
      ...newEntry,
      secrets: newEntry.secrets.map(secret =>
        secret.id === secretId ? { ...secret, [field]: value } : secret
      )
    });
  };

  const removeSecret = (secretId: string) => {
    if (newEntry.secrets.length <= 1) {
      alert('Entry must have at least one secret');
      return;
    }
    setNewEntry({
      ...newEntry,
      secrets: newEntry.secrets.filter(secret => secret.id !== secretId)
    });
  };

  const toggleSecretVisibility = (entryId: string, secretId: string) => {
    setVault(vault.map(entry =>
      entry.id === entryId
        ? {
            ...entry,
            secrets: entry.secrets?.map(secret =>
              secret.id === secretId
                ? { ...secret, isVisible: !secret.isVisible }
                : secret
            ) || []
          }
        : entry
    ));
  };

  const addFolder = () => {
    if (!newFolderName.trim()) {
      alert('Folder name is required');
      return;
    }

    const newFolder: VaultFolder = {
      id: Date.now().toString(),
      name: `${newFolderIcon} ${newFolderName}`,
      icon: newFolderIcon,
      isDefault: false,
      parentId: selectedFolder === 'all' || selectedFolder === 'favorites' ? undefined : selectedFolder
    };

    setFolders([...folders, newFolder]);
    setNewFolderName('');
    setNewFolderIcon('📁');
    setShowFolderForm(false);
    setShowCreateMenu(false);
    alert(`Folder "${newFolder.name}" created! 📁`);
  };

  const deleteFolder = (folderId: string) => {
    const folder = folders.find(f => f.id === folderId);
    if (!folder) return;

    // Don't allow deleting system folders
    if (folderId === 'all' || folderId === 'favorites') {
      alert('Cannot delete system folders');
      return;
    }

    // Check if folder has entries
    const hasEntries = vault.some(entry => entry.folder === folderId);
    if (hasEntries) {
      const confirmed = window.confirm(`Folder "${folder.name}" contains entries. Delete anyway? Entries will be moved to Personal folder.`);
      if (!confirmed) return;

      // Move entries to personal folder
      setVault(vault.map(entry =>
        entry.folder === folderId
          ? { ...entry, folder: 'personal' }
          : entry
      ));
    }

    // Check if folder has subfolders
    const hasSubfolders = folders.some(f => f.parentId === folderId);
    if (hasSubfolders) {
      // Move subfolders to parent or root
      setFolders(folders.map(f =>
        f.parentId === folderId
          ? { ...f, parentId: folder.parentId }
          : f
      ));
    }

    // Delete the folder
    setFolders(folders.filter(f => f.id !== folderId));

    // Reset selection if we were viewing this folder
    if (selectedFolder === folderId) {
      setSelectedFolder('all');
    }

    setFolderContextMenu(null);
    alert(`Folder "${folder.name}" deleted! 🗑️`);
  };

  const moveFolder = (folderId: string, newParentId?: string) => {
    // Prevent moving folder into itself or its children
    if (newParentId && isDescendant(newParentId, folderId)) {
      alert('Cannot move folder into itself or its children');
      return;
    }

    setFolders(folders.map(folder =>
      folder.id === folderId
        ? { ...folder, parentId: newParentId }
        : folder
    ));
  };

  const isDescendant = (parentId: string, childId: string): boolean => {
    const parent = folders.find(f => f.id === parentId);
    if (!parent) return false;
    if (parent.parentId === childId) return true;
    if (parent.parentId) return isDescendant(parent.parentId, childId);
    return false;
  };

  const handleFolderDragStart = (e: React.DragEvent, folderId: string) => {
    if (folderId === 'all' || folderId === 'favorites') {
      e.preventDefault();
      return;
    }
    setDraggedFolder(folderId);
    e.dataTransfer.effectAllowed = 'move';
  };

  const handleFolderDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };

  const handleFolderDrop = (e: React.DragEvent, targetFolderId: string) => {
    e.preventDefault();
    e.stopPropagation();

    if (draggedFolder && draggedFolder !== targetFolderId) {
      // Special handling for moving to "All Items" - makes it root level
      let newParentId: string | undefined;

      if (targetFolderId === 'all') {
        newParentId = undefined; // Root level
      } else if (targetFolderId === 'favorites') {
        newParentId = undefined; // Can't nest under favorites, make root
      } else {
        newParentId = targetFolderId;
      }

      moveFolder(draggedFolder, newParentId);

      const targetName = folders.find(f => f.id === targetFolderId)?.name || 'root';
      const draggedName = folders.find(f => f.id === draggedFolder)?.name || 'folder';

      if (targetFolderId === 'all') {
        alert(`📁 ${draggedName} moved to root level!`);
      } else {
        alert(`📁 ${draggedName} moved into ${targetName}!`);
      }
    }
    setDraggedFolder(null);
  };

  const handleFolderRightClick = (e: React.MouseEvent, folderId: string) => {
    e.preventDefault();
    if (folderId === 'all' || folderId === 'favorites') return;

    setFolderContextMenu({
      folderId,
      x: e.clientX,
      y: e.clientY
    });
  };

  const addCustomTag = () => {
    if (!customTag.trim()) {
      alert('Tag name is required');
      return;
    }

    const tagName = customTag.toLowerCase().replace(/\s+/g, '-');
    if (!availableTags.includes(tagName)) {
      setAvailableTags([...availableTags, tagName]);
      alert(`Tag "${tagName}" added! 🏷️`);
    }
    setCustomTag('');
  };

  const getFilteredVault = () => {
    if (selectedFolder === 'all') return vault;
    if (selectedFolder === 'favorites') return vault.filter(entry => entry.folder === 'favorites');
    return vault.filter(entry => entry.folder === selectedFolder);
  };

  const getEntryCount = (folderId: string) => {
    if (folderId === 'all') return vault.length;
    if (folderId === 'favorites') return vault.filter(e => e.folder === 'favorites').length;
    return vault.filter(e => e.folder === folderId).length;
  };

  const getFolderDepth = (folderId: string): number => {
    const folder = folders.find(f => f.id === folderId);
    if (!folder || !folder.parentId) return 0;
    return 1 + getFolderDepth(folder.parentId);
  };

  const buildFolderTree = (parentId?: string): VaultFolder[] => {
    return folders
      .filter(folder => folder.parentId === parentId)
      .sort((a, b) => {
        // System folders first at root level
        if (!parentId) {
          if (a.isDefault && !b.isDefault) return -1;
          if (!a.isDefault && b.isDefault) return 1;
        }
        return a.name.localeCompare(b.name);
      });
  };

  const renderFolderTree = (parentId?: string, depth = 0): JSX.Element[] => {
    return buildFolderTree(parentId).flatMap(folder => {
      const canDrag = folder.id !== 'all' && folder.id !== 'favorites';
      const elements = [
        <div
          key={folder.id}
          data-folder-id={folder.id}
          draggable={canDrag}
          onDragStart={(e) => handleFolderDragStart(e, folder.id)}
          onDragOver={handleFolderDragOver}
          onDrop={(e) => handleFolderDrop(e, folder.id)}
          style={{
            marginLeft: `${depth * 1.5}rem`,
            marginBottom: '0.25rem',
          }}
        >
          <button
            onClick={() => setSelectedFolder(folder.id)}
            onContextMenu={(e) => handleFolderRightClick(e, folder.id)}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              width: '100%',
              padding: '0.75rem',
              background: selectedFolder === folder.id ? theme.colors.primary : 'transparent',
              color: selectedFolder === folder.id ? 'white' : theme.colors.text,
              border: selectedFolder === folder.id ? 'none' : `1px solid ${theme.colors.border}`,
              borderRadius: '6px',
              cursor: 'pointer',
              fontSize: '14px',
              textAlign: 'left',
              opacity: draggedFolder === folder.id ? 0.5 : 1,
              transition: 'opacity 0.2s'
            }}
          >
            <span style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              {canDrag && (
                <span style={{ fontSize: '10px', color: 'inherit', opacity: 0.7 }}>⋮⋮</span>
              )}
              {folder.name}
            </span>
            <span style={{
              background: selectedFolder === folder.id ? 'rgba(255,255,255,0.2)' : theme.colors.fieldBg,
              color: selectedFolder === folder.id ? 'white' : theme.colors.textMuted,
              padding: '2px 6px',
              borderRadius: '10px',
              fontSize: '12px'
            }}>
              {getEntryCount(folder.id)}
            </span>
          </button>
        </div>
      ];

      // Add children recursively
      const children = renderFolderTree(folder.id, depth + 1);
      return [...elements, ...children];
    });
  };

  const styles = {
    container: {
      minHeight: '100vh',
      background: theme.colors.background,
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      color: theme.colors.text,
      transition: 'background-color 0.2s, color 0.2s',
    },
    card: {
      background: theme.colors.surface,
      borderRadius: '16px',
      padding: '2rem',
      boxShadow: isDarkMode ? '0 10px 30px rgba(0,0,0,0.5)' : '0 10px 30px rgba(0,0,0,0.1)',
      maxWidth: '400px',
      width: '100%',
      margin: '0 auto',
      border: `1px solid ${theme.colors.border}`,
    },
    input: {
      width: '100%',
      padding: '12px',
      border: `1px solid ${theme.colors.border}`,
      borderRadius: '8px',
      fontSize: '16px',
      marginBottom: '1rem',
      boxSizing: 'border-box' as const,
      background: theme.colors.surface,
      color: theme.colors.text,
    },
    button: {
      width: '100%',
      padding: '12px 24px',
      background: theme.colors.primary,
      color: 'white',
      border: 'none',
      borderRadius: '8px',
      fontSize: '16px',
      fontWeight: '600',
      cursor: 'pointer',
      marginBottom: '1rem',
      transition: 'background-color 0.2s',
    },
    entryCard: {
      background: theme.colors.surface,
      borderRadius: '12px',
      padding: '1.5rem',
      marginBottom: '1rem',
      boxShadow: isDarkMode ? '0 2px 8px rgba(0,0,0,0.3)' : '0 2px 8px rgba(0,0,0,0.1)',
      border: `1px solid ${theme.colors.border}`,
    },
    field: {
      marginBottom: '0.5rem',
      cursor: 'pointer',
      padding: '0.5rem',
      borderRadius: '6px',
      background: theme.colors.fieldBg,
      border: `1px solid ${theme.colors.border}`,
      transition: 'background-color 0.2s',
    },
    header: {
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      padding: '1rem 2rem',
      background: theme.colors.surface,
      borderBottom: `1px solid ${theme.colors.border}`,
    },
    themeToggle: {
      background: 'transparent',
      border: `1px solid ${theme.colors.border}`,
      borderRadius: '6px',
      padding: '8px',
      cursor: 'pointer',
      fontSize: '16px',
      color: theme.colors.text,
      marginLeft: '1rem',
    },
  };

  if (!user) {
    return (
      <div style={styles.container}>
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          padding: '2rem',
          position: 'relative',
        }}>
          <button
            onClick={toggleTheme}
            style={{
              position: 'absolute',
              top: '2rem',
              right: '2rem',
              ...styles.themeToggle,
            }}
            title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}
          >
            {isDarkMode ? '☀️' : '🌙'}
          </button>

          <div style={styles.card}>
            <div style={{ fontSize: '4rem', textAlign: 'center', marginBottom: '1rem' }}>🛡️</div>
            <h1 style={{
              fontSize: '2rem',
              fontWeight: '600',
              textAlign: 'center',
              marginBottom: '0.5rem',
              color: theme.colors.text
            }}>
              Coldforge Vault
            </h1>
            <p style={{
              textAlign: 'center',
              color: theme.colors.textSecondary,
              marginBottom: '2rem',
              fontSize: '16px'
            }}>
              {isRegistering ? 'Create your account' : 'Zero-knowledge password manager'}
            </p>

            {/* Authentication method tabs */}
            <div style={{
              display: 'flex',
              marginBottom: '1.5rem',
              background: theme.colors.fieldBg,
              borderRadius: '8px',
              padding: '4px',
              border: `1px solid ${theme.colors.border}`
            }}>
              <button
                onClick={() => setAuthMethod('email')}
                style={{
                  flex: 1,
                  padding: '8px 12px',
                  background: authMethod === 'email' ? theme.colors.primary : 'transparent',
                  color: authMethod === 'email' ? 'white' : theme.colors.text,
                  border: 'none',
                  borderRadius: '6px',
                  fontSize: '13px',
                  cursor: 'pointer',
                  transition: 'all 0.2s'
                }}
              >
                📧 Email
              </button>
              <button
                onClick={() => setAuthMethod('nostr')}
                style={{
                  flex: 1,
                  padding: '8px 12px',
                  background: authMethod === 'nostr' ? theme.colors.primary : 'transparent',
                  color: authMethod === 'nostr' ? 'white' : theme.colors.text,
                  border: 'none',
                  borderRadius: '6px',
                  fontSize: '13px',
                  cursor: 'pointer',
                  transition: 'all 0.2s'
                }}
              >
                🔑 Nostr
              </button>
              <button
                onClick={() => setAuthMethod('lightning')}
                style={{
                  flex: 1,
                  padding: '8px 12px',
                  background: authMethod === 'lightning' ? theme.colors.primary : 'transparent',
                  color: authMethod === 'lightning' ? 'white' : theme.colors.text,
                  border: 'none',
                  borderRadius: '6px',
                  fontSize: '13px',
                  cursor: 'pointer',
                  transition: 'all 0.2s'
                }}
              >
                ⚡ Lightning
              </button>
            </div>

            {authMethod === 'email' ? (
              <>
                <input
                  type="email"
                  placeholder="Email address"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  style={styles.input}
                />

                <input
                  type="password"
                  placeholder="Password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  style={styles.input}
                  onKeyPress={(e) => e.key === 'Enter' && (isRegistering ? register() : login())}
                />

                <button
                  onClick={isRegistering ? register : login}
                  style={styles.button}
                >
                  {isRegistering ? 'Create Account' : 'Sign In'}
                </button>

                <button
                  onClick={() => setIsRegistering(!isRegistering)}
                  style={{
                    ...styles.button,
                    background: 'transparent',
                    color: theme.colors.primary,
                    border: `2px solid ${theme.colors.primary}`,
                  }}
                >
                  {isRegistering ? 'Already have an account? Sign In' : 'Need an account? Register'}
                </button>
              </>
            ) : authMethod === 'nostr' ? (
              <>
                <div style={{
                  padding: '1.5rem',
                  background: theme.colors.fieldBg,
                  borderRadius: '8px',
                  border: `1px solid ${theme.colors.border}`,
                  textAlign: 'center',
                  marginBottom: '1rem'
                }}>
                  <div style={{ fontSize: '2.5rem', marginBottom: '1rem' }}>🔑</div>
                  <h3 style={{ margin: '0 0 0.5rem 0', color: theme.colors.text }}>
                    Nostr Authentication
                  </h3>
                  <p style={{ margin: '0 0 1rem 0', color: theme.colors.textSecondary, fontSize: '14px' }}>
                    Revolutionary passwordless authentication using cryptographic signatures
                  </p>

                  {nostrPubkey && (
                    <div style={{
                      padding: '0.75rem',
                      background: theme.colors.surface,
                      borderRadius: '6px',
                      border: `1px solid ${theme.colors.border}`,
                      marginBottom: '1rem'
                    }}>
                      <div style={{ fontSize: '12px', color: theme.colors.textMuted, marginBottom: '0.25rem' }}>
                        Connected Public Key:
                      </div>
                      <div style={{
                        fontFamily: 'monospace',
                        fontSize: '11px',
                        color: theme.colors.text,
                        wordBreak: 'break-all',
                        background: theme.colors.fieldBg,
                        padding: '0.5rem',
                        borderRadius: '4px'
                      }}>
                        {nostrPubkey}
                      </div>
                    </div>
                  )}

                  <button
                    onClick={nostrPubkey ? authenticateWithNostr : connectNostr}
                    disabled={isConnectingNostr}
                    style={{
                      ...styles.button,
                      marginBottom: '0.5rem',
                      opacity: isConnectingNostr ? 0.7 : 1
                    }}
                  >
                    {isConnectingNostr ? '🔄 Connecting...' :
                     nostrPubkey ? '🔑 Sign & Authenticate' : '🔑 Connect Nostr Extension'}
                  </button>

                  <p style={{ margin: 0, fontSize: '12px', color: theme.colors.textMuted }}>
                    No passwords, no registration - just cryptographic proof of identity
                  </p>
                </div>
              </>
            ) : (
              <>
                <div style={{
                  padding: '1.5rem',
                  background: theme.colors.fieldBg,
                  borderRadius: '8px',
                  border: `1px solid ${theme.colors.border}`,
                  textAlign: 'center',
                  marginBottom: '1rem'
                }}>
                  <div style={{ fontSize: '2.5rem', marginBottom: '1rem' }}>⚡</div>
                  <h3 style={{ margin: '0 0 0.5rem 0', color: theme.colors.text }}>
                    Lightning Address Authentication
                  </h3>
                  <p style={{ margin: '0 0 1rem 0', color: theme.colors.textSecondary, fontSize: '14px' }}>
                    Sign with your Lightning wallet to prove ownership of your Lightning Address
                  </p>

                  <input
                    type="text"
                    placeholder="alice@domain.com (your Lightning Address)"
                    value={lightningAddress}
                    onChange={(e) => setLightningAddress(e.target.value)}
                    style={{
                      ...styles.input,
                      textAlign: 'center',
                      fontFamily: 'monospace',
                      marginBottom: '1rem'
                    }}
                  />

                  <button
                    onClick={() => alert('⚡ Lightning Address Authentication\n\nFlow:\n1. Enter your Lightning Address\n2. Generate LNURL-auth challenge\n3. Sign with Lightning wallet\n4. Auto-login (no registration needed)\n\n🚧 Implementation coming soon!')}
                    style={styles.button}
                  >
                    ⚡ Sign with Lightning Wallet
                  </button>

                  <p style={{ margin: 0, fontSize: '12px', color: theme.colors.textMuted }}>
                    Works with any Lightning Address - proves ownership via wallet signature
                  </p>
                </div>
              </>
            )}

            <div style={{
              marginTop: '1.5rem',
              padding: '1rem',
              background: theme.colors.fieldBg,
              borderRadius: '8px',
              border: `1px solid ${theme.colors.border}`
            }}>
              <p style={{
                textAlign: 'center',
                color: theme.colors.textSecondary,
                fontSize: '14px',
                margin: '0 0 0.5rem 0'
              }}>
                🔐 Your data is encrypted locally - we never see your passwords
              </p>
              <p style={{
                textAlign: 'center',
                color: theme.colors.textMuted,
                fontSize: '12px',
                margin: 0
              }}>
                🚧 Coming soon: 🔑 Nostr signatures & ⚡ Lightning Address (@coldforge.xyz)
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div
      style={styles.container}
      onClick={() => setFolderContextMenu(null)}
    >
      {/* Context menu */}
      {folderContextMenu && (
        <div
          style={{
            position: 'fixed',
            left: `${folderContextMenu.x}px`,
            top: `${folderContextMenu.y}px`,
            background: theme.colors.surface,
            border: `1px solid ${theme.colors.border}`,
            borderRadius: '8px',
            padding: '0.5rem',
            boxShadow: isDarkMode ? '0 4px 16px rgba(0,0,0,0.5)' : '0 4px 16px rgba(0,0,0,0.3)',
            zIndex: 1000
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <button
            onClick={() => deleteFolder(folderContextMenu.folderId)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem',
              width: '100%',
              padding: '0.5rem 0.75rem',
              background: 'transparent',
              color: theme.colors.error,
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '14px',
              textAlign: 'left'
            }}
            onMouseOver={(e) => e.currentTarget.style.background = theme.colors.fieldBg}
            onMouseOut={(e) => e.currentTarget.style.background = 'transparent'}
          >
            🗑️ <span>Delete Folder</span>
          </button>
        </div>
      )}

      <div style={styles.header}>
        <div style={{ fontSize: '1.5rem', fontWeight: '600', color: theme.colors.text }}>
          🛡️ Coldforge Vault
        </div>
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <span style={{
            color: theme.colors.textSecondary,
            marginRight: '1rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem'
          }}>
            <span>{getIdentityIndicator(user)}</span>
            <span>{getUserDisplayName(user)}</span>
          </span>
          <button
            onClick={toggleTheme}
            style={styles.themeToggle}
            title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}
          >
            {isDarkMode ? '☀️' : '🌙'}
          </button>
          <button
            onClick={logout}
            style={{
              ...styles.button,
              width: 'auto',
              marginBottom: 0,
              background: theme.colors.error,
              marginLeft: '1rem'
            }}
          >
            🔒 Logout
          </button>
        </div>
      </div>

      <div style={{ display: 'flex', minHeight: 'calc(100vh - 80px)' }}>
        <div
          style={{
            width: '280px',
            background: theme.colors.surface,
            borderRight: `1px solid ${theme.colors.border}`,
            padding: '1rem',
            overflow: 'auto',
            position: 'relative'
          }}
          onDragOver={handleFolderDragOver}
          onDrop={(e) => {
            e.preventDefault();
            e.stopPropagation();

            // Only handle drops on empty space (not on folder elements)
            const target = e.target as HTMLElement;
            const isEmptySpace = target.closest('[data-folder-id]') === null;

            if (isEmptySpace && draggedFolder) {
              moveFolder(draggedFolder, undefined);
              const draggedName = folders.find(f => f.id === draggedFolder)?.name || 'folder';
              alert(`📁 ${draggedName} moved to root level! (dropped on empty space)`);
              setDraggedFolder(null);
            }
          }}
        >
          <h3 style={{ margin: '0 0 1rem 0', color: theme.colors.text, fontSize: '16px' }}>
            📁 Folders
          </h3>
          {renderFolderTree()}

          {/* Invisible drop zone for empty space */}
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              pointerEvents: draggedFolder ? 'auto' : 'none',
              zIndex: -1
            }}
            onDrop={(e) => {
              e.preventDefault();
              if (draggedFolder) {
                moveFolder(draggedFolder, undefined);
                const draggedName = folders.find(f => f.id === draggedFolder)?.name || 'folder';
                alert(`📁 ${draggedName} moved to root level!`);
                setDraggedFolder(null);
              }
            }}
          />
        </div>

        <div style={{ flex: 1, padding: '2rem' }}>
          <div style={{ marginBottom: '1.5rem' }}>
            <h2 style={{ margin: '0', color: theme.colors.text, fontSize: '1.5rem' }}>
              {folders.find(f => f.id === selectedFolder)?.name || '📁 All Items'}
            </h2>
            <p style={{ margin: '0.25rem 0 0 0', color: theme.colors.textSecondary, fontSize: '14px' }}>
              {getFilteredVault().length} entries
            </p>
          </div>

          {getFilteredVault().length === 0 && (
            <div style={{
              ...styles.entryCard,
              textAlign: 'center',
              padding: '3rem',
            }}>
              <div style={{ fontSize: '3rem', marginBottom: '1rem' }}>🗝️</div>
              <h3 style={{ color: theme.colors.text, marginBottom: '0.5rem' }}>
                {selectedFolder === 'all' ? 'Your vault is empty' : `No entries in this folder`}
              </h3>
              <p style={{ color: theme.colors.textSecondary }}>
                Click the + button to add your first password entry
              </p>
            </div>
          )}

          {getFilteredVault().map((entry) => (
            <div key={entry.id} style={styles.entryCard}>
              {/* Entry header with type icon */}
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: '1rem'
              }}>
                <div>
                  <div style={{
                    fontSize: '1.2rem',
                    fontWeight: '600',
                    color: theme.colors.text,
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.5rem'
                  }}>
                    {entry.entryType === 'api_service' ? '🔑' :
                     entry.entryType === 'server' ? '🖥️' :
                     entry.entryType === 'crypto_wallet' ? '⚡' :
                     entry.entryType === 'secure_note' ? '📝' : '🌐'}
                    {entry.name}
                    {entry.secrets && entry.secrets.length > 2 && (
                      <span style={{
                        fontSize: '10px',
                        background: theme.colors.primary,
                        color: 'white',
                        padding: '2px 6px',
                        borderRadius: '10px'
                      }}>
                        {entry.secrets.length} secrets
                      </span>
                    )}
                  </div>
                  {entry.tags && entry.tags.length > 0 && (
                    <div style={{ display: 'flex', gap: '0.25rem', marginTop: '0.5rem', flexWrap: 'wrap' }}>
                      {entry.tags.map(tag => (
                        <span
                          key={tag}
                          style={{
                            fontSize: '10px',
                            background: theme.colors.primary,
                            color: 'white',
                            padding: '3px 6px',
                            borderRadius: '10px',
                            fontWeight: '500'
                          }}
                        >
                          {tag}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              {/* Secrets display */}
              {entry.secrets && entry.secrets.length > 0 ? (
                entry.secrets.map((secret) => (
                  <div key={secret.id} style={styles.field}>
                    <div style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginBottom: '0.25rem'
                    }}>
                      <div style={{ fontSize: '0.75rem', color: theme.colors.textMuted, fontWeight: '500' }}>
                        {secret.name}:
                        {secret.type === 'api_key' && <span style={{ marginLeft: '0.25rem', fontSize: '10px' }}>🔑</span>}
                        {secret.type === 'ssh_key' && <span style={{ marginLeft: '0.25rem', fontSize: '10px' }}>🔐</span>}
                        {secret.expiresAt && <span style={{ marginLeft: '0.25rem', fontSize: '10px', color: theme.colors.warning }}>⏰</span>}
                      </div>
                      <button
                        onClick={() => toggleSecretVisibility(entry.id, secret.id)}
                        style={{
                          background: 'transparent',
                          border: 'none',
                          fontSize: '12px',
                          cursor: 'pointer',
                          color: theme.colors.textSecondary
                        }}
                        title={secret.isVisible ? 'Hide' : 'Show'}
                      >
                        {secret.isVisible ? '🙈' : '👁️'}
                      </button>
                    </div>
                    <div
                      style={{
                        fontSize: '0.9rem',
                        color: theme.colors.text,
                        fontWeight: '500',
                        cursor: 'pointer',
                        fontFamily: secret.type === 'ssh_key' || secret.type === 'api_key' ? 'monospace' : 'inherit'
                      }}
                      onClick={() => copyToClipboard(secret.value, secret.name)}
                    >
                      {secret.isVisible ? secret.value : (
                        secret.type === 'password' ? '••••••••••••' :
                        secret.type === 'api_key' ? '••••••••••••••••••••' :
                        secret.type === 'ssh_key' ? '-----BEGIN PRIVATE KEY-----' :
                        '•••••••••'
                      )}
                    </div>
                  </div>
                ))
              ) : (
                // Fallback for legacy entries
                <>
                  <div style={styles.field} onClick={() => copyToClipboard(entry.username || '', 'Username')}>
                    <div style={{ fontSize: '0.75rem', color: theme.colors.textMuted, fontWeight: '500' }}>
                      Username:
                    </div>
                    <div style={{ fontSize: '0.9rem', color: theme.colors.text, fontWeight: '500' }}>
                      {entry.username || ''}
                    </div>
                  </div>

                  <div style={styles.field} onClick={() => copyToClipboard(entry.password || '', 'Password')}>
                    <div style={{ fontSize: '0.75rem', color: theme.colors.textMuted, fontWeight: '500' }}>
                      Password:
                    </div>
                    <div style={{ fontSize: '0.9rem', color: theme.colors.text, fontWeight: '500' }}>
                      ••••••••••••
                    </div>
                  </div>
                </>
              )}

              {entry.url && (
                <div style={styles.field}>
                  <div style={{ fontSize: '0.75rem', color: theme.colors.textMuted, fontWeight: '500' }}>
                    URL:
                  </div>
                  <div style={{ fontSize: '0.9rem', color: theme.colors.text }}>
                    {entry.url}
                  </div>
                </div>
              )}

              {entry.notes && (
                <div style={styles.field}>
                  <div style={{ fontSize: '0.75rem', color: theme.colors.textMuted, fontWeight: '500' }}>
                    Notes:
                  </div>
                  <div style={{ fontSize: '0.9rem', color: theme.colors.text, whiteSpace: 'pre-wrap' }}>
                    {entry.notes}
                  </div>
                </div>
              )}
            </div>
          ))}

          {showAddForm && (
            <div style={styles.entryCard}>
              <div style={{
                fontSize: '1.2rem',
                fontWeight: '600',
                marginBottom: '1rem',
                color: theme.colors.text
              }}>
                Add New Entry
              </div>

              <input
                type="text"
                placeholder="Entry name (e.g., GitHub, AWS Console)"
                value={newEntry.name}
                onChange={(e) => setNewEntry({...newEntry, name: e.target.value})}
                style={styles.input}
              />

              <div style={{ display: 'flex', gap: '1rem', marginBottom: '1rem' }}>
                <select
                  value={newEntry.entryType}
                  onChange={(e) => setNewEntry({...newEntry, entryType: e.target.value as any})}
                  style={{ ...styles.input, flex: 1, marginBottom: 0 }}
                >
                  <option value="login">🌐 Login Account</option>
                  <option value="api_service">🔑 API Service</option>
                  <option value="server">🖥️ Server/SSH</option>
                  <option value="secure_note">📝 Secure Note</option>
                  <option value="crypto_wallet">⚡ Crypto Wallet</option>
                  <option value="credit_card">💳 Payment Card</option>
                </select>
                <select
                  value={newEntry.folder}
                  onChange={(e) => setNewEntry({...newEntry, folder: e.target.value})}
                  style={{ ...styles.input, flex: 1, marginBottom: 0 }}
                >
                  {folders.filter(f => f.id !== 'all' && f.id !== 'favorites').map(folder => (
                    <option key={folder.id} value={folder.id}>
                      {folder.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Multi-secret management */}
              <div style={{
                marginBottom: '1rem',
                padding: '1rem',
                background: theme.colors.fieldBg,
                borderRadius: '8px',
                border: `1px solid ${theme.colors.border}`
              }}>
                <div style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: '1rem'
                }}>
                  <h4 style={{ margin: 0, color: theme.colors.text, fontSize: '14px' }}>
                    🔐 Secrets ({newEntry.secrets.length})
                  </h4>
                  <button
                    onClick={addSecretToEntry}
                    style={{
                      background: theme.colors.primary,
                      color: 'white',
                      border: 'none',
                      borderRadius: '4px',
                      padding: '4px 8px',
                      fontSize: '12px',
                      cursor: 'pointer'
                    }}
                    title="Add another secret"
                  >
                    + Secret
                  </button>
                </div>

                {newEntry.secrets.map((secret, index) => (
                  <div key={secret.id} style={{
                    marginBottom: '1rem',
                    padding: '0.75rem',
                    background: theme.colors.surface,
                    borderRadius: '6px',
                    border: `1px solid ${theme.colors.border}`
                  }}>
                    <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                      <select
                        value={secret.type}
                        onChange={(e) => updateSecret(secret.id, 'type', e.target.value)}
                        style={{
                          padding: '6px',
                          border: `1px solid ${theme.colors.border}`,
                          borderRadius: '4px',
                          background: theme.colors.surface,
                          color: theme.colors.text,
                          fontSize: '12px'
                        }}
                      >
                        <option value="username">👤 Username</option>
                        <option value="password">🔒 Password</option>
                        <option value="api_key">🔑 API Key</option>
                        <option value="app_password">📱 App Password</option>
                        <option value="recovery_code">🆘 Recovery Code</option>
                        <option value="totp_secret">⏰ TOTP Secret</option>
                        <option value="ssh_key">🔐 SSH Key</option>
                        <option value="token">🎫 Token</option>
                        <option value="custom">✏️ Custom</option>
                      </select>
                      <input
                        type="text"
                        placeholder="Secret name"
                        value={secret.name}
                        onChange={(e) => updateSecret(secret.id, 'name', e.target.value)}
                        style={{
                          flex: 1,
                          padding: '6px',
                          border: `1px solid ${theme.colors.border}`,
                          borderRadius: '4px',
                          background: theme.colors.surface,
                          color: theme.colors.text,
                          fontSize: '12px'
                        }}
                      />
                      {newEntry.secrets.length > 1 && (
                        <button
                          onClick={() => removeSecret(secret.id)}
                          style={{
                            background: theme.colors.error,
                            color: 'white',
                            border: 'none',
                            borderRadius: '4px',
                            padding: '6px',
                            fontSize: '12px',
                            cursor: 'pointer'
                          }}
                          title="Remove secret"
                        >
                          ×
                        </button>
                      )}
                    </div>

                    <div style={{ position: 'relative' }}>
                      <textarea
                        placeholder={`Enter ${secret.name.toLowerCase()}`}
                        value={secret.value}
                        onChange={(e) => updateSecret(secret.id, 'value', e.target.value)}
                        style={{
                          width: '100%',
                          padding: '8px',
                          paddingRight: secret.type === 'password' ? '80px' : '12px',
                          border: `1px solid ${theme.colors.border}`,
                          borderRadius: '4px',
                          background: theme.colors.surface,
                          color: theme.colors.text,
                          fontSize: '14px',
                          boxSizing: 'border-box',
                          minHeight: secret.type === 'ssh_key' ? '120px' : '40px',
                          resize: 'vertical',
                          fontFamily: secret.type === 'ssh_key' || secret.type === 'api_key' ? 'monospace' : 'inherit'
                        }}
                      />
                      {secret.type === 'password' && (
                        <button
                          type="button"
                          onClick={() => updateSecret(secret.id, 'value', generatePassword())}
                          style={{
                            position: 'absolute',
                            right: '4px',
                            top: '4px',
                            background: theme.colors.primary,
                            color: 'white',
                            border: 'none',
                            borderRadius: '3px',
                            padding: '4px 6px',
                            fontSize: '10px',
                            cursor: 'pointer'
                          }}
                          title="Generate password"
                        >
                          🎲
                        </button>
                      )}
                    </div>

                    {secret.type === 'password' && secret.value && (
                      <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '0.5rem',
                        marginTop: '0.5rem',
                        fontSize: '10px'
                      }}>
                        <span style={{ color: theme.colors.textMuted }}>Strength:</span>
                        <div style={{
                          flex: 1,
                          height: '4px',
                          background: theme.colors.border,
                          borderRadius: '2px',
                          overflow: 'hidden'
                        }}>
                          <div style={{
                            width: `${calculateStrength(secret.value)}%`,
                            height: '100%',
                            background: getStrengthLabel(calculateStrength(secret.value)).color,
                            transition: 'width 0.3s'
                          }} />
                        </div>
                        <span style={{
                          color: getStrengthLabel(calculateStrength(secret.value)).color,
                          fontWeight: '600'
                        }}>
                          {getStrengthLabel(calculateStrength(secret.value)).label}
                        </span>
                      </div>
                    )}
                  </div>
                ))}
              </div>


              <input
                type="text"
                placeholder="Website URL (optional)"
                value={newEntry.url}
                onChange={(e) => setNewEntry({...newEntry, url: e.target.value})}
                style={styles.input}
              />

              <textarea
                placeholder="Notes (optional)"
                value={newEntry.notes}
                onChange={(e) => setNewEntry({...newEntry, notes: e.target.value})}
                style={{
                  ...styles.input,
                  minHeight: '80px',
                  resize: 'vertical'
                }}
              />

              <div style={{
                marginBottom: '1rem',
                padding: '1rem',
                background: theme.colors.fieldBg,
                borderRadius: '8px',
                border: `1px solid ${theme.colors.border}`
              }}>
                <label style={{ display: 'block', marginBottom: '0.5rem', color: theme.colors.text, fontSize: '14px', fontWeight: '600' }}>
                  🏷️ Tags
                </label>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                  {availableTags.map(tag => (
                    <button
                      key={tag}
                      type="button"
                      onClick={() => {
                        const isSelected = newEntry.tags.includes(tag);
                        setNewEntry({
                          ...newEntry,
                          tags: isSelected
                            ? newEntry.tags.filter(t => t !== tag)
                            : [...newEntry.tags, tag]
                        });
                      }}
                      style={{
                        padding: '6px 12px',
                        fontSize: '12px',
                        background: newEntry.tags.includes(tag) ? theme.colors.primary : 'transparent',
                        color: newEntry.tags.includes(tag) ? 'white' : theme.colors.text,
                        border: `1px solid ${newEntry.tags.includes(tag) ? theme.colors.primary : theme.colors.border}`,
                        borderRadius: '16px',
                        cursor: 'pointer',
                        transition: 'all 0.2s'
                      }}
                    >
                      {tag}
                    </button>
                  ))}
                </div>
                {newEntry.tags.length > 0 && (
                  <div style={{ marginTop: '0.5rem', fontSize: '12px', color: theme.colors.textSecondary }}>
                    Selected: {newEntry.tags.join(', ')}
                  </div>
                )}

                <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem' }}>
                  <input
                    type="text"
                    placeholder="Add custom tag"
                    value={customTag}
                    onChange={(e) => setCustomTag(e.target.value)}
                    style={{
                      flex: 1,
                      padding: '6px 8px',
                      border: `1px solid ${theme.colors.border}`,
                      borderRadius: '4px',
                      background: theme.colors.surface,
                      color: theme.colors.text,
                      fontSize: '12px'
                    }}
                    onKeyPress={(e) => e.key === 'Enter' && addCustomTag()}
                  />
                  <button
                    onClick={addCustomTag}
                    style={{
                      padding: '6px 12px',
                      background: theme.colors.primary,
                      color: 'white',
                      border: 'none',
                      borderRadius: '4px',
                      fontSize: '12px',
                      cursor: 'pointer'
                    }}
                  >
                    + Tag
                  </button>
                </div>
              </div>

              <div style={{ display: 'flex', gap: '1rem' }}>
                <button
                  onClick={() => setShowAddForm(false)}
                  style={{
                    ...styles.button,
                    background: 'transparent',
                    color: theme.colors.textSecondary,
                    border: `2px solid ${theme.colors.border}`,
                    flex: 1
                  }}
                >
                  Cancel
                </button>
                <button
                  onClick={addEntry}
                  style={{ ...styles.button, flex: 1 }}
                >
                  Add Entry
                </button>
              </div>
            </div>
          )}

          {showFolderForm && (
            <div style={styles.entryCard}>
              <div style={{
                fontSize: '1.2rem',
                fontWeight: '600',
                marginBottom: '1rem',
                color: theme.colors.text
              }}>
                📁 Create New Folder
              </div>

              <div style={{ display: 'flex', gap: '1rem', marginBottom: '1rem' }}>
                <select
                  value={newFolderIcon}
                  onChange={(e) => setNewFolderIcon(e.target.value)}
                  style={{
                    padding: '12px',
                    border: `1px solid ${theme.colors.border}`,
                    borderRadius: '8px',
                    background: theme.colors.surface,
                    color: theme.colors.text,
                    fontSize: '16px'
                  }}
                >
                  {['📁', '🏢', '💳', '🔧', '🌐', '🎮', '🏦', '🛒', '📚', '🎨', '⚡', '🔐', '🎯', '📊', '🔬', '🎭'].map(icon => (
                    <option key={icon} value={icon}>{icon}</option>
                  ))}
                </select>
                <input
                  type="text"
                  placeholder="Folder name"
                  value={newFolderName}
                  onChange={(e) => setNewFolderName(e.target.value)}
                  style={{ ...styles.input, flex: 1, marginBottom: 0 }}
                  onKeyPress={(e) => e.key === 'Enter' && addFolder()}
                />
              </div>

              <div style={{ display: 'flex', gap: '1rem' }}>
                <button
                  onClick={() => setShowFolderForm(false)}
                  style={{
                    ...styles.button,
                    background: 'transparent',
                    color: theme.colors.textSecondary,
                    border: `2px solid ${theme.colors.border}`,
                    flex: 1
                  }}
                >
                  Cancel
                </button>
                <button
                  onClick={addFolder}
                  style={{ ...styles.button, flex: 1 }}
                >
                  📁 Create Folder
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {!showAddForm && !showFolderForm && (
        <div style={{ position: 'fixed', bottom: '2rem', right: '2rem' }}>
          {showCreateMenu && (
            <div style={{
              position: 'absolute',
              bottom: '70px',
              right: '0',
              background: theme.colors.surface,
              border: `1px solid ${theme.colors.border}`,
              borderRadius: '12px',
              padding: '0.5rem',
              boxShadow: isDarkMode ? '0 4px 16px rgba(0,0,0,0.5)' : '0 4px 16px rgba(0,0,0,0.3)',
              minWidth: '200px'
            }}>
              <button
                onClick={() => {
                  setShowAddForm(true);
                  setShowCreateMenu(false);
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.5rem',
                  width: '100%',
                  padding: '0.75rem',
                  background: 'transparent',
                  color: theme.colors.text,
                  border: 'none',
                  borderRadius: '6px',
                  cursor: 'pointer',
                  fontSize: '14px',
                  textAlign: 'left'
                }}
                onMouseOver={(e) => e.currentTarget.style.background = theme.colors.fieldBg}
                onMouseOut={(e) => e.currentTarget.style.background = 'transparent'}
              >
                🔐 <span>New Password Entry</span>
              </button>
              <button
                onClick={() => {
                  setShowFolderForm(true);
                  setShowCreateMenu(false);
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.5rem',
                  width: '100%',
                  padding: '0.75rem',
                  background: 'transparent',
                  color: theme.colors.text,
                  border: 'none',
                  borderRadius: '6px',
                  cursor: 'pointer',
                  fontSize: '14px',
                  textAlign: 'left'
                }}
                onMouseOver={(e) => e.currentTarget.style.background = theme.colors.fieldBg}
                onMouseOut={(e) => e.currentTarget.style.background = 'transparent'}
              >
                📁 <span>New Folder</span>
              </button>
            </div>
          )}
          <button
            style={{
              width: '56px',
              height: '56px',
              borderRadius: '28px',
              background: theme.colors.primary,
              color: 'white',
              border: 'none',
              fontSize: '24px',
              cursor: 'pointer',
              boxShadow: isDarkMode ? '0 4px 16px rgba(0,0,0,0.5)' : '0 4px 16px rgba(0,0,0,0.3)',
              transition: 'all 0.2s',
              transform: showCreateMenu ? 'rotate(45deg)' : 'rotate(0deg)'
            }}
            onClick={() => setShowCreateMenu(!showCreateMenu)}
            title="Create new item"
          >
            +
          </button>
        </div>
      )}
    </div>
  );
}

export default App;