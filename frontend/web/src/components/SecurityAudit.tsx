/**
 * SecurityAudit — password health report for the vault.
 *
 * Shows weak passwords, duplicate passwords, and entries with no password
 * set. Clicking an entry opens the edit modal.
 */
import React, { useMemo, useState } from 'react';
import { Shield, AlertTriangle, AlertCircle, CheckCircle, X, Copy } from 'lucide-react';
import { useVault, VaultEntry } from '../contexts/VaultContext';
import { passwordStrength, findDuplicatePasswords } from '../crypto/password-strength';
import toast from 'react-hot-toast';

interface SecurityAuditProps {
  isOpen: boolean;
  onClose: () => void;
  onEditEntry: (entry: VaultEntry) => void;
}

const SCORE_COLORS = [
  'text-red-500',
  'text-orange-500',
  'text-yellow-500',
  'text-green-500',
  'text-emerald-500',
];

const SCORE_BG = [
  'bg-red-500/10 border-red-500/30',
  'bg-orange-500/10 border-orange-500/30',
  'bg-yellow-500/10 border-yellow-500/30',
  'bg-green-500/10 border-green-500/30',
  'bg-emerald-500/10 border-emerald-500/30',
];

const BAR_COLORS = ['bg-red-500', 'bg-orange-500', 'bg-yellow-500', 'bg-green-500', 'bg-emerald-500'];

interface ScoredEntry {
  entry: VaultEntry;
  score: 0 | 1 | 2 | 3 | 4;
  label: string;
  entropy: number;
}

export default function SecurityAudit({ isOpen, onClose, onEditEntry }: SecurityAuditProps) {
  const { vaultData } = useVault();
  const [activeTab, setActiveTab] = useState<'overview' | 'weak' | 'duplicates' | 'missing'>('overview');

  const loginEntries = useMemo(
    () => (vaultData?.entries ?? []).filter((e) => e.type === 'login'),
    [vaultData],
  );

  const scored: ScoredEntry[] = useMemo(
    () =>
      loginEntries
        .filter((e) => e.fields.password)
        .map((entry) => {
          const r = passwordStrength(entry.fields.password);
          return { entry, score: r.score, label: r.label, entropy: r.entropy };
        })
        .sort((a, b) => a.score - b.score),
    [loginEntries],
  );

  const weakEntries = scored.filter((s) => s.score <= 1);
  const fairEntries = scored.filter((s) => s.score === 2);
  const strongEntries = scored.filter((s) => s.score >= 3);

  const duplicateGroups = useMemo(() => findDuplicatePasswords(loginEntries), [loginEntries]);

  const missingPassword = loginEntries.filter((e) => !e.fields.password);

  const overallScore = scored.length === 0 ? null : Math.round(scored.reduce((s, e) => s + e.score, 0) / scored.length);

  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`${label} copied`);
    } catch {
      toast.error('Failed to copy');
    }
  };

  if (!isOpen) return null;

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
            <Shield className="h-5 w-5 text-cloistr-primary" />
            <h2 className="text-base font-semibold">Security Audit</h2>
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

        {/* Tabs */}
        <div className="flex border-b border-cloistr-border overflow-x-auto flex-shrink-0">
          {(
            [
              ['overview', 'Overview'],
              ['weak', `Weak (${weakEntries.length + fairEntries.length})`],
              ['duplicates', `Duplicates (${duplicateGroups.length})`],
              ['missing', `Missing (${missingPassword.length})`],
            ] as [typeof activeTab, string][]
          ).map(([tab, label]) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              style={{ minHeight: '44px' }}
              className={`px-4 py-2 text-sm font-medium whitespace-nowrap border-b-2 transition-colors ${
                activeTab === tab
                  ? 'border-cloistr-primary text-cloistr-primary'
                  : 'border-transparent text-cloistr-text-muted hover:text-cloistr-text'
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {activeTab === 'overview' && (
            <div className="space-y-4">
              {loginEntries.length === 0 ? (
                <p className="text-cloistr-text-muted text-center py-8">
                  No login entries to audit yet.
                </p>
              ) : (
                <>
                  {/* Overall health */}
                  <div className="card p-4">
                    <div className="flex items-center gap-4">
                      {overallScore !== null && (
                        <div
                          className={`text-4xl font-bold ${SCORE_COLORS[overallScore]}`}
                          title="Average strength score (0–4)"
                        >
                          {overallScore}/4
                        </div>
                      )}
                      <div>
                        <p className="font-semibold">
                          {overallScore === null
                            ? 'No passwords scored'
                            : overallScore >= 3
                            ? 'Your vault is in good shape'
                            : overallScore >= 2
                            ? 'Some passwords need attention'
                            : 'Several passwords need improvement'}
                        </p>
                        <p className="text-sm text-cloistr-text-muted">
                          {scored.length} password{scored.length !== 1 ? 's' : ''} analysed
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* Stats grid */}
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    {[
                      { label: 'Very Weak', count: scored.filter((s) => s.score === 0).length, color: SCORE_COLORS[0], bg: SCORE_BG[0] },
                      { label: 'Weak', count: weakEntries.filter((s) => s.score === 1).length, color: SCORE_COLORS[1], bg: SCORE_BG[1] },
                      { label: 'Fair', count: fairEntries.length, color: SCORE_COLORS[2], bg: SCORE_BG[2] },
                      { label: 'Strong+', count: strongEntries.length, color: SCORE_COLORS[4], bg: SCORE_BG[4] },
                    ].map(({ label, count, color, bg }) => (
                      <div key={label} className={`rounded-lg border p-3 ${bg}`}>
                        <div className={`text-2xl font-bold ${color}`}>{count}</div>
                        <div className="text-xs text-cloistr-text-muted mt-1">{label}</div>
                      </div>
                    ))}
                  </div>

                  {/* Issues summary */}
                  <div className="space-y-2">
                    {weakEntries.length > 0 && (
                      <div className="flex items-start gap-3 p-3 rounded-lg bg-red-500/10 border border-red-500/30">
                        <AlertCircle className="h-4 w-4 text-red-500 flex-shrink-0 mt-0.5" />
                        <p className="text-sm">
                          <span className="font-semibold">{weakEntries.length}</span> password
                          {weakEntries.length !== 1 ? 's are' : ' is'} very weak or weak.{' '}
                          <button
                            onClick={() => setActiveTab('weak')}
                            className="underline text-cloistr-primary"
                          >
                            Review them
                          </button>
                        </p>
                      </div>
                    )}
                    {duplicateGroups.length > 0 && (
                      <div className="flex items-start gap-3 p-3 rounded-lg bg-orange-500/10 border border-orange-500/30">
                        <AlertTriangle className="h-4 w-4 text-orange-500 flex-shrink-0 mt-0.5" />
                        <p className="text-sm">
                          <span className="font-semibold">{duplicateGroups.length}</span> duplicate
                          password group{duplicateGroups.length !== 1 ? 's' : ''} found.{' '}
                          <button
                            onClick={() => setActiveTab('duplicates')}
                            className="underline text-cloistr-primary"
                          >
                            Review them
                          </button>
                        </p>
                      </div>
                    )}
                    {missingPassword.length > 0 && (
                      <div className="flex items-start gap-3 p-3 rounded-lg bg-yellow-500/10 border border-yellow-500/30">
                        <AlertTriangle className="h-4 w-4 text-yellow-500 flex-shrink-0 mt-0.5" />
                        <p className="text-sm">
                          <span className="font-semibold">{missingPassword.length}</span>{' '}
                          login entr{missingPassword.length !== 1 ? 'ies' : 'y'} missing a password.{' '}
                          <button
                            onClick={() => setActiveTab('missing')}
                            className="underline text-cloistr-primary"
                          >
                            Review
                          </button>
                        </p>
                      </div>
                    )}
                    {weakEntries.length === 0 && duplicateGroups.length === 0 && missingPassword.length === 0 && (
                      <div className="flex items-center gap-3 p-3 rounded-lg bg-green-500/10 border border-green-500/30">
                        <CheckCircle className="h-4 w-4 text-green-500 flex-shrink-0" />
                        <p className="text-sm font-medium text-green-700 dark:text-green-400">
                          No issues found. Keep it up!
                        </p>
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>
          )}

          {activeTab === 'weak' && (
            <div className="space-y-2">
              {weakEntries.length === 0 && fairEntries.length === 0 ? (
                <p className="text-cloistr-text-muted text-center py-8">No weak passwords found.</p>
              ) : (
                [...weakEntries, ...fairEntries].map(({ entry, score, label, entropy }) => (
                  <div
                    key={entry.id}
                    className={`rounded-lg border p-3 ${SCORE_BG[score]}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className={`text-xs font-medium ${SCORE_COLORS[score]}`}>{label}</span>
                          <span className="text-xs text-cloistr-text-muted">~{Math.round(entropy)} bits</span>
                        </div>
                        <p className="font-medium text-sm truncate mt-0.5">{entry.name}</p>
                        {entry.fields.username && (
                          <p className="text-xs text-cloistr-text-muted truncate">{entry.fields.username}</p>
                        )}
                        {/* Strength bar */}
                        <div className="flex gap-0.5 mt-2">
                          {[0,1,2,3,4].map((i) => (
                            <div
                              key={i}
                              className={`h-1 flex-1 rounded-full ${i <= score ? BAR_COLORS[score] : 'bg-cloistr-border'}`}
                            />
                          ))}
                        </div>
                      </div>
                      <button
                        onClick={() => { onEditEntry(entry); onClose(); }}
                        className="btn-outline text-xs flex-shrink-0"
                        style={{ minHeight: '44px' }}
                      >
                        Fix
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}

          {activeTab === 'duplicates' && (
            <div className="space-y-4">
              {duplicateGroups.length === 0 ? (
                <p className="text-cloistr-text-muted text-center py-8">No duplicate passwords found.</p>
              ) : (
                duplicateGroups.map((group, i) => {
                  const groupEntries = loginEntries.filter((e) => group.entryNames.includes(e.name));
                  return (
                    <div key={i} className="card p-4 space-y-3">
                      <p className="text-sm font-medium text-orange-500">
                        {group.entryNames.length} sites share the same password
                      </p>
                      <div className="space-y-2">
                        {groupEntries.map((entry) => (
                          <div key={entry.id} className="flex items-center justify-between gap-2">
                            <div className="min-w-0 flex-1">
                              <p className="font-medium text-sm truncate">{entry.name}</p>
                              {entry.fields.username && (
                                <p className="text-xs text-cloistr-text-muted truncate">{entry.fields.username}</p>
                              )}
                            </div>
                            <button
                              onClick={() => { onEditEntry(entry); onClose(); }}
                              className="btn-outline text-xs flex-shrink-0"
                              style={{ minHeight: '44px' }}
                            >
                              Fix
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}

          {activeTab === 'missing' && (
            <div className="space-y-2">
              {missingPassword.length === 0 ? (
                <p className="text-cloistr-text-muted text-center py-8">All login entries have passwords.</p>
              ) : (
                missingPassword.map((entry) => (
                  <div key={entry.id} className="card p-3 flex items-center justify-between gap-2">
                    <div className="min-w-0 flex-1">
                      <p className="font-medium text-sm truncate">{entry.name}</p>
                      {entry.fields.username && (
                        <p className="text-xs text-cloistr-text-muted truncate">{entry.fields.username}</p>
                      )}
                    </div>
                    <button
                      onClick={() => { onEditEntry(entry); onClose(); }}
                      className="btn-outline text-xs flex-shrink-0"
                      style={{ minHeight: '44px' }}
                    >
                      Edit
                    </button>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
