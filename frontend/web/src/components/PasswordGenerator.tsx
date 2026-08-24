/**
 * PasswordGenerator — a modal for generating passwords with configurable rules.
 *
 * Mobile-first: the modal uses max-h-dvh and a scrollable body so it stays
 * reachable when the mobile URL bar is visible.
 */
import React, { useState, useCallback } from 'react';
import { X, Copy, RefreshCw, Check } from 'lucide-react';
import { useCrypto } from '../contexts/CryptoContext';
import { passwordStrength } from '../crypto/password-strength';
import toast from 'react-hot-toast';

interface PasswordGeneratorProps {
  isOpen: boolean;
  onClose: () => void;
  /** Called when the user clicks "Use this password". */
  onUse?: (password: string) => void;
}

const STRENGTH_COLORS = [
  'bg-red-500',
  'bg-orange-500',
  'bg-yellow-500',
  'bg-green-500',
  'bg-emerald-500',
];
const STRENGTH_LABELS = ['Very Weak', 'Weak', 'Fair', 'Strong', 'Very Strong'];

export default function PasswordGenerator({ isOpen, onClose, onUse }: PasswordGeneratorProps) {
  const { generatePasswordFromOptions } = useCrypto();

  const [options, setOptions] = useState({
    length: 20,
    includeLower: true,
    includeUpper: true,
    includeNumbers: true,
    includeSpecial: true,
    excludeAmbiguous: false,
  });

  const [password, setPassword] = useState(() => {
    try {
      return generatePasswordFromOptions({ length: 20 });
    } catch {
      return '';
    }
  });
  const [copied, setCopied] = useState(false);

  const anyClassEnabled = options.includeLower || options.includeUpper || options.includeNumbers || options.includeSpecial;

  const regenerate = useCallback(() => {
    if (!anyClassEnabled) return;
    try {
      setPassword(generatePasswordFromOptions(options));
    } catch {
      // length too short for the selected classes — ignored; slider prevents this
    }
  }, [generatePasswordFromOptions, options, anyClassEnabled]);

  const handleOptionChange = (key: keyof typeof options, value: boolean | number) => {
    const next = { ...options, [key]: value };
    // Don't regenerate if no class would be enabled.
    const enabled = next.includeLower || next.includeUpper || next.includeNumbers || next.includeSpecial;
    setOptions(next);
    if (enabled) {
      try {
        setPassword(generatePasswordFromOptions(next));
      } catch {
        // length < number of classes; keep previous password
      }
    }
  };

  const handleCopy = async () => {
    if (!password) return;
    try {
      await navigator.clipboard.writeText(password);
      setCopied(true);
      toast.success('Password copied');
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error('Failed to copy');
    }
  };

  const handleUse = () => {
    if (password && onUse) {
      onUse(password);
      onClose();
    }
  };

  if (!isOpen) return null;

  const strength = password ? passwordStrength(password) : null;
  // Minimum length = number of enabled classes (each must contribute at least 1 char)
  const minLength = [options.includeLower, options.includeUpper, options.includeNumbers, options.includeSpecial].filter(Boolean).length || 1;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        className="bg-cloistr-bg-elevated rounded-lg shadow-xl w-full max-w-sm flex flex-col"
        style={{ maxHeight: '90dvh' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-cloistr-border flex-shrink-0">
          <h2 className="text-base font-semibold">Password Generator</h2>
          <button
            onClick={onClose}
            className="p-2 text-cloistr-text-muted hover:text-cloistr-text rounded-md"
            style={{ minHeight: '44px', minWidth: '44px' }}
            aria-label="Close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto p-4 space-y-5">
          {/* Generated password */}
          <div className="space-y-2">
            <div className="flex gap-2">
              <div className="flex-1 font-mono text-sm bg-cloistr-bg rounded-md px-3 py-2 border border-cloistr-border break-all min-h-[44px] flex items-center select-all">
                {password || <span className="text-cloistr-text-muted">No classes enabled</span>}
              </div>
              <button
                onClick={handleCopy}
                disabled={!password}
                style={{ minHeight: '44px', minWidth: '44px' }}
                className="flex items-center justify-center rounded-md border border-cloistr-border hover:bg-cloistr-bg-hover text-cloistr-text-muted hover:text-cloistr-text disabled:opacity-40"
                aria-label="Copy password"
              >
                {copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
              </button>
              <button
                onClick={regenerate}
                disabled={!anyClassEnabled}
                style={{ minHeight: '44px', minWidth: '44px' }}
                className="flex items-center justify-center rounded-md border border-cloistr-border hover:bg-cloistr-bg-hover text-cloistr-text-muted hover:text-cloistr-text disabled:opacity-40"
                aria-label="Regenerate"
              >
                <RefreshCw className="h-4 w-4" />
              </button>
            </div>

            {/* Strength meter */}
            {strength && (
              <div className="space-y-1">
                <div className="flex gap-1">
                  {[0, 1, 2, 3, 4].map((i) => (
                    <div
                      key={i}
                      className={`h-1.5 flex-1 rounded-full transition-colors ${
                        i <= strength.score ? STRENGTH_COLORS[strength.score] : 'bg-cloistr-border'
                      }`}
                    />
                  ))}
                </div>
                <div className="flex justify-between text-xs text-cloistr-text-muted">
                  <span>{STRENGTH_LABELS[strength.score]}</span>
                  <span>~{Math.round(strength.entropy)} bits</span>
                </div>
              </div>
            )}
          </div>

          {/* Length slider */}
          <div className="space-y-2">
            <div className="flex justify-between text-sm">
              <label className="font-medium">Length</label>
              <span className="font-mono font-bold text-cloistr-primary">{options.length}</span>
            </div>
            <input
              type="range"
              min={Math.max(minLength, 4)}
              max={128}
              value={options.length}
              onChange={(e) => handleOptionChange('length', parseInt(e.target.value, 10))}
              className="w-full accent-cloistr-primary"
              style={{ minHeight: '44px' }}
            />
            <div className="flex justify-between text-xs text-cloistr-text-muted">
              <span>4</span>
              <span>128</span>
            </div>
          </div>

          {/* Character class toggles */}
          <div className="space-y-2">
            <span className="text-sm font-medium">Character types</span>
            <div className="grid grid-cols-2 gap-2">
              {(
                [
                  ['includeLower', 'Lowercase (a-z)'],
                  ['includeUpper', 'Uppercase (A-Z)'],
                  ['includeNumbers', 'Numbers (0-9)'],
                  ['includeSpecial', 'Special (!@#...)'],
                ] as [keyof typeof options, string][]
              ).map(([key, label]) => (
                <label
                  key={key}
                  className="flex items-center gap-2 p-2 rounded-md border border-cloistr-border cursor-pointer hover:bg-cloistr-bg-hover select-none"
                  style={{ minHeight: '44px' }}
                >
                  <input
                    type="checkbox"
                    checked={options[key] as boolean}
                    onChange={(e) => handleOptionChange(key, e.target.checked)}
                    className="accent-cloistr-primary h-4 w-4 flex-shrink-0"
                  />
                  <span className="text-xs">{label}</span>
                </label>
              ))}
            </div>
          </div>

          {/* Exclude ambiguous toggle */}
          <label
            className="flex items-center gap-3 p-3 rounded-md border border-cloistr-border cursor-pointer hover:bg-cloistr-bg-hover select-none"
            style={{ minHeight: '44px' }}
          >
            <input
              type="checkbox"
              checked={options.excludeAmbiguous}
              onChange={(e) => handleOptionChange('excludeAmbiguous', e.target.checked)}
              className="accent-cloistr-primary h-4 w-4 flex-shrink-0"
            />
            <div>
              <span className="text-sm font-medium">Exclude ambiguous chars</span>
              <p className="text-xs text-cloistr-text-muted">Avoid I, l, 1, O, 0</p>
            </div>
          </label>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-cloistr-border flex gap-2 flex-shrink-0">
          <button onClick={onClose} className="btn-outline flex-1" style={{ minHeight: '44px' }}>
            Cancel
          </button>
          {onUse && (
            <button
              onClick={handleUse}
              disabled={!password}
              className="btn-primary flex-1 disabled:opacity-40"
              style={{ minHeight: '44px' }}
            >
              Use this password
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
