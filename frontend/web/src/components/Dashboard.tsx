import React, { useState, useEffect, useCallback } from 'react';
import { useVault, VaultEntry } from '../contexts/VaultContext';
import { Globe, StickyNote, CreditCard, User, Star, Eye, EyeOff, Copy, ExternalLink, Shield, Plus, Lock, Search, PanelLeftClose, PanelLeft, Timer, Download } from 'lucide-react';
import toast from 'react-hot-toast';
import VaultEntryModal from './VaultEntryModal';
import FolderTree from './FolderTree';
import { totp, totpSecondsRemaining } from '../crypto/totp';

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

  if (error) {
    return <span className="text-cloistr-error text-sm">Invalid TOTP secret</span>;
  }
  return (
    <div className="flex items-center gap-3">
      <span className={`text-2xl font-mono font-bold tracking-widest ${color}`} style={{ letterSpacing: '0.2em' }}>
        {code}
      </span>
      <span className={`text-sm ${color}`}>{seconds}s</span>
      <button
        onClick={() => navigator.clipboard.writeText(code).then(() => toast.success('Code copied'))}
        className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
        title="Copy code"
      >
        <Copy className="h-4 w-4" />
      </button>
    </div>
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

  // Filter entries based on search query, type filter, and folder
  const filteredEntries = vaultData?.entries.filter(entry => {
    const matchesSearch = searchQuery === '' ||
      entry.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      entry.fields.username?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      entry.fields.url?.toLowerCase().includes(searchQuery.toLowerCase());

    const matchesType = typeFilter === null || entry.type === typeFilter;

    const matchesFolder = selectedFolderId === null || entry.folder_id === selectedFolderId;

    return matchesSearch && matchesType && matchesFolder;
  }) || [];

  // Folder name for the header title — read directly from vaultData (same blob source as entries)
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
    setShowPasswords(prev => ({
      ...prev,
      [entryId]: !prev[entryId]
    }));
  };

  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`${label} copied to clipboard`);
    } catch {
      toast.error('Failed to copy to clipboard');
    }
  };

  const handleAddEntry = () => {
    setEditingEntry(null);
    setModalOpen(true);
  };

  const handleEditEntry = (entry: VaultEntry) => {
    setEditingEntry(entry);
    setModalOpen(true);
  };

  const handleSaveEntry = async (entry: VaultEntry) => {
    try {
      const isNew = !editingEntry;
      if (isNew) {
        await addEntry({
          type: entry.type,
          name: entry.name,
          fields: entry.fields,
          notes: entry.notes,
          favorite: entry.favorite,
          folder_id: entry.folder_id,
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

  /**
   * Export the decrypted vault entries as JSON.
   * Pure frontend operation — the vault is already in memory.
   * Uses a Blob URL triggered from a hidden <a> since the artifact CSP blocks
   * script-driven saves; on the live app this works normally.
   */
  const handleExport = () => {
    if (!vaultData) return;
    const exportData = {
      exported_at: new Date().toISOString(),
      entries: vaultData.entries.map(({ id, type, name, fields, notes, favorite, folder_id, created_at, updated_at }) => ({
        id, type, name, fields, notes, favorite, folder_id, created_at, updated_at,
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

  if (isLocked) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <Lock className="h-12 w-12 text-cloistr-text-muted mx-auto mb-4" />
          <h2 className="text-lg font-semibold mb-2">Vault Locked</h2>
          <p className="text-cloistr-text-muted">
            Enter your master password to access your vault.
          </p>
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
                title="Hide sidebar"
              >
                <PanelLeftClose className="h-4 w-4 text-cloistr-text-muted" />
              </button>
            </div>
            <div className="card-content p-0">
              <FolderTree
                selectedFolderId={selectedFolderId}
                onSelectFolder={setSelectedFolderId}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
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
                title="Show folders"
              >
                <PanelLeft className="h-4 w-4" />
              </button>
            )}
            <div>
              <h1 className="text-2xl font-bold text-cloistr-text">
                {selectedFolderName || 'Your Vault'}
              </h1>
              <p className="text-cloistr-text-muted">
                {filteredEntries.length} item{filteredEntries.length !== 1 ? 's' : ''}
                {selectedFolderId ? ' in this folder' : ''} - All data encrypted locally
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {vaultData && vaultData.entries.length > 0 && (
              <button
                onClick={handleExport}
                className="btn-outline"
                title="Export vault as JSON"
              >
                <Download className="h-4 w-4 mr-2" />
                Export
              </button>
            )}
            <button
              onClick={handleAddEntry}
              className="btn-primary"
              disabled={saving}
            >
              <Plus className="h-4 w-4 mr-2" />
              Add Item
            </button>
          </div>
        </div>

        {/* Search and Filter */}
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-cloistr-text-muted" />
            <input
              type="text"
              placeholder="Search vault..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input w-full pl-10"
            />
          </div>
          <div className="flex gap-2 flex-wrap">
            <button
              onClick={() => setTypeFilter(null)}
              className={`px-3 py-2 rounded-md text-sm transition-colors ${
                typeFilter === null
                  ? 'bg-cloistr-primary text-white'
                  : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
              }`}
            >
              All
            </button>
            <button
              onClick={() => setTypeFilter('login')}
              className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
                typeFilter === 'login'
                  ? 'bg-cloistr-primary text-white'
                  : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
              }`}
            >
              <Globe className="h-3 w-3" /> Logins
            </button>
            <button
              onClick={() => setTypeFilter('note')}
              className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
                typeFilter === 'note'
                  ? 'bg-cloistr-primary text-white'
                  : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
              }`}
            >
              <StickyNote className="h-3 w-3" /> Notes
            </button>
            <button
              onClick={() => setTypeFilter('card')}
              className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
                typeFilter === 'card'
                  ? 'bg-cloistr-primary text-white'
                  : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
              }`}
            >
              <CreditCard className="h-3 w-3" /> Cards
            </button>
            <button
              onClick={() => setTypeFilter('identity')}
              className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
                typeFilter === 'identity'
                  ? 'bg-cloistr-primary text-white'
                  : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
              }`}
            >
              <User className="h-3 w-3" /> Identities
            </button>
            <button
              onClick={() => setTypeFilter('totp')}
              className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
                typeFilter === 'totp'
                  ? 'bg-cloistr-primary text-white'
                  : 'bg-cloistr-bg-hover hover:bg-cloistr-bg-hover/80'
              }`}
            >
              <Timer className="h-3 w-3" /> TOTP
            </button>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="card">
            <div className="card-content p-4">
              <div className="flex items-center space-x-2">
                <Globe className="h-5 w-5 text-cloistr-info" />
                <div>
                  <p className="text-sm text-cloistr-text-muted">Logins</p>
                  <p className="text-2xl font-bold">
                    {vaultData?.entries.filter(e => e.type === 'login').length || 0}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <div className="card">
            <div className="card-content p-4">
              <div className="flex items-center space-x-2">
                <StickyNote className="h-5 w-5 text-cloistr-success" />
                <div>
                  <p className="text-sm text-cloistr-text-muted">Notes</p>
                  <p className="text-2xl font-bold">
                    {vaultData?.entries.filter(e => e.type === 'note').length || 0}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <div className="card">
            <div className="card-content p-4">
              <div className="flex items-center space-x-2">
                <CreditCard className="h-5 w-5 text-cloistr-primary" />
                <div>
                  <p className="text-sm text-cloistr-text-muted">Cards</p>
                  <p className="text-2xl font-bold">
                    {vaultData?.entries.filter(e => e.type === 'card').length || 0}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <div className="card">
            <div className="card-content p-4">
              <div className="flex items-center space-x-2">
                <Star className="h-5 w-5 text-cloistr-warning" />
                <div>
                  <p className="text-sm text-cloistr-text-muted">Favorites</p>
                  <p className="text-2xl font-bold">
                    {vaultData?.entries.filter(e => e.favorite).length || 0}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Vault Items */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Items List */}
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Vault Items</h2>

            {vaultData?.entries.length === 0 ? (
              <div className="card">
                <div className="card-content p-8 text-center">
                  <Shield className="h-12 w-12 text-cloistr-text-muted mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">Your vault is empty</h3>
                  <p className="text-cloistr-text-muted mb-4">
                    Start adding passwords, notes, and other items to secure them with zero-knowledge encryption.
                  </p>
                  <button className="btn-primary" onClick={handleAddEntry}>
                    Add your first item
                  </button>
                </div>
              </div>
            ) : filteredEntries.length === 0 ? (
              <div className="card">
                <div className="card-content p-8 text-center">
                  <Search className="h-12 w-12 text-cloistr-text-muted mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">No results found</h3>
                  <p className="text-cloistr-text-muted mb-4">
                    Try adjusting your search or filter criteria.
                  </p>
                  <button
                    className="btn-outline"
                    onClick={() => {
                      setSearchQuery('');
                      setTypeFilter(null);
                    }}
                  >
                    Clear filters
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-2">
                {filteredEntries.map((entry) => {
                  const IconComponent = getEntryIcon(entry.type);
                  return (
                    <div
                      key={entry.id}
                      className={`vault-item ${selectedEntry?.id === entry.id ? 'bg-cloistr-bg-hover' : ''}`}
                      onClick={() => setSelectedEntry(entry)}
                    >
                      <div className="vault-item-info">
                        <div className="vault-item-icon">
                          <IconComponent className="h-4 w-4 text-cloistr-primary" />
                        </div>

                        <div className="vault-item-content">
                          <div className="flex items-center space-x-2">
                            <span className="vault-item-title">{entry.name}</span>
                            {entry.favorite && (
                              <Star className="h-3 w-3 text-cloistr-warning fill-current" />
                            )}
                          </div>
                          <span className="vault-item-subtitle">
                            {entry.type === 'totp'
                              ? (entry.fields.issuer || 'TOTP')
                              : (entry.fields.username || entry.fields.url || entry.type)}
                          </span>
                        </div>
                      </div>

                      <div className="text-xs text-cloistr-text-muted">
                        {entry.type === 'totp' ? 'TOTP' : entry.type.charAt(0).toUpperCase() + entry.type.slice(1)}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Item Details */}
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Item Details</h2>

            {selectedEntry ? (
              <div className="card">
                <div className="card-header">
                  <div className="flex items-center space-x-2">
                    {React.createElement(getEntryIcon(selectedEntry.type), {
                      className: "h-5 w-5 text-cloistr-primary"
                    })}
                    <div>
                      <h3 className="card-title text-lg">{selectedEntry.name}</h3>
                      <p className="card-description">
                        {selectedEntry.type === 'totp' ? 'TOTP Authenticator' : selectedEntry.type.charAt(0).toUpperCase() + selectedEntry.type.slice(1)}
                      </p>
                    </div>
                  </div>
                </div>

                <div className="card-content space-y-4">
                  {/* TOTP live code display */}
                  {selectedEntry.type === 'totp' && selectedEntry.fields.secret && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Current Code</label>
                      <TotpCodeDisplay secret={selectedEntry.fields.secret} />
                    </div>
                  )}

                  {/* Fields (skip 'secret' for TOTP — shown above as live code) */}
                  {Object.entries(selectedEntry.fields)
                    .filter(([key]) => !(selectedEntry.type === 'totp' && key === 'secret'))
                    .map(([key, value]) => (
                    <div key={key} className="space-y-2">
                      <label className="text-sm font-medium capitalize">
                        {key.replace(/([A-Z])/g, ' $1').trim()}
                      </label>
                      <div className="flex items-center space-x-2">
                        {key.toLowerCase().includes('password') ? (
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
                              >
                                {showPasswords[selectedEntry.id] ? (
                                  <EyeOff className="h-4 w-4" />
                                ) : (
                                  <Eye className="h-4 w-4" />
                                )}
                              </button>
                              <button
                                onClick={() => copyToClipboard(value, key)}
                                className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
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
                              >
                                <Copy className="h-4 w-4" />
                              </button>
                              {key.toLowerCase() === 'url' && value && (
                                <button
                                  onClick={() => window.open(value.startsWith('http') ? value : `https://${value}`, '_blank')}
                                  className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                                >
                                  <ExternalLink className="h-4 w-4" />
                                </button>
                              )}
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  ))}

                  {/* Notes */}
                  {selectedEntry.notes && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Notes</label>
                      <textarea
                        value={selectedEntry.notes}
                        readOnly
                        className="input w-full h-20 resize-none"
                      />
                    </div>
                  )}

                  {/* Actions */}
                  <div className="flex space-x-2 pt-4 border-t border-cloistr-border">
                    <button
                      className="btn-outline flex-1"
                      onClick={() => handleEditEntry(selectedEntry)}
                      disabled={saving}
                    >
                      Edit
                    </button>
                    <button
                      className="btn-outline"
                      onClick={() => handleToggleFavorite(selectedEntry)}
                      disabled={saving}
                    >
                      {selectedEntry.favorite ? (
                        <>
                          <Star className="h-4 w-4 mr-2 fill-current text-cloistr-warning" />
                          Unfavorite
                        </>
                      ) : (
                        <>
                          <Star className="h-4 w-4 mr-2" />
                          Favorite
                        </>
                      )}
                    </button>
                  </div>
                </div>
              </div>
            ) : (
              <div className="card">
                <div className="card-content p-8 text-center">
                  <div className="h-12 w-12 bg-cloistr-bg-hover rounded-full flex items-center justify-center mx-auto mb-4">
                    <Globe className="h-6 w-6 text-cloistr-text-muted" />
                  </div>
                  <h3 className="text-lg font-semibold mb-2">Select an item</h3>
                  <p className="text-cloistr-text-muted">
                    Choose an item from your vault to view its details here.
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Add/Edit Modal */}
        <VaultEntryModal
          isOpen={modalOpen}
          onClose={() => {
            setModalOpen(false);
            setEditingEntry(null);
          }}
          onSave={handleSaveEntry}
          onDelete={handleDeleteEntry}
          entry={editingEntry}
          mode={editingEntry ? 'edit' : 'add'}
        />
      </div>
    </div>
  );
}
