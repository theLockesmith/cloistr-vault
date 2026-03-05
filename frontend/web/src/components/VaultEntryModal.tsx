import React, { useState, useEffect } from 'react';
import { X, Eye, EyeOff, RefreshCw, Globe, StickyNote, CreditCard, User, Trash2 } from 'lucide-react';
import { useCrypto } from '../contexts/CryptoContext';

export type EntryType = 'login' | 'note' | 'card' | 'identity';

export interface VaultEntry {
  id: string;
  type: EntryType;
  name: string;
  fields: Record<string, string>;
  notes: string;
  created_at: string;
  updated_at: string;
  favorite: boolean;
  folder_id?: string;
}

interface VaultEntryModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (entry: VaultEntry) => void;
  onDelete?: (id: string) => void;
  entry?: VaultEntry | null;
  mode: 'add' | 'edit';
}

const typeConfig: Record<EntryType, { icon: typeof Globe; fields: string[]; fieldLabels: Record<string, string> }> = {
  login: {
    icon: Globe,
    fields: ['url', 'username', 'password'],
    fieldLabels: {
      url: 'Website URL',
      username: 'Username',
      password: 'Password',
    },
  },
  note: {
    icon: StickyNote,
    fields: [],
    fieldLabels: {},
  },
  card: {
    icon: CreditCard,
    fields: ['cardNumber', 'cardholderName', 'expirationDate', 'cvv'],
    fieldLabels: {
      cardNumber: 'Card Number',
      cardholderName: 'Cardholder Name',
      expirationDate: 'Expiration (MM/YY)',
      cvv: 'CVV',
    },
  },
  identity: {
    icon: User,
    fields: ['firstName', 'lastName', 'email', 'phone', 'address'],
    fieldLabels: {
      firstName: 'First Name',
      lastName: 'Last Name',
      email: 'Email',
      phone: 'Phone',
      address: 'Address',
    },
  },
};

export default function VaultEntryModal({
  isOpen,
  onClose,
  onSave,
  onDelete,
  entry,
  mode,
}: VaultEntryModalProps) {
  const { generatePassword } = useCrypto();
  const [type, setType] = useState<EntryType>('login');
  const [name, setName] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [notes, setNotes] = useState('');
  const [showPassword, setShowPassword] = useState<Record<string, boolean>>({});
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    if (entry && mode === 'edit') {
      setType(entry.type);
      setName(entry.name);
      setFields(entry.fields || {});
      setNotes(entry.notes || '');
    } else {
      resetForm();
    }
  }, [entry, mode, isOpen]);

  const resetForm = () => {
    setType('login');
    setName('');
    setFields({});
    setNotes('');
    setShowPassword({});
    setConfirmDelete(false);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const now = new Date().toISOString();
    const savedEntry: VaultEntry = {
      id: entry?.id || crypto.randomUUID(),
      type,
      name,
      fields,
      notes,
      created_at: entry?.created_at || now,
      updated_at: now,
      favorite: entry?.favorite || false,
      folder_id: entry?.folder_id,
    };

    onSave(savedEntry);
    resetForm();
    onClose();
  };

  const handleFieldChange = (fieldName: string, value: string) => {
    setFields((prev) => ({
      ...prev,
      [fieldName]: value,
    }));
  };

  const handleGeneratePassword = () => {
    const newPassword = generatePassword(20, true);
    handleFieldChange('password', newPassword);
  };

  const togglePasswordVisibility = (fieldName: string) => {
    setShowPassword((prev) => ({
      ...prev,
      [fieldName]: !prev[fieldName],
    }));
  };

  const handleDelete = () => {
    if (confirmDelete && entry && onDelete) {
      onDelete(entry.id);
      resetForm();
      onClose();
    } else {
      setConfirmDelete(true);
    }
  };

  const isPasswordField = (fieldName: string) => {
    return ['password', 'cvv'].includes(fieldName.toLowerCase());
  };

  if (!isOpen) return null;

  const config = typeConfig[type];
  const IconComponent = config.icon;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div className="bg-card rounded-lg shadow-xl w-full max-w-md max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b">
          <div className="flex items-center space-x-2">
            <IconComponent className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold">
              {mode === 'add' ? 'Add New Item' : 'Edit Item'}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-1 text-muted-foreground hover:text-foreground rounded"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-4 space-y-4">
          {/* Entry Type Selection (only for new entries) */}
          {mode === 'add' && (
            <div className="space-y-2">
              <label className="text-sm font-medium">Item Type</label>
              <div className="grid grid-cols-4 gap-2">
                {(Object.keys(typeConfig) as EntryType[]).map((t) => {
                  const TypeIcon = typeConfig[t].icon;
                  return (
                    <button
                      key={t}
                      type="button"
                      onClick={() => {
                        setType(t);
                        setFields({});
                      }}
                      className={`flex flex-col items-center p-3 rounded-lg border transition-colors ${
                        type === t
                          ? 'border-primary bg-primary/10 text-primary'
                          : 'border-border hover:border-primary/50'
                      }`}
                    >
                      <TypeIcon className="h-5 w-5 mb-1" />
                      <span className="text-xs capitalize">{t}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Name */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Name *</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={`e.g., ${type === 'login' ? 'GitHub' : type === 'card' ? 'Personal Visa' : type === 'note' ? 'API Keys' : 'Home Address'}`}
              className="input w-full"
              required
            />
          </div>

          {/* Type-specific Fields */}
          {config.fields.map((fieldName) => (
            <div key={fieldName} className="space-y-2">
              <label className="text-sm font-medium">
                {config.fieldLabels[fieldName]}
              </label>
              <div className="relative">
                <input
                  type={isPasswordField(fieldName) && !showPassword[fieldName] ? 'password' : 'text'}
                  value={fields[fieldName] || ''}
                  onChange={(e) => handleFieldChange(fieldName, e.target.value)}
                  placeholder={config.fieldLabels[fieldName]}
                  className={`input w-full ${isPasswordField(fieldName) ? 'pr-20' : ''}`}
                />
                {isPasswordField(fieldName) && (
                  <div className="absolute inset-y-0 right-0 flex items-center space-x-1 pr-2">
                    <button
                      type="button"
                      onClick={() => togglePasswordVisibility(fieldName)}
                      className="p-1 text-muted-foreground hover:text-foreground"
                    >
                      {showPassword[fieldName] ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </button>
                    {fieldName === 'password' && (
                      <button
                        type="button"
                        onClick={handleGeneratePassword}
                        className="p-1 text-muted-foreground hover:text-foreground"
                        title="Generate password"
                      >
                        <RefreshCw className="h-4 w-4" />
                      </button>
                    )}
                  </div>
                )}
              </div>
            </div>
          ))}

          {/* Notes */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Notes</label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder={type === 'note' ? 'Enter your secure note...' : 'Optional notes...'}
              className="input w-full h-24 resize-none"
              rows={type === 'note' ? 6 : 3}
            />
          </div>

          {/* Actions */}
          <div className="flex items-center justify-between pt-4 border-t">
            {mode === 'edit' && onDelete ? (
              <button
                type="button"
                onClick={handleDelete}
                className={`btn-outline ${confirmDelete ? 'border-red-500 text-red-500 hover:bg-red-500/10' : ''}`}
              >
                <Trash2 className="h-4 w-4 mr-2" />
                {confirmDelete ? 'Confirm Delete' : 'Delete'}
              </button>
            ) : (
              <div />
            )}

            <div className="flex space-x-2">
              <button type="button" onClick={onClose} className="btn-outline">
                Cancel
              </button>
              <button type="submit" className="btn-primary">
                {mode === 'add' ? 'Add Item' : 'Save Changes'}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
