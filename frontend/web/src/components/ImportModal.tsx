/**
 * ImportModal — imports entries from KeePass XML, KeePass CSV, 1Password CSV,
 * or generic CSV.
 *
 * All parsing is done client-side. The decrypted entries are passed to the
 * vault's addEntry function which encrypts them locally before persisting.
 * No unencrypted data ever leaves the browser.
 */
import React, { useState, useRef } from 'react';
import { X, Upload, FileText, AlertCircle, Check } from 'lucide-react';
import { useVault } from '../contexts/VaultContext';
import toast from 'react-hot-toast';

type Format = 'keepass-xml' | 'keepass-csv' | '1password-csv' | 'generic-csv';

interface ParsedEntry {
  name: string;
  username?: string;
  password?: string;
  url?: string;
  notes?: string;
  totp?: string;
}

// ─── KeePass XML parser ───────────────────────────────────────────────────────

function parseKeePassXml(xml: string): ParsedEntry[] {
  const parser = new DOMParser();
  const doc = parser.parseFromString(xml, 'application/xml');
  const parseError = doc.querySelector('parsererror');
  if (parseError) {
    throw new Error('Invalid KeePass XML: ' + parseError.textContent?.slice(0, 120));
  }

  const entries: ParsedEntry[] = [];

  doc.querySelectorAll('Entry').forEach((entryEl) => {
    const fields: Record<string, string> = {};
    entryEl.querySelectorAll(':scope > String').forEach((strEl) => {
      const key = strEl.querySelector('Key')?.textContent ?? '';
      const value = strEl.querySelector('Value')?.textContent ?? '';
      fields[key] = value;
    });

    // KeePass standard keys: Title, UserName, Password, URL, Notes
    const name = fields['Title'] || fields['title'] || 'Untitled';
    if (!name || name === 'Untitled' && !fields['Password'] && !fields['UserName']) return;

    // TOTP: KeePass stores it in a custom field, often "otp" or "TOTP"
    const totp = fields['otp'] || fields['TOTP'] || fields['totp'] || undefined;

    entries.push({
      name,
      username: fields['UserName'] || undefined,
      password: fields['Password'] || undefined,
      url: fields['URL'] || undefined,
      notes: fields['Notes'] || undefined,
      totp,
    });
  });

  return entries;
}

// ─── CSV parser (RFC 4180) ────────────────────────────────────────────────────

function parseCsvRows(text: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = '';
  let inQuotes = false;
  let i = 0;

  while (i < text.length) {
    const ch = text[i];

    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i += 2;
        } else {
          inQuotes = false;
          i++;
        }
      } else {
        field += ch;
        i++;
      }
    } else {
      if (ch === '"') {
        inQuotes = true;
        i++;
      } else if (ch === ',') {
        row.push(field);
        field = '';
        i++;
      } else if (ch === '\r' && text[i + 1] === '\n') {
        row.push(field);
        field = '';
        rows.push(row);
        row = [];
        i += 2;
      } else if (ch === '\n') {
        row.push(field);
        field = '';
        rows.push(row);
        row = [];
        i++;
      } else {
        field += ch;
        i++;
      }
    }
  }

  if (field || row.length > 0) {
    row.push(field);
    rows.push(row);
  }

  return rows.filter((r) => r.some((c) => c.trim()));
}

/**
 * Maps a CSV header row to an index lookup, normalising keys to lowercase.
 */
function headerMap(headers: string[]): Record<string, number> {
  const map: Record<string, number> = {};
  headers.forEach((h, i) => {
    map[h.trim().toLowerCase()] = i;
  });
  return map;
}

function parseKeePassCsv(text: string): ParsedEntry[] {
  const rows = parseCsvRows(text);
  if (rows.length < 2) return [];
  const headers = headerMap(rows[0]);
  // KeePass CSV: "Account","Login Name","Password","Web Site","Comments"
  const get = (row: string[], keys: string[]): string | undefined => {
    for (const k of keys) {
      const idx = headers[k.toLowerCase()];
      if (idx !== undefined && row[idx]?.trim()) return row[idx].trim();
    }
    return undefined;
  };

  return rows.slice(1).map((row) => ({
    name: get(row, ['Account', 'Title', 'Name']) || 'Untitled',
    username: get(row, ['Login Name', 'Username', 'User Name']),
    password: get(row, ['Password', 'Pass']),
    url: get(row, ['Web Site', 'URL', 'Url']),
    notes: get(row, ['Comments', 'Notes']),
  }));
}

function parse1PasswordCsv(text: string): ParsedEntry[] {
  const rows = parseCsvRows(text);
  if (rows.length < 2) return [];
  const headers = headerMap(rows[0]);
  const get = (row: string[], keys: string[]): string | undefined => {
    for (const k of keys) {
      const idx = headers[k.toLowerCase()];
      if (idx !== undefined && row[idx]?.trim()) return row[idx].trim();
    }
    return undefined;
  };

  return rows.slice(1).map((row) => {
    const totp = get(row, ['OTPAuth', 'otp auth', 'one time password']);
    return {
      name: get(row, ['Title', 'Name']) || 'Untitled',
      username: get(row, ['Username', 'Login']),
      password: get(row, ['Password']),
      url: get(row, ['URL', 'Website']),
      notes: get(row, ['Notes', 'Comments']),
      totp,
    };
  });
}

function parseGenericCsv(text: string): ParsedEntry[] {
  const rows = parseCsvRows(text);
  if (rows.length < 2) return [];
  const headers = headerMap(rows[0]);
  const get = (row: string[], keys: string[]): string | undefined => {
    for (const k of keys) {
      const idx = headers[k.toLowerCase()];
      if (idx !== undefined && row[idx]?.trim()) return row[idx].trim();
    }
    return undefined;
  };

  return rows.slice(1).map((row) => ({
    name: get(row, ['name', 'title', 'account', 'site']) || 'Untitled',
    username: get(row, ['username', 'user', 'email', 'login name']),
    password: get(row, ['password', 'pass', 'secret']),
    url: get(row, ['url', 'website', 'web site']),
    notes: get(row, ['notes', 'comments', 'memo']),
  }));
}

const FORMAT_INFO: Record<Format, { label: string; accept: string; hint: string }> = {
  'keepass-xml': {
    label: 'KeePass XML',
    accept: '.xml',
    hint: 'Export from KeePass 2: File > Export > KeePass XML 2.x',
  },
  'keepass-csv': {
    label: 'KeePass CSV',
    accept: '.csv',
    hint: 'Export from KeePass 1.x/2.x: File > Export > CSV',
  },
  '1password-csv': {
    label: '1Password CSV',
    accept: '.csv',
    hint: 'Export from 1Password: File > Export > All Items (CSV)',
  },
  'generic-csv': {
    label: 'Generic CSV',
    accept: '.csv',
    hint: 'Any CSV with columns: name, username, password, url, notes',
  },
};

interface ImportModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function ImportModal({ isOpen, onClose }: ImportModalProps) {
  const { addEntry } = useVault();
  const [format, setFormat] = useState<Format>('keepass-xml');
  const [parsed, setParsed] = useState<ParsedEntry[] | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [importing, setImporting] = useState(false);
  const [imported, setImported] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const fileRef = useRef<HTMLInputElement>(null);

  const handleFile = (file: File) => {
    setParseError(null);
    setParsed(null);
    setSelected(new Set());
    setImported(false);

    const reader = new FileReader();
    reader.onload = (e) => {
      const text = e.target?.result as string;
      try {
        let entries: ParsedEntry[];
        if (format === 'keepass-xml') {
          entries = parseKeePassXml(text);
        } else if (format === 'keepass-csv') {
          entries = parseKeePassCsv(text);
        } else if (format === '1password-csv') {
          entries = parse1PasswordCsv(text);
        } else {
          entries = parseGenericCsv(text);
        }
        if (entries.length === 0) {
          setParseError('No entries found in the file. Make sure you selected the right format.');
          return;
        }
        setParsed(entries);
        setSelected(new Set(entries.map((_, i) => i)));
      } catch (err: any) {
        setParseError(err?.message ?? 'Failed to parse the file.');
      }
    };
    reader.readAsText(file, 'utf-8');
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    const file = e.dataTransfer.files[0];
    if (file) handleFile(file);
  };

  const handleImport = async () => {
    if (!parsed) return;
    setImporting(true);
    let count = 0;
    try {
      for (const [i, entry] of parsed.entries()) {
        if (!selected.has(i)) continue;
        // Determine entry type
        const hasTotp = !!entry.totp;
        await addEntry({
          type: 'login',
          name: entry.name,
          fields: {
            ...(entry.username ? { username: entry.username } : {}),
            ...(entry.password ? { password: entry.password } : {}),
            ...(entry.url ? { url: entry.url } : {}),
          },
          notes: entry.notes ?? '',
          favorite: false,
        });
        count++;

        // If there's a TOTP secret, add a separate TOTP entry
        if (hasTotp) {
          // Extract the secret from an otpauth:// URI if present
          let secret = entry.totp!;
          try {
            const url = new URL(entry.totp!);
            secret = url.searchParams.get('secret') ?? entry.totp!;
          } catch {
            // Not a URL — treat as raw secret
          }
          await addEntry({
            type: 'totp',
            name: `${entry.name} (TOTP)`,
            fields: { secret, issuer: entry.name },
            notes: '',
            favorite: false,
          });
          count++;
        }
      }
      toast.success(`Imported ${count} item${count !== 1 ? 's' : ''}`);
      setImported(true);
    } catch (err: any) {
      toast.error(err?.message ?? 'Import failed');
    } finally {
      setImporting(false);
    }
  };

  const toggleSelect = (i: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(i)) next.delete(i);
      else next.add(i);
      return next;
    });
  };

  const toggleAll = () => {
    if (!parsed) return;
    if (selected.size === parsed.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(parsed.map((_, i) => i)));
    }
  };

  if (!isOpen) return null;

  const info = FORMAT_INFO[format];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        className="bg-cloistr-bg-elevated rounded-lg shadow-xl w-full max-w-2xl flex flex-col"
        style={{ maxHeight: '90dvh' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-cloistr-border flex-shrink-0">
          <div className="flex items-center gap-2">
            <Upload className="h-5 w-5 text-cloistr-primary" />
            <h2 className="text-base font-semibold">Import Entries</h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-cloistr-text-muted hover:text-cloistr-text rounded-md"
            style={{ minHeight: '44px', minWidth: '44px' }}
            aria-label="Close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {imported ? (
            <div className="text-center py-12 space-y-3">
              <Check className="h-12 w-12 text-green-500 mx-auto" />
              <p className="text-lg font-semibold">Import complete</p>
              <p className="text-cloistr-text-muted">Your entries have been added to the vault.</p>
              <button onClick={onClose} className="btn-primary mt-4" style={{ minHeight: '44px' }}>
                Done
              </button>
            </div>
          ) : (
            <>
              {/* Format picker */}
              <div className="space-y-2">
                <label className="text-sm font-medium">Format</label>
                <div className="grid grid-cols-2 gap-2">
                  {(Object.entries(FORMAT_INFO) as [Format, typeof FORMAT_INFO[Format]][]).map(([f, fi]) => (
                    <button
                      key={f}
                      onClick={() => {
                        setFormat(f);
                        setParsed(null);
                        setParseError(null);
                        setSelected(new Set());
                      }}
                      style={{ minHeight: '44px' }}
                      className={`text-sm px-3 py-2 rounded-md border text-left transition-colors ${
                        format === f
                          ? 'border-cloistr-primary bg-cloistr-primary/10 text-cloistr-primary'
                          : 'border-cloistr-border hover:border-cloistr-primary/50'
                      }`}
                    >
                      {fi.label}
                    </button>
                  ))}
                </div>
                <p className="text-xs text-cloistr-text-muted">{info.hint}</p>
              </div>

              {/* Drop zone */}
              {!parsed && (
                <div
                  onDrop={handleDrop}
                  onDragOver={(e) => e.preventDefault()}
                  onClick={() => fileRef.current?.click()}
                  className="border-2 border-dashed border-cloistr-border rounded-lg p-8 text-center cursor-pointer hover:border-cloistr-primary transition-colors"
                >
                  <FileText className="h-10 w-10 text-cloistr-text-muted mx-auto mb-3" />
                  <p className="font-medium">Drop a file here or click to browse</p>
                  <p className="text-sm text-cloistr-text-muted mt-1">Accepts {info.accept} files</p>
                  <input
                    ref={fileRef}
                    type="file"
                    accept={info.accept}
                    className="hidden"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) handleFile(file);
                    }}
                  />
                </div>
              )}

              {/* Parse error */}
              {parseError && (
                <div className="flex items-start gap-2 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-600 dark:text-red-400 text-sm">
                  <AlertCircle className="h-4 w-4 flex-shrink-0 mt-0.5" />
                  {parseError}
                </div>
              )}

              {/* Preview table */}
              {parsed && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium">
                      {parsed.length} entr{parsed.length !== 1 ? 'ies' : 'y'} found
                    </p>
                    <div className="flex items-center gap-3">
                      <button
                        onClick={toggleAll}
                        className="text-xs text-cloistr-primary underline"
                        style={{ minHeight: '44px' }}
                      >
                        {selected.size === parsed.length ? 'Deselect all' : 'Select all'}
                      </button>
                      <button
                        onClick={() => { setParsed(null); setSelected(new Set()); }}
                        className="text-xs text-cloistr-text-muted underline"
                        style={{ minHeight: '44px' }}
                      >
                        Change file
                      </button>
                    </div>
                  </div>
                  <div className="border border-cloistr-border rounded-lg overflow-auto" style={{ maxHeight: '280px' }}>
                    <table className="w-full text-xs">
                      <thead className="bg-cloistr-bg-hover sticky top-0">
                        <tr>
                          <th className="p-2 text-left w-8">
                            <input
                              type="checkbox"
                              checked={selected.size === parsed.length}
                              onChange={toggleAll}
                              className="accent-cloistr-primary h-4 w-4"
                            />
                          </th>
                          <th className="p-2 text-left font-medium">Name</th>
                          <th className="p-2 text-left font-medium hidden sm:table-cell">Username</th>
                          <th className="p-2 text-left font-medium hidden sm:table-cell">URL</th>
                          <th className="p-2 text-left font-medium">Pwd</th>
                        </tr>
                      </thead>
                      <tbody>
                        {parsed.map((entry, i) => (
                          <tr
                            key={i}
                            onClick={() => toggleSelect(i)}
                            className={`border-t border-cloistr-border cursor-pointer transition-colors ${
                              selected.has(i) ? 'bg-cloistr-primary/5' : 'hover:bg-cloistr-bg-hover'
                            }`}
                          >
                            <td className="p-2">
                              <input
                                type="checkbox"
                                checked={selected.has(i)}
                                onChange={() => toggleSelect(i)}
                                onClick={(e) => e.stopPropagation()}
                                className="accent-cloistr-primary h-4 w-4"
                              />
                            </td>
                            <td className="p-2 font-medium truncate max-w-[120px]">{entry.name}</td>
                            <td className="p-2 text-cloistr-text-muted truncate max-w-[100px] hidden sm:table-cell">
                              {entry.username || '-'}
                            </td>
                            <td className="p-2 text-cloistr-text-muted truncate max-w-[100px] hidden sm:table-cell">
                              {entry.url || '-'}
                            </td>
                            <td className="p-2">
                              {entry.password ? (
                                <span className="text-green-500">Yes</span>
                              ) : (
                                <span className="text-cloistr-text-muted">No</span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        {!imported && (
          <div className="p-4 border-t border-cloistr-border flex gap-2 flex-shrink-0">
            <button onClick={onClose} className="btn-outline flex-1" style={{ minHeight: '44px' }}>
              Cancel
            </button>
            {parsed && (
              <button
                onClick={handleImport}
                disabled={importing || selected.size === 0}
                className="btn-primary flex-1 disabled:opacity-40"
                style={{ minHeight: '44px' }}
              >
                {importing ? (
                  <><div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent mr-2" />Importing...</>
                ) : (
                  `Import ${selected.size} item${selected.size !== 1 ? 's' : ''}`
                )}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
