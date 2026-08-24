import React, { useState, useEffect, useCallback, useRef } from 'react';
import { X, Eye, EyeOff, RefreshCw, Globe, StickyNote, CreditCard, User, Trash2, Timer, Plus, Settings2, Paperclip, AlertCircle } from 'lucide-react';
import { useCrypto } from '../contexts/CryptoContext';
import { useVault } from '../contexts/VaultContext';
import { totp, totpSecondsRemaining } from '../crypto/totp';
import { passwordStrength } from '../crypto/password-strength';
import PasswordGenerator from './PasswordGenerator';
import type { VaultAttachment } from '../contexts/VaultContext';

export type EntryType = 'login' | 'note' | 'card' | 'identity' | 'totp';

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
  custom_fields?: Array<{ label: string; value: string }>;
  attachments?: VaultAttachment[];
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
  totp: {
    icon: Timer,
    fields: ['issuer', 'secret'],
    fieldLabels: {
      issuer: 'Issuer (e.g. GitHub)',
      secret: 'Secret (base32)',
    },
  },
};

/** Live TOTP preview inside the modal. */
function TotpPreview({ secret }: { secret: string }) {
  const [code, setCode] = useState<string>('------');
  const [seconds, setSeconds] = useState(30);
  const [error, setError] = useState(false);

  const refresh = useCallback(async () => {
    if (!secret.trim()) { setCode('------'); setError(false); return; }
    try {
      const c = await totp(secret.trim());
      setCode(c);
      setSeconds(totpSecondsRemaining());
      setError(false);
    } catch {
      setCode('------');
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
  return (
    <div className="flex items-center gap-3 p-3 rounded-lg border border-cloistr-border bg-cloistr-bg-hover/30">
      {error ? (
        <span className="text-cloistr-error text-sm">Invalid secret</span>
      ) : (
        <span className={`text-2xl font-mono font-bold ${color}`} style={{ letterSpacing: '0.2em' }}>{code}</span>
      )}
      <div className={`text-sm ml-auto ${color}`}>{seconds}s</div>
    </div>
  );
}

/** Password strength bar. */
function StrengthBar({ password }: { password: string }) {
  if (!password) return null;
  const result = passwordStrength(password);
  const colors = ['bg-red-500', 'bg-orange-500', 'bg-yellow-500', 'bg-green-500', 'bg-emerald-500'];
  const textColors = ['text-red-500', 'text-orange-500', 'text-yellow-500', 'text-green-500', 'text-emerald-500'];

  return (
    <div className="space-y-1">
      <div className="flex gap-1">
        {[0, 1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className={`h-1.5 flex-1 rounded-full transition-colors ${
              i <= result.score ? colors[result.score] : 'bg-cloistr-border'
            }`}
          />
        ))}
      </div>
      <div className="flex justify-between items-start">
        <span className={`text-xs ${textColors[result.score]}`}>{result.label}</span>
        {result.suggestions[0] && (
          <span className="text-xs text-cloistr-text-muted text-right ml-2">{result.suggestions[0]}</span>
        )}
      </div>
    </div>
  );
}

/** Max per-attachment size: 512 KiB encoded, ~384 KiB binary. */
const MAX_ATTACHMENT_BYTES = 512 * 1024;
const MAX_ATTACHMENTS = 5;

export default function VaultEntryModal({
  isOpen,
  onClose,
  onSave,
  onDelete,
  entry,
  mode,
}: VaultEntryModalProps) {
  const { generatePassword } = useCrypto();
  const { vaultData } = useVault();

  const [type, setType] = useState<EntryType>('login');
  const [name, setName] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [notes, setNotes] = useState('');
  const [showPassword, setShowPassword] = useState<Record<string, boolean>>({});
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [folderId, setFolderId] = useState<string>('');
  const [customFields, setCustomFields] = useState<Array<{ label: string; value: string }>>([]);
  const [attachments, setAttachments] = useState<VaultAttachment[]>([]);
  const [showGenerator, setShowGenerator] = useState(false);
  const [generatorTargetField, setGeneratorTargetField] = useState<string>('password');
  const [activeSection, setActiveSection] = useState<'main' | 'custom' | 'attachments'>('main');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const folders = vaultData?.folders ?? [];

  useEffect(() => {
    if (entry && mode === 'edit') {
      setType(entry.type);
      setName(entry.name);
      setFields(entry.fields || {});
      setNotes(entry.notes || '');
      setFolderId(entry.folder_id ?? '');
      setCustomFields(entry.custom_fields ?? []);
      setAttachments(entry.attachments ?? []);
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
    setFolderId('');
    setCustomFields([]);
    setAttachments([]);
    setActiveSection('main');
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
      folder_id: folderId || undefined,
      custom_fields: customFields.filter((f) => f.label.trim()),
      attachments: attachments.length > 0 ? attachments : undefined,
    };
    onSave(savedEntry);
    resetForm();
    onClose();
  };

  const handleFieldChange = (fieldName: string, value: string) => {
    setFields((prev) => ({ ...prev, [fieldName]: value }));
  };

  const handleGeneratePassword = (fieldName = 'password') => {
    const newPassword = generatePassword(20, true);
    handleFieldChange(fieldName, newPassword);
  };

  const openGeneratorFor = (fieldName: string) => {
    setGeneratorTargetField(fieldName);
    setShowGenerator(true);
  };

  const togglePasswordVisibility = (fieldName: string) => {
    setShowPassword((prev) => ({ ...prev, [fieldName]: !prev[fieldName] }));
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

  const isPasswordField = (fieldName: string) =>
    ['password', 'cvv'].includes(fieldName.toLowerCase());

  // Custom fields
  const addCustomField = () => setCustomFields((prev) => [...prev, { label: '', value: '' }]);
  const removeCustomField = (i: number) => setCustomFields((prev) => prev.filter((_, idx) => idx !== i));
  const updateCustomField = (i: number, key: 'label' | 'value', val: string) =>
    setCustomFields((prev) => prev.map((f, idx) => (idx === i ? { ...f, [key]: val } : f)));

  // Attachments
  const handleAttachmentFile = (file: File) => {
    if (attachments.length >= MAX_ATTACHMENTS) {
      alert(`Maximum ${MAX_ATTACHMENTS} attachments per entry.`);
      return;
    }
    const reader = new FileReader();
    reader.onload = (e) => {
      const dataUrl = e.target?.result as string;
      // dataUrl format: "data:<mime>;base64,<data>"
      const commaIdx = dataUrl.indexOf(',');
      const meta = dataUrl.slice(5, commaIdx); // "<mime>;base64"
      const mime = meta.replace(';base64', '');
      const data = dataUrl.slice(commaIdx + 1);
      if (data.length > MAX_ATTACHMENT_BYTES) {
        alert(`File too large. Maximum size is ${MAX_ATTACHMENT_BYTES / 1024} KiB.`);
        return;
      }
      setAttachments((prev) => [...prev, { name: file.name, mime, data }]);
    };
    reader.readAsDataURL(file);
  };

  const removeAttachment = (i: number) => setAttachments((prev) => prev.filter((_, idx) => idx !== i));

  const downloadAttachment = (att: VaultAttachment) => {
    const a = document.createElement('a');
    a.href = `data:${att.mime};base64,${att.data}`;
    a.download = att.name;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  if (!isOpen) return null;

  const config = typeConfig[type];
  const IconComponent = config.icon;

  const customFieldCount = customFields.filter((f) => f.label.trim()).length;
  const attachmentCount = attachments.length;

  return (
    <>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
        onClick={(e) => { if (e.target === e.currentTarget) { resetForm(); onClose(); } }}
      >
        <div
          className="bg-cloistr-bg-elevated rounded-lg shadow-xl w-full max-w-md flex flex-col"
          style={{ maxHeight: '90dvh' }}
        >
          {/* Header */}
          <div className="flex items-center justify-between p-4 border-b border-cloistr-border flex-shrink-0">
            <div className="flex items-center space-x-2">
              <IconComponent className="h-5 w-5 text-cloistr-primary" />
              <h2 className="text-lg font-semibold">
                {mode === 'add' ? 'Add New Item' : 'Edit Item'}
              </h2>
            </div>
            <button
              onClick={() => { resetForm(); onClose(); }}
              className="p-2 text-cloistr-text-muted hover:text-cloistr-text rounded"
              style={{ minHeight: '44px', minWidth: '44px' }}
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {/* Section tabs */}
          <div className="flex border-b border-cloistr-border flex-shrink-0 overflow-x-auto">
            {[
              ['main', 'Details'],
              ['custom', `Custom${customFieldCount > 0 ? ` (${customFieldCount})` : ''}`],
              ['attachments', `Files${attachmentCount > 0 ? ` (${attachmentCount})` : ''}`],
            ].map(([tab, label]) => (
              <button
                key={tab}
                onClick={() => setActiveSection(tab as typeof activeSection)}
                style={{ minHeight: '44px' }}
                className={`px-4 py-2 text-sm font-medium whitespace-nowrap border-b-2 transition-colors flex-shrink-0 ${
                  activeSection === tab
                    ? 'border-cloistr-primary text-cloistr-primary'
                    : 'border-transparent text-cloistr-text-muted hover:text-cloistr-text'
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          <form onSubmit={handleSubmit} className="flex-1 flex flex-col overflow-hidden">
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
              {activeSection === 'main' && (
                <>
                  {/* Entry type (add only) */}
                  {mode === 'add' && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Item Type</label>
                      <div className="grid grid-cols-5 gap-2">
                        {(Object.keys(typeConfig) as EntryType[]).map((t) => {
                          const TypeIcon = typeConfig[t].icon;
                          return (
                            <button
                              key={t}
                              type="button"
                              onClick={() => { setType(t); setFields({}); }}
                              style={{ minHeight: '44px' }}
                              className={`flex flex-col items-center p-2 rounded-lg border transition-colors ${
                                type === t
                                  ? 'border-primary bg-cloistr-primary/10 text-cloistr-primary'
                                  : 'border-cloistr-border hover:border-cloistr-primary/50'
                              }`}
                            >
                              <TypeIcon className="h-4 w-4 mb-1" />
                              <span className="text-xs capitalize">{t === 'totp' ? 'TOTP' : t}</span>
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
                      placeholder={
                        type === 'login' ? 'e.g., GitHub' :
                        type === 'card' ? 'e.g., Personal Visa' :
                        type === 'note' ? 'e.g., API Keys' :
                        type === 'totp' ? 'e.g., GitHub 2FA' :
                        'e.g., Home Address'
                      }
                      className="input w-full"
                      required
                    />
                  </div>

                  {/* Folder picker */}
                  {folders.length > 0 && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Folder</label>
                      <select
                        value={folderId}
                        onChange={(e) => setFolderId(e.target.value)}
                        className="input w-full"
                      >
                        <option value="">No folder</option>
                        {folders.map((f) => (
                          <option key={f.id} value={f.id}>{f.name}</option>
                        ))}
                      </select>
                    </div>
                  )}

                  {/* Type-specific fields */}
                  {config.fields.map((fieldName) => (
                    <div key={fieldName} className="space-y-2">
                      <label className="text-sm font-medium">{config.fieldLabels[fieldName]}</label>
                      <div className="relative">
                        <input
                          type={isPasswordField(fieldName) && !showPassword[fieldName] ? 'password' : 'text'}
                          value={fields[fieldName] || ''}
                          onChange={(e) => handleFieldChange(fieldName, e.target.value)}
                          placeholder={config.fieldLabels[fieldName]}
                          className={`input w-full ${
                            isPasswordField(fieldName) || fieldName === 'password' ? 'pr-20' : ''
                          }`}
                          autoComplete={fieldName === 'secret' ? 'off' : undefined}
                        />
                        {isPasswordField(fieldName) && (
                          <div className="absolute inset-y-0 right-0 flex items-center space-x-1 pr-2">
                            <button
                              type="button"
                              onClick={() => togglePasswordVisibility(fieldName)}
                              className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                              style={{ minHeight: '32px', minWidth: '32px' }}
                              aria-label={showPassword[fieldName] ? 'Hide' : 'Show'}
                            >
                              {showPassword[fieldName] ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </button>
                            {fieldName === 'password' && (
                              <button
                                type="button"
                                onClick={() => openGeneratorFor(fieldName)}
                                className="p-1 text-cloistr-text-muted hover:text-cloistr-text"
                                style={{ minHeight: '32px', minWidth: '32px' }}
                                title="Open password generator"
                              >
                                <Settings2 className="h-4 w-4" />
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                      {/* Strength bar for password fields */}
                      {fieldName === 'password' && fields[fieldName] && (
                        <StrengthBar password={fields[fieldName]} />
                      )}
                    </div>
                  ))}

                  {/* Live TOTP preview */}
                  {type === 'totp' && fields.secret && (
                    <TotpPreview secret={fields.secret} />
                  )}

                  {/* Notes (not for TOTP) */}
                  {type !== 'totp' && (
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Notes</label>
                      <textarea
                        value={notes}
                        onChange={(e) => setNotes(e.target.value)}
                        placeholder={type === 'note' ? 'Enter your secure note...' : 'Optional notes...'}
                        className="input w-full resize-none"
                        rows={type === 'note' ? 6 : 3}
                      />
                    </div>
                  )}
                </>
              )}

              {activeSection === 'custom' && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium">Custom Fields</p>
                      <p className="text-xs text-cloistr-text-muted">Add arbitrary key-value pairs to this entry.</p>
                    </div>
                    <button
                      type="button"
                      onClick={addCustomField}
                      className="btn-outline text-sm"
                      style={{ minHeight: '44px' }}
                    >
                      <Plus className="h-4 w-4 mr-1" /> Add field
                    </button>
                  </div>

                  {customFields.length === 0 ? (
                    <div className="text-center py-8 text-cloistr-text-muted">
                      <Settings2 className="h-8 w-8 mx-auto mb-2 opacity-40" />
                      <p className="text-sm">No custom fields yet.</p>
                    </div>
                  ) : (
                    customFields.map((cf, i) => (
                      <div key={i} className="flex gap-2 items-start">
                        <div className="flex-1 space-y-1">
                          <input
                            type="text"
                            value={cf.label}
                            onChange={(e) => updateCustomField(i, 'label', e.target.value)}
                            placeholder="Field name"
                            className="input w-full text-sm"
                          />
                          <input
                            type="text"
                            value={cf.value}
                            onChange={(e) => updateCustomField(i, 'value', e.target.value)}
                            placeholder="Value"
                            className="input w-full text-sm"
                          />
                        </div>
                        <button
                          type="button"
                          onClick={() => removeCustomField(i)}
                          className="p-2 text-cloistr-text-muted hover:text-cloistr-error mt-1 flex-shrink-0"
                          style={{ minHeight: '44px', minWidth: '44px' }}
                          aria-label="Remove field"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}

              {activeSection === 'attachments' && (
                <div className="space-y-3">
                  <div>
                    <p className="text-sm font-medium">Attachments</p>
                    <p className="text-xs text-cloistr-text-muted">
                      Files are stored encrypted inside the vault blob. Max {MAX_ATTACHMENTS} files, 512 KiB each.
                    </p>
                  </div>

                  {attachments.length < MAX_ATTACHMENTS && (
                    <button
                      type="button"
                      onClick={() => fileInputRef.current?.click()}
                      className="w-full border-2 border-dashed border-cloistr-border rounded-lg p-4 text-center hover:border-cloistr-primary transition-colors"
                      style={{ minHeight: '44px' }}
                    >
                      <Paperclip className="h-6 w-6 mx-auto mb-1 text-cloistr-text-muted" />
                      <span className="text-sm text-cloistr-text-muted">Click to attach a file</span>
                      <input
                        ref={fileInputRef}
                        type="file"
                        className="hidden"
                        onChange={(e) => {
                          const file = e.target.files?.[0];
                          if (file) handleAttachmentFile(file);
                          e.target.value = '';
                        }}
                      />
                    </button>
                  )}

                  {attachments.length === 0 ? (
                    <div className="text-center py-6 text-cloistr-text-muted">
                      <p className="text-sm">No attachments yet.</p>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {attachments.map((att, i) => (
                        <div key={i} className="flex items-center gap-3 p-3 rounded-lg border border-cloistr-border">
                          <Paperclip className="h-4 w-4 text-cloistr-text-muted flex-shrink-0" />
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{att.name}</p>
                            <p className="text-xs text-cloistr-text-muted">
                              {att.mime} · {Math.round(att.data.length * 0.75 / 1024)} KiB
                            </p>
                          </div>
                          <button
                            type="button"
                            onClick={() => downloadAttachment(att)}
                            className="text-xs text-cloistr-primary underline flex-shrink-0"
                            style={{ minHeight: '44px' }}
                          >
                            Save
                          </button>
                          <button
                            type="button"
                            onClick={() => removeAttachment(i)}
                            className="p-1 text-cloistr-text-muted hover:text-cloistr-error flex-shrink-0"
                            style={{ minHeight: '44px', minWidth: '44px' }}
                            aria-label="Remove"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between p-4 border-t border-cloistr-border flex-shrink-0">
              {mode === 'edit' && onDelete ? (
                <button
                  type="button"
                  onClick={handleDelete}
                  style={{ minHeight: '44px' }}
                  className={`btn-outline ${confirmDelete ? 'border-cloistr-error text-cloistr-error hover:bg-cloistr-error/10' : ''}`}
                >
                  <Trash2 className="h-4 w-4 mr-2" />
                  {confirmDelete ? 'Confirm Delete' : 'Delete'}
                </button>
              ) : (
                <div />
              )}

              <div className="flex space-x-2">
                <button type="button" onClick={() => { resetForm(); onClose(); }} className="btn-outline" style={{ minHeight: '44px' }}>
                  Cancel
                </button>
                <button type="submit" className="btn-primary" style={{ minHeight: '44px' }}>
                  {mode === 'add' ? 'Add Item' : 'Save Changes'}
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>

      {/* Password generator modal */}
      <PasswordGenerator
        isOpen={showGenerator}
        onClose={() => setShowGenerator(false)}
        onUse={(pw) => handleFieldChange(generatorTargetField, pw)}
      />
    </>
  );
}
