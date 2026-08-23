import React, { useState, useEffect, useCallback } from 'react';
import { useVault, VaultEntry } from '../contexts/VaultContext';
import { Globe, StickyNote, CreditCard, User, Star, Eye, EyeOff, Copy, ExternalLink, Shield, Plus, Lock, Search, PanelLeftClose, PanelLeft, Timer, Download, Upload, Paperclip, Settings2, CheckCircle } from 'lucide-react';
import toast from 'react-hot-toast';
import VaultEntryModal from './VaultEntryModal';
import FolderTree from './FolderTree';
import SecurityAudit from './SecurityAudit';
import ImportModal from './ImportModal';
import { totp, totpSecondsRemaining } from '../crypto/totp';
import { passwordStrength } from '../crypto/password-strength';

/** Live TOTP code display in the details panel. */
function TotpCodeDisplay({ secret }: { secret: string }) {
  const [code, setCode] = useState<string>('------');
  const [seconds, setSeconds] = useState(30);
  const [error, setError] = useState(false);

  const refresh = useCallback(async () => {
    if (!secret.trim()) return;
    try {
      setCode(await totp(secret.trim()));
      setSeconds(totpSecondsRemaining());
      setError(false);
    } catch {
      setError(true);
    }
  }, [secret]);

  useEffect(() => {
    refresh();
    const id = setInterval(() => {
      setSeconds(totpSecondsRemaining());
      if (totpSecondsRemaining() === 30) refresh();
    }, 1000);
    return () => clearInterval(id);
  }, [refresh]);

  const color = seconds <= 5 ? 'text-cloistr-error' : seconds <= 10 ? 'text-cloistr-warning' : 'text-cloistr-success';

  if (error) return <span className="text-cloistr-error text-sm">Invalid TOTP secret</span>;
  return (
    <div className="flex items-center gap-3">
      <span className={`text-2xl font-mono font-bold tracking-widest ${color}`} style={{ letterSpacing: '0.2em' }}>
        {code}
      </span>
      <span className={`text-sm ${color}`}>{seconds}s</span>
      <button
        onClick={() => navigator.clipboard.writeText(code).then(() => toast.success('Code copied'))}
        className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
        style={{ minHeight: '44px', minWidth: '44px' }}
        title="Copy code"
      >
        <Copy className="h-4 w-4" />
      </button>
    </div>
  );
}

/** Small strength badge next to a password entry in the list. */
function StrengthBadge({ password }: { password?: string }) {
  if (!password) return null;
  const { score } = passwordStrength(password);
  const colors = [
    'bg-red-500/20 text-red-600 dark:text-red-400',
    'bg-orange-500/20 text-orange-600 dark:text-orange-400',
    'bg-yellow-500/20 text-yellow-600 dark:text-yellow-400',
    'bg-green-500/20 text-green-600 dark:text-green-400',
    'bg-emerald-500/20 text-emerald-600 dark:text-emerald-400',
  ];
  const labels = ['VW', 'W', 'F', 'S', 'VS'];
  if (score >= 3) return null; // Don't clutter strong passwords
  return (
    <span className={`text-xs rounded px-1 py-0.5 font-medium ${colors[score]}`} title={['Very Weak','Weak','Fair','Strong','Very Strong'][score]}>
      {labels[score]}
    </span>
  );
}

export default function Dashboard() {
  const { vaultData, isLocked, saving, addEntry, updateEntry, deleteEntry, toggleFavorite } = useVault();
  const [selectedEntry, setSelectedEntry] = useState<VaultEntry | null>(null);
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});
  const [modalOpen, setModalOpen] = useState(false);
  const [editingEntry, setEditingEntry] = useState<VaultEntry | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState<string | null>(null);
  const [selectedFolderId, setSelectedFolderId] = useState<string | null>(null);
  const [showSidebar, setShowSidebar] = useState(true);
  // SEPARATE from showSidebar on purpose. showSidebar is the DESKTOP rail and
  // correctly defaults to open; a phone drawer must default to CLOSED. Driving
  // both from one boolean is what rendered the fixed 256px folder column over a
  // 375px viewport — 68% of the screen — with the entry list crushed beside it.
  const [mobileFoldersOpen, setMobileFoldersOpen] = useState(false);
  const [auditOpen, setAuditOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  // On small screens the details panel is shown by toggling — avoids the
  // two-column layout eating the full width.
  const [showDetails, setShowDetails] = useState(false);

  const filteredEntries = vaultData?.entries.filter(entry => {
    const matchesSearch = searchQuery === '' ||
      entry.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      entry.fields.username?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      entry.fields.url?.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesType = typeFilter === null || entry.type === typeFilter;
    const matchesFolder = selectedFolderId === null || entry.folder_id === selectedFolderId;
    return matchesSearch && matchesType && matchesFolder;
  }) || [];

  const selectedFolderName = selectedFolderId
    ? (vaultData?.folders.find(f => f.id === selectedFolderId)?.name ?? null)
    : null;

  const getEntryIcon = (type: string) => {
    switch (type) {
      case 'login': return Globe;
      case 'note': return StickyNote;
      case 'card': return CreditCard;
      case 'identity': return User;
      case 'totp': return Timer;
      default: return Globe;
    }
  };

  const togglePasswordVisibility = (entryId: string) => {
    setShowPasswords(prev => ({ ...prev, [entryId]: !prev[entryId] }));
  };

  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`${label} copied to clipboard`);
    } catch {
      toast.error('Failed to copy to clipboard');
    }
  };

  const handleAddEntry = () => { setEditingEntry(null); setModalOpen(true); };
  const handleEditEntry = (entry: VaultEntry) => { setEditingEntry(entry); setModalOpen(true); };

  const handleSaveEntry = async (entry: VaultEntry) => {
    try {
      if (!editingEntry) {
        await addEntry({
          type: entry.type,
          name: entry.name,
          fields: entry.fields,
          notes: entry.notes,
          favorite: entry.favorite,
          folder_id: entry.folder_id,
          custom_fields: entry.custom_fields,
          attachments: entry.attachments,
        });
      } else {
        await updateEntry(entry);
        setSelectedEntry(entry);
      }
    } catch (error) {
      console.error('Save error:', error);
    }
  };

  const handleDeleteEntry = async (id: string) => {
    try {
      await deleteEntry(id);
      setSelectedEntry(null);
      setShowDetails(false);
    } catch (error) {
      console.error('Delete error:', error);
    }
  };

  const handleToggleFavorite = async (entry: VaultEntry) => {
    try {
      await toggleFavorite(entry.id);
      setSelectedEntry(prev => prev?.id === entry.id ? { ...prev, favorite: !prev.favorite } : prev);
    } catch (error) {
      console.error('Favorite error:', error);
    }
  };

  const handleExport = () => {
    if (!vaultData) return;
    const exportData = {
      exported_at: new Date().toISOString(),
      entries: vaultData.entries.map(({ id, type, name, fields, notes, favorite, folder_id, created_at, updated_at, custom_fields, attachments }) => ({
        id, type, name, fields, notes, favorite, folder_id, created_at, updated_at, custom_fields, attachments,
      })),
    };
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `cloistr-vault-export-${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success('Vault exported');
  };

  const selectEntry = (entry: VaultEntry) => {
    setSelectedEntry(entry);
    setShowDetails(true);
  };

  if (isLocked) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <Lock className="h-12 w-12 text-cloistr-text-muted mx-auto mb-4" />
          <h2 className="text-lg font-semibold mb-2">Vault Locked</h2>
          <p className="text-cloistr-text-muted">Enter your master password to access your vault.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex gap-6">
      {/* Backdrop: mobile only, only while the folder drawer is open. Tap to close. */}
      {mobileFoldersOpen && (
        <div
          className="fixed inset-0 z-[var(--cloistr-z-drawer-backdrop,60)] bg-black/50 md:hidden"
          aria-hidden="true"
          onClick={() => setMobileFoldersOpen(false)}
        />
      )}

      {/* Folder Sidebar.
          Below md: an off-canvas drawer. `fixed` takes it OUT of flow, which is
          what stops it stealing 68% of a phone viewport, and it sits on the
          shared drawer layer so the sticky header cannot paint over its top.
          At md+: `md:static` puts it back as the in-flow rail that showSidebar
          controls, unchanged. */}
      <div
        className={[
          'w-64 flex-shrink-0',
          'fixed inset-y-0 left-0 z-[var(--cloistr-z-drawer,70)] overflow-y-auto bg-cloistr-bg p-2',
          'transition-transform duration-200 ease-out',
          mobileFoldersOpen ? 'translate-x-0' : '-translate-x-full',
          'md:static md:z-auto md:translate-x-0 md:overflow-visible md:bg-transparent md:p-0',
          showSidebar ? 'md:block' : 'md:hidden',
        ].join(' ')}
      >
        <div>
          <div className="card sticky top-4">
            <div className="card-header py-3 px-4 flex items-center justify-between">
              <h3 className="text-sm font-medium">Folders</h3>
              <button
                onClick={() => { setShowSidebar(false); setMobileFoldersOpen(false); }}
                className="p-1 hover:bg-cloistr-bg-hover rounded"
                style={{ minHeight: '44px', minWidth: '44px' }}
                title="Hide sidebar"
              >
                <PanelLeftClose className="h-4 w-4 text-cloistr-text-muted" />
              </button>
            </div>
            <div className="card-content p-0">
              <FolderTree selectedFolderId={selectedFolderId} onSelectFolder={setSelectedFolderId} />
            </div>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 min-w-0 space-y-4">
        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3 min-w-0">
            {/* Mobile: the only way to reach the folder drawer, which is off-canvas. */}
            <button
              onClick={() => setMobileFoldersOpen(true)}
              className="p-2 hover:bg-cloistr-bg-hover rounded md:hidden"
              aria-label="Show folders"
              aria-expanded={mobileFoldersOpen}
            >
              <PanelLeft className="h-4 w-4" />
            </button>
            {!showSidebar && (
              <button
                onClick={() => setShowSidebar(true)}
                className="hidden p-2 hover:bg-cloistr-bg-hover rounded md:block"
                style={{ minHeight: '44px', minWidth: '44px' }}
                title="Show folders"
              >
                <PanelLeft className="h-4 w-4" />
              </button>
            )}
            <div className="min-w-0">
              <h1 className="text-xl sm:text-2xl font-bold text-cloistr-text truncate">
                {selectedFolderName || 'Your Vault'}
              </h1>
              <p className="text-cloistr-text-muted text-sm">
                {filteredEntries.length} item{filteredEntries.length !== 1 ? 's' : ''}
                {selectedFolderId ? ' in folder' : ''} — encrypted locally
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <button
              onClick={() => setImportOpen(true)}
              className="btn-outline text-sm"
              style={{ minHeight: '44px' }}
              title="Import entries"
            >
              <Upload className="h-4 w-4 mr-1 sm:mr-2" />
              <span className="hidden sm:inline">Import</span>
            </button>
            {vaultData && vaultData.entries.length > 0 && (
              <>
                <button
                  onClick={handleExport}
                  className="btn-outline text-sm"
                  style={{ minHeight: '44px' }}
                  title="Export vault"
                >
                  <Download className="h-4 w-4 mr-1 sm:mr-2" />
                  <span className="hidden sm:inline">Export</span>
                </button>
                <button
                  onClick={() => setAuditOpen(true)}
                  className="btn-outline text-sm"
                  style={{ minHeight: '44px' }}
                  title="Security audit"
                >
                  <Shield className="h-4 w-4 mr-1 sm:mr-2" />
                  <span className="hidden sm:inline">Audit</span>
                </button>
              </>
            )}
            <button
              onClick={handleAddEntry}
              className="btn-primary text-sm"
              disabled={saving}
              style={{ minHeight: '44px' }}
            >
              <Plus className="h-4 w-4 mr-1 sm:mr-2" />
              Add
            </button>
          </div>
        </div>

        {/* Search and Filter */}
        <div className="flex flex-col gap-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-cloistr-text-muted" />
            <input
              type="text"
              placeholder="Search vault..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input w-full pl-10"
              style={{ minHeight: '44px' }}
            />
          </div>
          <div className="flex gap-2 overflow-x-auto pb-1">
            {[
              [null, 'All'],
              ['login', 'Logins'],
              ['note', 'Notes'],
              ['card', 'Cards'],
              ['identity', 'IDs'],
              ['totp', 'TOTP'],
            ].map(([val, label]) => (
              <button
                key={String(val)}
                onClick={() => setTypeFilter(val as string | null)}
                style={{ minHeight: '44px' }}
                className={`px-3 py-1 rounded-md text-sm whitespace-nowrap transition-colors flex-shrink-0 ${
                  typeFilter === val
                    ? 'bg-cloistr-primary text-white'
                    : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {[
            { Icon: Globe, label: 'Logins', color: 'text-cloistr-info', count: vaultData?.entries.filter(e => e.type === 'login').length || 0 },
            { Icon: StickyNote, label: 'Notes', color: 'text-cloistr-success', count: vaultData?.entries.filter(e => e.type === 'note').length || 0 },
            { Icon: CreditCard, label: 'Cards', color: 'text-cloistr-primary', count: vaultData?.entries.filter(e => e.type === 'card').length || 0 },
            { Icon: Star, label: 'Favorites', color: 'text-cloistr-warning', count: vaultData?.entries.filter(e => e.favorite).length || 0 },
          ].map(({ Icon, label, color, count }) => (
            <div key={label} className="card">
              <div className="card-content p-4">
                <div className="flex items-center space-x-2">
                  <Icon className={`h-5 w-5 ${color}`} />
                  <div>
                    <p className="text-xs text-cloistr-text-muted">{label}</p>
                    <p className="text-2xl font-bold">{count}</p>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Vault Items + Details */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Items List */}
          <div className="space-y-3">
            <h2 className="text-lg font-semibold">Vault Items</h2>

            {vaultData?.entries.length === 0 ? (
              <div className="card">
                <div className="card-content p-8 text-center">
                  <Shield className="h-12 w-12 text-cloistr-text-muted mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">Your vault is empty</h3>
                  <p className="text-cloistr-text-muted mb-4">
                    Start adding passwords, notes, and other items to secure them with zero-knowledge encryption.
                  </p>
                  <div className="flex flex-col sm:flex-row gap-2 justify-center">
                    <button className="btn-primary" onClick={handleAddEntry} style={{ minHeight: '44px' }}>
                      Add your first item
                    </button>
                    <button className="btn-outline" onClick={() => setImportOpen(true)} style={{ minHeight: '44px' }}>
                      <Upload className="h-4 w-4 mr-2" />
                      Import from KeePass
                    </button>
                  </div>
                </div>
              </div>
            ) : filteredEntries.length === 0 ? (
              <div className="card">
                <div className="card-content p-8 text-center">
                  <Search className="h-12 w-12 text-cloistr-text-muted mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">No results found</h3>
                  <button
                    className="btn-outline"
                    style={{ minHeight: '44px' }}
                    onClick={() => { setSearchQuery(''); setTypeFilter(null); }}
                  >
                    Clear filters
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-1">
                {filteredEntries.map((entry) => {
                  const IconComponent = getEntryIcon(entry.type);
                  const isSelected = selectedEntry?.id === entry.id;
                  return (
                    <div
                      key={entry.id}
                      className={`vault-item ${isSelected ? 'bg-cloistr-bg-hover ring-1 ring-cloistr-primary/30' : ''}`}
                      onClick={() => selectEntry(entry)}
                    >
                      <div className="vault-item-info min-w-0 flex-1">
                        <div className="vault-item-icon flex-shrink-0">
                          <IconComponent className="h-4 w-4 text-cloistr-primary" />
                        </div>
                        <div className="vault-item-content min-w-0">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            <span className="vault-item-title truncate">{entry.name}</span>
                            {entry.favorite && <Star className="h-3 w-3 text-cloistr-warning fill-current flex-shrink-0" />}
                            {entry.type === 'login' && <StrengthBadge password={entry.fields.password} />}
                          </div>
                          <span className="vault-item-subtitle truncate">
                            {entry.type === 'totp'
                              ? (entry.fields.issuer || 'TOTP')
                              : (entry.fields.username || entry.fields.url || entry.type)}
                          </span>
                        </div>
                      </div>

                      {/* Quick-copy buttons — autofill-friendly affordances */}
                      <div className="flex items-center gap-1 flex-shrink-0 ml-2">
                        {entry.fields.username && (
                          <button
                            onClick={(e) => { e.stopPropagation(); copyToClipboard(entry.fields.username, 'Username'); }}
                            className="p-1 text-cloistr-text-muted hover:text-cloistr-text opacity-0 group-hover:opacity-100 transition-opacity"
                            style={{ minHeight: '44px', minWidth: '36px' }}
                            title="Copy username"
                          >
                            <User className="h-3 w-3" />
                          </button>
                        )}
                        {entry.fields.password && (
                          <button
                            onClick={(e) => { e.stopPropagation(); copyToClipboard(entry.fields.password, 'Password'); }}
                            className="p-2 text-cloistr-text-muted hover:text-cloistr-text"
                            style={{ minHeight: '44px', minWidth: '36px' }}
                            title="Copy password"
                          >
                            <Copy className="h-3.5 w-3.5" />
                          </button>
                        )}
                      </div>

                      <div className="text-xs text-cloistr-text-muted flex-shrink-0 hidden sm:block">
                        {entry.type === 'totp' ? 'TOTP' : entry.type.charAt(0).toUpperCase() + entry.type.slice(1)}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Item Details */}
          <div className={`space-y-3 ${!showDetails ? 'hidden lg:block' : ''}`}>
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold">Item Details</h2>
              {showDetails && (
                <button
                  onClick={() => { setShowDetails(false); setSelectedEntry(null); }}
                  className="text-sm text-cloistr-text-muted underline lg:hidden"
                  style={{ minHeight: '44px' }}
                >
                  Back to list
                </button>
              )}
            </div>

            {selectedEntry ? (
              <div className="card">
                <div className="card-header pb-3">
                  <div className="flex items-center space-x-2">
                    {React.createElement(getEntryIcon(selectedEntry.type), {
                      className: "h-5 w-5 text-cloistr-primary flex-shrink-0"
                    })}
                    <div className="min-w-0">
                      <h3 className="card-title text-lg truncate">{selectedEntry.name}</h3>
                      <p className="card-description">
                        {selectedEntry.type === 'totp' ? 'TOTP Authenticator' : selectedEntry.type.charAt(0).toUpperCase() + selectedEntry.type.slice(1)}
                      </p>
                    </div>
                  </div>
                </div>

                <div className="card-content space-y-4">
                  {/* TOTP live display */}
                  {selectedEntry.type === 'totp' && selectedEntry.fields.secret && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Current Code</label>
                      <TotpCodeDisplay secret={selectedEntry.fields.secret} />
                    </div>
                  )}

                  {/* Standard fields */}
                  {Object.entries(selectedEntry.fields)
                    .filter(([key]) => !(selectedEntry.type === 'totp' && key === 'secret'))
                    .map(([key, value]) => (
                    <div key={key} className="space-y-1">
                      <label className="text-sm font-medium capitalize">
                        {key.replace(/([A-Z])/g, ' $1').trim()}
                      </label>
                      <div className="flex items-center space-x-2">
                        {key.toLowerCase().includes('password') || key.toLowerCase() === 'cvv' ? (
                          <div className="flex-1 relative">
                            <input
                              type={showPasswords[selectedEntry.id] ? 'text' : 'password'}
                              value={value}
                              readOnly
                              className="input w-full pr-20"
                            />
                            <div className="absolute inset-y-0 right-0 flex items-center space-x-1 pr-2">
                              <button
                                onClick={() => togglePasswordVisibility(selectedEntry.id)}
                                className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                                style={{ minHeight: '32px', minWidth: '32px' }}
                              >
                                {showPasswords[selectedEntry.id] ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                              </button>
                              <button
                                onClick={() => copyToClipboard(value, key)}
                                className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                                style={{ minHeight: '32px', minWidth: '32px' }}
                              >
                                <Copy className="h-4 w-4" />
                              </button>
                            </div>
                          </div>
                        ) : (
                          <div className="flex-1 relative">
                            <input
                              type="text"
                              value={value}
                              readOnly
                              className="input w-full pr-12"
                            />
                            <div className="absolute inset-y-0 right-0 flex items-center space-x-1 pr-2">
                              <button
                                onClick={() => copyToClipboard(value, key)}
                                className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                                style={{ minHeight: '32px', minWidth: '32px' }}
                              >
                                <Copy className="h-4 w-4" />
                              </button>
                              {key.toLowerCase() === 'url' && value && (
                                <button
                                  onClick={() => window.open(value.startsWith('http') ? value : `https://${value}`, '_blank')}
                                  className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                                  style={{ minHeight: '32px', minWidth: '32px' }}
                                >
                                  <ExternalLink className="h-4 w-4" />
                                </button>
                              )}
                            </div>
                          </div>
                        )}
                      </div>
                      {/* Password strength inline */}
                      {key.toLowerCase().includes('password') && value && (() => {
                        const { score, label } = passwordStrength(value);
                        const colors = ['text-red-500','text-orange-500','text-yellow-500','text-green-500','text-emerald-500'];
                        const bars = ['bg-red-500','bg-orange-500','bg-yellow-500','bg-green-500','bg-emerald-500'];
                        return (
                          <div className="space-y-1">
                            <div className="flex gap-1">
                              {[0,1,2,3,4].map(i => (
                                <div key={i} className={`h-1 flex-1 rounded-full ${i <= score ? bars[score] : 'bg-cloistr-border'}`} />
                              ))}
                            </div>
                            <span className={`text-xs ${colors[score]}`}>{label}</span>
                          </div>
                        );
                      })()}
                    </div>
                  ))}

                  {/* Custom fields */}
                  {selectedEntry.custom_fields && selectedEntry.custom_fields.length > 0 && (
                    <div className="space-y-3 pt-2 border-t border-cloistr-border">
                      <p className="text-xs font-medium text-cloistr-text-muted uppercase tracking-wide">Custom Fields</p>
                      {selectedEntry.custom_fields.map((cf, i) => (
                        <div key={i} className="space-y-1">
                          <label className="text-sm font-medium">{cf.label}</label>
                          <div className="flex items-center gap-2">
                            <input type="text" value={cf.value} readOnly className="input flex-1 pr-10" />
                            <button
                              onClick={() => copyToClipboard(cf.value, cf.label)}
                              className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                              style={{ minHeight: '32px', minWidth: '32px' }}
                            >
                              <Copy className="h-4 w-4" />
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Attachments */}
                  {selectedEntry.attachments && selectedEntry.attachments.length > 0 && (
                    <div className="space-y-2 pt-2 border-t border-cloistr-border">
                      <p className="text-xs font-medium text-cloistr-text-muted uppercase tracking-wide">Attachments</p>
                      {selectedEntry.attachments.map((att, i) => (
                        <div key={i} className="flex items-center gap-2 p-2 rounded border border-cloistr-border">
                          <Paperclip className="h-4 w-4 text-cloistr-text-muted flex-shrink-0" />
                          <span className="text-sm flex-1 truncate">{att.name}</span>
                          <button
                            onClick={() => {
                              const a = document.createElement('a');
                              a.href = `data:${att.mime};base64,${att.data}`;
                              a.download = att.name;
                              document.body.appendChild(a);
                              a.click();
                              document.body.removeChild(a);
                            }}
                            className="text-xs text-cloistr-primary underline flex-shrink-0"
                            style={{ minHeight: '44px' }}
                          >
                            Download
                          </button>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Notes */}
                  {selectedEntry.notes && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Notes</label>
                      <textarea value={selectedEntry.notes} readOnly className="input w-full h-20 resize-none" />
                    </div>
                  )}

                  {/* Actions */}
                  <div className="flex flex-wrap gap-2 pt-4 border-t border-cloistr-border">
                    <button
                      className="btn-outline flex-1"
                      style={{ minHeight: '44px' }}
                      onClick={() => handleEditEntry(selectedEntry)}
                      disabled={saving}
                    >
                      Edit
                    </button>
                    <button
                      className="btn-outline"
                      style={{ minHeight: '44px' }}
                      onClick={() => handleToggleFavorite(selectedEntry)}
                      disabled={saving}
                    >
                      {selectedEntry.favorite ? (
                        <><Star className="h-4 w-4 mr-2 fill-current text-cloistr-warning" />Unfavorite</>
                      ) : (
                        <><Star className="h-4 w-4 mr-2" />Favorite</>
                      )}
                    </button>
                    {selectedEntry.fields.username && (
                      <button
                        className="btn-outline"
                        style={{ minHeight: '44px' }}
                        onClick={() => copyToClipboard(selectedEntry.fields.username, 'Username')}
                        title="Copy username"
                      >
                        <User className="h-4 w-4 mr-1" />
                        <span className="hidden sm:inline">Copy user</span>
                      </button>
                    )}
                    {selectedEntry.fields.password && (
                      <button
                        className="btn-primary"
                        style={{ minHeight: '44px' }}
                        onClick={() => copyToClipboard(selectedEntry.fields.password, 'Password')}
                        title="Copy password"
                      >
                        <Copy className="h-4 w-4 mr-1" />
                        <span className="hidden sm:inline">Copy password</span>
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ) : (
              <div className="card hidden lg:block">
                <div className="card-content p-8 text-center">
                  <div className="h-12 w-12 bg-cloistr-bg-hover rounded-full flex items-center justify-center mx-auto mb-4">
                    <Globe className="h-6 w-6 text-cloistr-text-muted" />
                  </div>
                  <h3 className="text-lg font-semibold mb-2">Select an item</h3>
                  <p className="text-cloistr-text-muted">Choose an item from your vault to view its details.</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Modals */}
      <VaultEntryModal
        isOpen={modalOpen}
        onClose={() => { setModalOpen(false); setEditingEntry(null); }}
        onSave={handleSaveEntry}
        onDelete={handleDeleteEntry}
        entry={editingEntry}
        mode={editingEntry ? 'edit' : 'add'}
      />

      <SecurityAudit
        isOpen={auditOpen}
        onClose={() => setAuditOpen(false)}
        onEditEntry={(entry) => { handleEditEntry(entry); }}
      />

      <ImportModal
        isOpen={importOpen}
        onClose={() => setImportOpen(false)}
      />
    </div>
  );
}
