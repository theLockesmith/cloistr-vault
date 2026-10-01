/**
 * Password strength scoring — pure client-side, no external libraries.
 *
 * Estimates bits of entropy from character class breadth and length, then
 * applies penalties for repeated characters and sequential patterns (keyboard
 * walks, digit runs, alphabet runs). Returns a 0–4 score with a human-readable
 * label and actionable suggestions.
 *
 * This is not a cracking-time predictor; it is a heuristic that gives
 * meaningful signal for the typical vault-entry context without the weight of
 * a full dictionary attack simulator.
 */

export interface StrengthResult {
  score: 0 | 1 | 2 | 3 | 4;
  label: 'Very Weak' | 'Weak' | 'Fair' | 'Strong' | 'Very Strong';
  /** Estimated bits of entropy (after pattern penalties). */
  entropy: number;
  suggestions: string[];
}

/** Returns the number of distinct characters usable by the password's charset. */
function charsetSize(pw: string): number {
  let size = 0;
  if (/[a-z]/.test(pw)) size += 26;
  if (/[A-Z]/.test(pw)) size += 26;
  if (/[0-9]/.test(pw)) size += 10;
  if (/[^a-zA-Z0-9]/.test(pw)) size += 32;
  return size || 1;
}

// Sequences in forward direction; the scorer also checks their reverses.
const FORWARD_SEQUENCES = [
  '0123456789',
  'abcdefghijklmnopqrstuvwxyz',
  'qwertyuiopasdfghjklzxcvbnm',
  'zxcvbnmasdfghjklqwertyuiop',
];

/**
 * Counts 3-character sequential runs (forward and backward) in the password.
 * A keyboard walk like "qwe" or "321" each contribute 1 to the count.
 */
function countSequences(pw: string): number {
  const lower = pw.toLowerCase();
  let count = 0;
  for (const seq of FORWARD_SEQUENCES) {
    const rev = [...seq].reverse().join('');
    for (let i = 0; i <= lower.length - 3; i++) {
      const triple = lower.slice(i, i + 3);
      if (seq.includes(triple) || rev.includes(triple)) {
        count++;
      }
    }
  }
  return count;
}

/**
 * Scores a password's strength.
 *
 * The function is intentionally O(n) on the password length and has no
 * async dependencies — it can be called on every keystroke.
 */
export function passwordStrength(pw: string): StrengthResult {
  if (!pw) {
    return {
      score: 0,
      label: 'Very Weak',
      entropy: 0,
      suggestions: ['Enter a password'],
    };
  }

  const suggestions: string[] = [];

  // Raw entropy from character set and length.
  const charset = charsetSize(pw);
  let entropy = pw.length * Math.log2(charset);

  // Penalise repeated characters: each excess repeat (over the first occurrence)
  // is worth 2 bits of penalty. A password of all identical characters gets
  // nearly zero credit for its repetitions.
  const uniqueChars = new Set(pw).size;
  const repeats = pw.length - uniqueChars;
  entropy -= repeats * 2;

  // Penalise sequential patterns.
  const seqCount = countSequences(pw);
  entropy -= seqCount * 6;

  entropy = Math.max(0, entropy);

  // Build suggestions.
  if (pw.length < 12) suggestions.push('Use at least 12 characters');
  if (!/[A-Z]/.test(pw)) suggestions.push('Add uppercase letters');
  if (!/[0-9]/.test(pw)) suggestions.push('Add numbers');
  if (!/[^a-zA-Z0-9]/.test(pw)) suggestions.push('Add special characters');
  if (repeats > pw.length / 2) suggestions.push('Avoid repeated characters');
  if (seqCount > 0) suggestions.push('Avoid sequential patterns (abc, 123, qwe)');

  // Entropy thresholds tuned to give realistic labels:
  // < 28 bits  → Very Weak  (≤ 8 lower-only chars, or any trivially guessable string)
  // < 36 bits  → Weak       (short mixed, or longer single-class)
  // < 50 bits  → Fair       (typical 10-char mixed)
  // < 65 bits  → Strong     (12+ chars, multi-class)
  // ≥ 65 bits  → Very Strong
  let score: 0 | 1 | 2 | 3 | 4;
  let label: StrengthResult['label'];

  if (entropy < 28) {
    score = 0;
    label = 'Very Weak';
  } else if (entropy < 36) {
    score = 1;
    label = 'Weak';
  } else if (entropy < 50) {
    score = 2;
    label = 'Fair';
  } else if (entropy < 65) {
    score = 3;
    label = 'Strong';
  } else {
    score = 4;
    label = 'Very Strong';
  }

  return { score, label, entropy, suggestions };
}

/**
 * Identifies groups of entries whose passwords are identical.
 *
 * Returns a list of groups; each group contains the names of entries that
 * share the same password. Groups with only one entry are omitted.
 */
export function findDuplicatePasswords(
  entries: Array<{ name: string; fields: Record<string, string> }>
): Array<{ password: string; entryNames: string[] }> {
  const map = new Map<string, string[]>();
  for (const entry of entries) {
    const pw = entry.fields.password;
    if (!pw) continue;
    const list = map.get(pw) ?? [];
    list.push(entry.name);
    map.set(pw, list);
  }
  const duplicates: Array<{ password: string; entryNames: string[] }> = [];
  for (const [password, entryNames] of map.entries()) {
    if (entryNames.length > 1) {
      duplicates.push({ password, entryNames });
    }
  }
  return duplicates;
}
