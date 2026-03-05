import React, { useState } from 'react';
import { useVault, VaultEntry } from '../contexts/VaultContext';
import { Globe, StickyNote, CreditCard, User, Star, Eye, EyeOff, Copy, ExternalLink, Shield, Plus, Lock, Search } from 'lucide-react';
import toast from 'react-hot-toast';
import VaultEntryModal from './VaultEntryModal';

export default function Dashboard() {
  const { vaultData, isLocked, saving, addEntry, updateEntry, deleteEntry, toggleFavorite } = useVault();
  const [selectedEntry, setSelectedEntry] = useState<VaultEntry | null>(null);
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});
  const [modalOpen, setModalOpen] = useState(false);
  const [editingEntry, setEditingEntry] = useState<VaultEntry | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState<string | null>(null);

  // Filter entries based on search query and type filter
  const filteredEntries = vaultData?.entries.filter(entry => {
    const matchesSearch = searchQuery === '' ||
      entry.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      entry.fields.username?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      entry.fields.url?.toLowerCase().includes(searchQuery.toLowerCase());

    const matchesType = typeFilter === null || entry.type === typeFilter;

    return matchesSearch && matchesType;
  }) || [];

  const getEntryIcon = (type: string) => {
    switch (type) {
      case 'login':
        return Globe;
      case 'note':
        return StickyNote;
      case 'card':
        return CreditCard;
      case 'identity':
        return User;
      default:
        return Globe;
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
    } catch (error) {
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
        // New entry - use addEntry which generates id and timestamps
        await addEntry({
          type: entry.type,
          name: entry.name,
          fields: entry.fields,
          notes: entry.notes,
          favorite: entry.favorite,
          folder_id: entry.folder_id,
        });
      } else {
        // Editing existing entry
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
      // Update selected entry to reflect the change
      setSelectedEntry(prev => prev?.id === entry.id ? { ...prev, favorite: !prev.favorite } : prev);
    } catch (error) {
      console.error('Favorite error:', error);
    }
  };

  // Show locked state message if vault is locked
  if (isLocked) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <Lock className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
          <h2 className="text-lg font-semibold mb-2">Vault Locked</h2>
          <p className="text-muted-foreground">
            Enter your master password to access your vault.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Your Vault</h1>
          <p className="text-muted-foreground">
            {vaultData?.entries.length || 0} items - All data encrypted locally
          </p>
        </div>
        <button
          onClick={handleAddEntry}
          className="btn-primary"
          disabled={saving}
        >
          <Plus className="h-4 w-4 mr-2" />
          Add Item
        </button>
      </div>

      {/* Search and Filter */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search vault..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="input w-full pl-10"
          />
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setTypeFilter(null)}
            className={`px-3 py-2 rounded-md text-sm transition-colors ${
              typeFilter === null
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted hover:bg-muted/80'
            }`}
          >
            All
          </button>
          <button
            onClick={() => setTypeFilter('login')}
            className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
              typeFilter === 'login'
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted hover:bg-muted/80'
            }`}
          >
            <Globe className="h-3 w-3" /> Logins
          </button>
          <button
            onClick={() => setTypeFilter('note')}
            className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
              typeFilter === 'note'
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted hover:bg-muted/80'
            }`}
          >
            <StickyNote className="h-3 w-3" /> Notes
          </button>
          <button
            onClick={() => setTypeFilter('card')}
            className={`px-3 py-2 rounded-md text-sm transition-colors flex items-center gap-1 ${
              typeFilter === 'card'
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted hover:bg-muted/80'
            }`}
          >
            <CreditCard className="h-3 w-3" /> Cards
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="card">
          <div className="card-content p-4">
            <div className="flex items-center space-x-2">
              <Globe className="h-5 w-5 text-blue-500" />
              <div>
                <p className="text-sm text-muted-foreground">Logins</p>
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
              <StickyNote className="h-5 w-5 text-green-500" />
              <div>
                <p className="text-sm text-muted-foreground">Notes</p>
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
              <CreditCard className="h-5 w-5 text-purple-500" />
              <div>
                <p className="text-sm text-muted-foreground">Cards</p>
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
              <Star className="h-5 w-5 text-yellow-500" />
              <div>
                <p className="text-sm text-muted-foreground">Favorites</p>
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
                <Shield className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-semibold mb-2">Your vault is empty</h3>
                <p className="text-muted-foreground mb-4">
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
                <Search className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-semibold mb-2">No results found</h3>
                <p className="text-muted-foreground mb-4">
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
                    className={`vault-item ${selectedEntry?.id === entry.id ? 'bg-accent' : ''}`}
                    onClick={() => setSelectedEntry(entry)}
                  >
                    <div className="vault-item-info">
                      <div className="vault-item-icon">
                        <IconComponent className="h-4 w-4 text-primary" />
                      </div>

                      <div className="vault-item-content">
                        <div className="flex items-center space-x-2">
                          <span className="vault-item-title">{entry.name}</span>
                          {entry.favorite && (
                            <Star className="h-3 w-3 text-yellow-500 fill-current" />
                          )}
                        </div>
                        <span className="vault-item-subtitle">
                          {entry.fields.username || entry.fields.url || entry.type}
                        </span>
                      </div>
                    </div>

                    <div className="text-xs text-muted-foreground">
                      {entry.type.charAt(0).toUpperCase() + entry.type.slice(1)}
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
                    className: "h-5 w-5 text-primary"
                  })}
                  <div>
                    <h3 className="card-title text-lg">{selectedEntry.name}</h3>
                    <p className="card-description">
                      {selectedEntry.type.charAt(0).toUpperCase() + selectedEntry.type.slice(1)}
                    </p>
                  </div>
                </div>
              </div>

              <div className="card-content space-y-4">
                {/* Fields */}
                {Object.entries(selectedEntry.fields).map(([key, value]) => (
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
                              className="p-1 text-muted-foreground hover:text-foreground"
                            >
                              {showPasswords[selectedEntry.id] ? (
                                <EyeOff className="h-4 w-4" />
                              ) : (
                                <Eye className="h-4 w-4" />
                              )}
                            </button>
                            <button
                              onClick={() => copyToClipboard(value, key)}
                              className="p-1 text-muted-foreground hover:text-foreground"
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
                              className="p-1 text-muted-foreground hover:text-foreground"
                            >
                              <Copy className="h-4 w-4" />
                            </button>
                            {key.toLowerCase() === 'url' && value && (
                              <button
                                onClick={() => window.open(value.startsWith('http') ? value : `https://${value}`, '_blank')}
                                className="p-1 text-muted-foreground hover:text-foreground"
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
                <div className="flex space-x-2 pt-4 border-t">
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
                        <Star className="h-4 w-4 mr-2 fill-current text-yellow-500" />
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
                <div className="h-12 w-12 bg-muted rounded-full flex items-center justify-center mx-auto mb-4">
                  <Globe className="h-6 w-6 text-muted-foreground" />
                </div>
                <h3 className="text-lg font-semibold mb-2">Select an item</h3>
                <p className="text-muted-foreground">
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
  );
}
