/**
 * FolderTree — sidebar folder list.
 *
 * Folders are stored inside the encrypted vault blob (VaultContext.vaultData.folders),
 * not via the server's REST /folders API. This keeps folder IDs in the same
 * namespace as the entry.folder_id field, so filtering works correctly.
 *
 * Previous version called folderApi (server REST) and produced IDs that never
 * matched any entry.folder_id — that is what caused every folder selection to
 * show "No results found".
 */
import React, { useState } from 'react';
import { Folder, FolderPlus, Trash2, Check, X } from 'lucide-react';
import { useVault } from '../contexts/VaultContext';
import type { VaultFolder } from '../contexts/VaultContext';
import toast from 'react-hot-toast';

interface FolderTreeProps {
  selectedFolderId: string | null;
  onSelectFolder: (folderId: string | null) => void;
}

export default function FolderTree({ selectedFolderId, onSelectFolder }: FolderTreeProps) {
  const { vaultData, isLocked, addFolder, deleteFolder } = useVault();
  const [isCreating, setIsCreating] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');

  const folders: VaultFolder[] = vaultData?.folders ?? [];

  const handleStartCreate = () => {
    setNewFolderName('');
    setIsCreating(true);
  };

  const handleCancelCreate = () => {
    setIsCreating(false);
    setNewFolderName('');
  };

  const handleSubmitCreate = async () => {
    const name = newFolderName.trim();
    if (!name) {
      toast.error('Folder name is required');
      return;
    }
    try {
      await addFolder(name);
      handleCancelCreate();
    } catch (err) {
      console.error('Failed to create folder:', err);
      toast.error('Failed to create folder');
    }
  };

  const handleDelete = async (folder: VaultFolder) => {
    if (!window.confirm(`Delete folder "${folder.name}"? Entries inside will move to All Items.`)) {
      return;
    }
    try {
      await deleteFolder(folder.id);
      if (selectedFolderId === folder.id) {
        onSelectFolder(null);
      }
    } catch (err) {
      console.error('Failed to delete folder:', err);
      toast.error('Failed to delete folder');
    }
  };

  if (isLocked) {
    return null;
  }

  return (
    <div className="py-2">
      {/* All Items */}
      <div
        className={`flex items-center px-3 py-1.5 rounded-md cursor-pointer transition-colors ${
          selectedFolderId === null
            ? 'bg-cloistr-primary/10 text-cloistr-primary'
            : 'hover:bg-cloistr-bg-hover'
        }`}
        onClick={() => onSelectFolder(null)}
      >
        <Folder className="h-4 w-4 mr-2" />
        <span className="text-sm font-medium">All Items</span>
      </div>

      {/* Folder list */}
      {folders.map((folder) => (
        <div
          key={folder.id}
          className={`flex items-center px-3 py-1.5 rounded-md cursor-pointer group transition-colors ${
            selectedFolderId === folder.id
              ? 'bg-cloistr-primary/10 text-cloistr-primary'
              : 'hover:bg-cloistr-bg-hover'
          }`}
          onClick={() => onSelectFolder(folder.id)}
        >
          <Folder className="h-4 w-4 mr-2 flex-shrink-0" />
          <span className="flex-1 text-sm truncate">{folder.name}</span>
          <button
            className="p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-cloistr-bg-hover/50"
            onClick={(e) => {
              e.stopPropagation();
              handleDelete(folder);
            }}
            title="Delete folder"
          >
            <Trash2 className="h-3 w-3 text-cloistr-error" />
          </button>
        </div>
      ))}

      {/* New folder inline input */}
      {isCreating && (
        <div className="flex items-center gap-1 px-3 py-1">
          <Folder className="h-4 w-4 text-cloistr-text-muted flex-shrink-0" />
          <input
            type="text"
            className="flex-1 px-2 py-1 text-sm bg-transparent border rounded focus:outline-none focus:ring-1 focus:ring-cloistr-primary"
            placeholder="Folder name"
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleSubmitCreate();
              if (e.key === 'Escape') handleCancelCreate();
            }}
            autoFocus
          />
          <button
            className="p-1 hover:bg-cloistr-bg-hover rounded"
            onClick={handleSubmitCreate}
            title="Create"
          >
            <Check className="h-3 w-3 text-cloistr-success" />
          </button>
          <button
            className="p-1 hover:bg-cloistr-bg-hover rounded"
            onClick={handleCancelCreate}
            title="Cancel"
          >
            <X className="h-3 w-3 text-cloistr-error" />
          </button>
        </div>
      )}

      {/* Add folder button */}
      <button
        className="flex items-center gap-2 px-3 py-1.5 mt-1 text-sm text-cloistr-text-muted hover:text-cloistr-text hover:bg-cloistr-bg-hover rounded-md w-full transition-colors"
        onClick={handleStartCreate}
      >
        <FolderPlus className="h-4 w-4" />
        <span>New Folder</span>
      </button>
    </div>
  );
}
