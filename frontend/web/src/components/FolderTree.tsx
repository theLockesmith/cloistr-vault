import React, { useState, useEffect, useCallback } from 'react';
import { Folder, FolderPlus, ChevronRight, ChevronDown, MoreVertical, Edit2, Trash2, FolderOpen, Check, X } from 'lucide-react';
import { VaultFolder, folderApi, CreateFolderRequest } from '../services/folderApi';
import toast from 'react-hot-toast';

interface FolderTreeProps {
  selectedFolderId: string | null;
  onSelectFolder: (folderId: string | null) => void;
  onFoldersChange?: (folders: VaultFolder[]) => void;
}

interface FolderNodeProps {
  folder: VaultFolder;
  level: number;
  selectedFolderId: string | null;
  expandedFolders: Set<string>;
  onSelect: (folderId: string | null) => void;
  onToggleExpand: (folderId: string) => void;
  onRename: (folder: VaultFolder) => void;
  onDelete: (folder: VaultFolder) => void;
  onAddSubfolder: (parentId: string) => void;
}

function FolderNode({
  folder,
  level,
  selectedFolderId,
  expandedFolders,
  onSelect,
  onToggleExpand,
  onRename,
  onDelete,
  onAddSubfolder,
}: FolderNodeProps) {
  const [showMenu, setShowMenu] = useState(false);
  const hasChildren = folder.children && folder.children.length > 0;
  const isExpanded = expandedFolders.has(folder.id);
  const isSelected = selectedFolderId === folder.id;

  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onSelect(folder.id);
  };

  const handleToggleExpand = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (hasChildren) {
      onToggleExpand(folder.id);
    }
  };

  const handleMenuClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    setShowMenu(!showMenu);
  };

  return (
    <div>
      <div
        className={`flex items-center px-2 py-1.5 rounded-md cursor-pointer group transition-colors ${
          isSelected
            ? 'bg-cloistr-primary/10 text-cloistr-primary'
            : 'hover:bg-cloistr-bg-hover'
        }`}
        style={{ paddingLeft: `${level * 12 + 8}px` }}
        onClick={handleClick}
      >
        {/* Expand/collapse icon */}
        <button
          className={`p-0.5 mr-1 ${hasChildren ? 'visible' : 'invisible'}`}
          onClick={handleToggleExpand}
        >
          {isExpanded ? (
            <ChevronDown className="h-3 w-3 text-cloistr-text-muted" />
          ) : (
            <ChevronRight className="h-3 w-3 text-cloistr-text-muted" />
          )}
        </button>

        {/* Folder icon */}
        <span className="mr-2" title={folder.icon}>
          {isExpanded ? (
            <FolderOpen className="h-4 w-4" style={{ color: folder.color }} />
          ) : (
            <Folder className="h-4 w-4" style={{ color: folder.color }} />
          )}
        </span>

        {/* Folder name */}
        <span className="flex-1 text-sm truncate">{folder.name}</span>

        {/* Entry count */}
        {folder.entry_count !== undefined && folder.entry_count > 0 && (
          <span className="text-xs text-cloistr-text-muted mr-2">
            {folder.entry_count}
          </span>
        )}

        {/* Context menu button */}
        <div className="relative">
          <button
            className={`p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-cloistr-bg-hover-foreground/20 ${
              showMenu ? 'opacity-100' : ''
            }`}
            onClick={handleMenuClick}
          >
            <MoreVertical className="h-3 w-3" />
          </button>

          {showMenu && (
            <>
              <div
                className="fixed inset-0 z-10"
                onClick={() => setShowMenu(false)}
              />
              <div className="absolute right-0 top-full mt-1 py-1 bg-cloistr-bg-elevated border rounded-md shadow-lg z-20 min-w-[140px]">
                <button
                  className="w-full px-3 py-1.5 text-sm text-left hover:bg-cloistr-bg-hover flex items-center gap-2"
                  onClick={(e) => {
                    e.stopPropagation();
                    setShowMenu(false);
                    onAddSubfolder(folder.id);
                  }}
                >
                  <FolderPlus className="h-3 w-3" />
                  Add subfolder
                </button>
                <button
                  className="w-full px-3 py-1.5 text-sm text-left hover:bg-cloistr-bg-hover flex items-center gap-2"
                  onClick={(e) => {
                    e.stopPropagation();
                    setShowMenu(false);
                    onRename(folder);
                  }}
                >
                  <Edit2 className="h-3 w-3" />
                  Rename
                </button>
                <button
                  className="w-full px-3 py-1.5 text-sm text-left hover:bg-cloistr-bg-hover flex items-center gap-2 text-cloistr-error"
                  onClick={(e) => {
                    e.stopPropagation();
                    setShowMenu(false);
                    onDelete(folder);
                  }}
                >
                  <Trash2 className="h-3 w-3" />
                  Delete
                </button>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Children */}
      {hasChildren && isExpanded && (
        <div>
          {folder.children!.map((child) => (
            <FolderNode
              key={child.id}
              folder={child}
              level={level + 1}
              selectedFolderId={selectedFolderId}
              expandedFolders={expandedFolders}
              onSelect={onSelect}
              onToggleExpand={onToggleExpand}
              onRename={onRename}
              onDelete={onDelete}
              onAddSubfolder={onAddSubfolder}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export default function FolderTree({
  selectedFolderId,
  onSelectFolder,
  onFoldersChange,
}: FolderTreeProps) {
  const [folders, setFolders] = useState<VaultFolder[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set());
  const [isCreating, setIsCreating] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [newFolderParentId, setNewFolderParentId] = useState<string | null>(null);
  const [editingFolder, setEditingFolder] = useState<VaultFolder | null>(null);
  const [editingName, setEditingName] = useState('');

  const loadFolders = useCallback(async () => {
    try {
      const data = await folderApi.getFolders(true); // Get tree structure
      setFolders(data);
      onFoldersChange?.(data);
    } catch (error) {
      console.error('Failed to load folders:', error);
      toast.error('Failed to load folders');
    } finally {
      setLoading(false);
    }
  }, [onFoldersChange]);

  useEffect(() => {
    loadFolders();
  }, [loadFolders]);

  const handleToggleExpand = (folderId: string) => {
    setExpandedFolders((prev) => {
      const next = new Set(prev);
      if (next.has(folderId)) {
        next.delete(folderId);
      } else {
        next.add(folderId);
      }
      return next;
    });
  };

  const handleStartCreate = (parentId: string | null = null) => {
    setNewFolderParentId(parentId);
    setNewFolderName('');
    setIsCreating(true);
    if (parentId) {
      setExpandedFolders((prev) => new Set(prev).add(parentId));
    }
  };

  const handleCancelCreate = () => {
    setIsCreating(false);
    setNewFolderName('');
    setNewFolderParentId(null);
  };

  const handleSubmitCreate = async () => {
    if (!newFolderName.trim()) {
      toast.error('Folder name is required');
      return;
    }

    try {
      const request: CreateFolderRequest = {
        name: newFolderName.trim(),
        parent_id: newFolderParentId || undefined,
      };
      await folderApi.createFolder(request);
      toast.success('Folder created');
      handleCancelCreate();
      loadFolders();
    } catch (error) {
      console.error('Failed to create folder:', error);
      toast.error('Failed to create folder');
    }
  };

  const handleStartRename = (folder: VaultFolder) => {
    setEditingFolder(folder);
    setEditingName(folder.name);
  };

  const handleCancelRename = () => {
    setEditingFolder(null);
    setEditingName('');
  };

  const handleSubmitRename = async () => {
    if (!editingFolder || !editingName.trim()) return;

    try {
      await folderApi.updateFolder(editingFolder.id, { name: editingName.trim() });
      toast.success('Folder renamed');
      handleCancelRename();
      loadFolders();
    } catch (error) {
      console.error('Failed to rename folder:', error);
      toast.error('Failed to rename folder');
    }
  };

  const handleDelete = async (folder: VaultFolder) => {
    const hasChildren = folder.children && folder.children.length > 0;
    const message = hasChildren
      ? `Delete "${folder.name}" and all its subfolders?`
      : `Delete folder "${folder.name}"?`;

    if (!window.confirm(message)) return;

    try {
      await folderApi.deleteFolder(folder.id, hasChildren);
      toast.success('Folder deleted');
      if (selectedFolderId === folder.id) {
        onSelectFolder(null);
      }
      loadFolders();
    } catch (error: any) {
      console.error('Failed to delete folder:', error);
      const message = error.response?.data?.error || 'Failed to delete folder';
      toast.error(message);
    }
  };

  if (loading) {
    return (
      <div className="p-4">
        <div className="animate-pulse space-y-2">
          <div className="h-6 bg-cloistr-bg-hover rounded w-3/4"></div>
          <div className="h-6 bg-cloistr-bg-hover rounded w-1/2"></div>
          <div className="h-6 bg-cloistr-bg-hover rounded w-2/3"></div>
        </div>
      </div>
    );
  }

  return (
    <div className="py-2">
      {/* All Items option */}
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

      {/* Folder tree */}
      {folders.map((folder) => (
        <FolderNode
          key={folder.id}
          folder={folder}
          level={0}
          selectedFolderId={selectedFolderId}
          expandedFolders={expandedFolders}
          onSelect={onSelectFolder}
          onToggleExpand={handleToggleExpand}
          onRename={handleStartRename}
          onDelete={handleDelete}
          onAddSubfolder={handleStartCreate}
        />
      ))}

      {/* New folder input */}
      {isCreating && (
        <div
          className="flex items-center gap-1 px-3 py-1"
          style={{ paddingLeft: newFolderParentId ? '32px' : '12px' }}
        >
          <Folder className="h-4 w-4 text-cloistr-text-muted" />
          <input
            type="text"
            className="flex-1 px-2 py-1 text-sm bg-transparent border rounded focus:outline-none focus:ring-1 focus:ring-primary"
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
          >
            <Check className="h-3 w-3 text-cloistr-success" />
          </button>
          <button
            className="p-1 hover:bg-cloistr-bg-hover rounded"
            onClick={handleCancelCreate}
          >
            <X className="h-3 w-3 text-cloistr-error" />
          </button>
        </div>
      )}

      {/* Rename input (modal/inline) */}
      {editingFolder && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-cloistr-bg-elevated p-4 rounded-lg shadow-lg w-80">
            <h3 className="text-sm font-medium mb-3">Rename Folder</h3>
            <input
              type="text"
              className="w-full px-3 py-2 text-sm border rounded focus:outline-none focus:ring-1 focus:ring-primary"
              value={editingName}
              onChange={(e) => setEditingName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleSubmitRename();
                if (e.key === 'Escape') handleCancelRename();
              }}
              autoFocus
            />
            <div className="flex justify-end gap-2 mt-4">
              <button
                className="px-3 py-1.5 text-sm rounded hover:bg-cloistr-bg-hover"
                onClick={handleCancelRename}
              >
                Cancel
              </button>
              <button
                className="px-3 py-1.5 text-sm bg-cloistr-primary text-cloistr-primary-foreground rounded hover:bg-cloistr-primary/90"
                onClick={handleSubmitRename}
              >
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Add folder button */}
      <button
        className="flex items-center gap-2 px-3 py-1.5 mt-2 text-sm text-cloistr-text-muted hover:text-cloistr-text hover:bg-cloistr-bg-hover rounded-md w-full transition-colors"
        onClick={() => handleStartCreate(null)}
      >
        <FolderPlus className="h-4 w-4" />
        <span>New Folder</span>
      </button>
    </div>
  );
}
