/**
 * Cryptographically secure randomness primitives.
 *
 * Everything here draws from `crypto.getRandomValues`. Nothing in this file may
 * use `Math.random()` — it is a non-cryptographic PRNG (V8 seeds xorshift128+
 * from a low-entropy source and its internal state is recoverable from a short
 * run of outputs), so any password derived from it is far weaker than its
 * length and charset suggest.
 */

/** Fills a buffer of `length` bytes from the platform CSPRNG. */
export function randomBytes(length: number): Uint8Array {
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  return bytes;
}

/**
 * Returns a uniformly distributed integer in [0, max).
 *
 * Uses rejection sampling rather than `random % max`. The modulo approach is
 * biased whenever `max` is not a power of two: the low residues absorb the
 * leftover values at the top of the range, so for a 26-letter alphabet drawn
 * from a byte, 'a'–'d' come up ~1.5% more often than the rest. Rejecting the
 * incomplete final block removes the skew entirely.
 */
export function randomInt(max: number): number {
  if (!Number.isInteger(max) || max <= 0) {
    throw new Error(`randomInt: max must be a positive integer, got ${max}`);
  }
  if (max > 0x100000000) {
    throw new Error(`randomInt: max must fit in 32 bits, got ${max}`);
  }

  // Largest multiple of `max` that fits in 32 bits. Values at or above this
  // land in a partial block and are redrawn.
  const limit = Math.floor(0x100000000 / max) * max;

  const buf = new Uint32Array(1);
  let value: number;
  do {
    crypto.getRandomValues(buf);
    value = buf[0];
  } while (value >= limit);

  return value % max;
}

/**
 * Returns a new array shuffled with Fisher-Yates, driven by the CSPRNG.
 *
 * The idiom this replaces — `arr.sort(() => Math.random() - 0.5)` — is not a
 * shuffle. It hands the sort a comparator that is inconsistent between calls,
 * which violates the ordering contract; the resulting permutation distribution
 * is badly non-uniform and depends on the engine's sort implementation.
 */
export function shuffle<T>(items: readonly T[]): T[] {
  const out = [...items];
  for (let i = out.length - 1; i > 0; i--) {
    const j = randomInt(i + 1);
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

/** Picks one element (or, for a string, one character) uniformly at random. */
export function randomChoice(items: string): string;
export function randomChoice<T>(items: readonly T[]): T;
export function randomChoice<T>(items: readonly T[] | string): T | string {
  if (items.length === 0) {
    throw new Error('randomChoice: cannot choose from an empty list');
  }
  return items[randomInt(items.length)];
}

const LOWERCASE = 'abcdefghijklmnopqrstuvwxyz';
const UPPERCASE = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
const NUMBERS = '0123456789';
const SPECIAL = '!@#$%^&*()_+-=[]{}|;:,.<>?';

/**
 * Generates a password of `length` characters, guaranteeing at least one
 * character from each enabled class.
 *
 * The guaranteed characters are placed first and then the whole string is
 * shuffled, so their positions carry no information.
 */
export function generatePassword(length: number = 16, includeSpecial: boolean = true): string {
  const classes = [LOWERCASE, UPPERCASE, NUMBERS, ...(includeSpecial ? [SPECIAL] : [])];

  if (length < classes.length) {
    throw new Error(
      `generatePassword: length ${length} cannot satisfy ${classes.length} required character classes`
    );
  }

  const charset = classes.join('');

  // One guaranteed character per class, then fill the remainder from the union.
  const chars = classes.map((cls) => randomChoice(cls));
  while (chars.length < length) {
    chars.push(randomChoice(charset));
  }

  return shuffle(chars).join('');
}

// Ambiguous characters that are hard to distinguish visually in some fonts.
const AMBIGUOUS = new Set(['I', 'l', '1', 'O', '0']);

/**
 * Options for the configurable password generator.
 *
 * Every character class defaults to enabled; `excludeAmbiguous` defaults to
 * false so the old behaviour is preserved when no options are supplied.
 */
export interface PasswordOptions {
  length?: number;
  includeLower?: boolean;
  includeUpper?: boolean;
  includeNumbers?: boolean;
  includeSpecial?: boolean;
  excludeAmbiguous?: boolean;
}

/**
 * Generates a password from a full options object.
 *
 * At least one character class must be enabled. Guarantees at least one
 * character from each enabled class, then fills the remainder from the union.
 * The full password is shuffled so the guaranteed characters reveal nothing
 * about where in the string they are.
 */
export function generatePasswordFromOptions(opts: PasswordOptions = {}): string {
  const {
    length = 16,
    includeLower = true,
    includeUpper = true,
    includeNumbers = true,
    includeSpecial = true,
    excludeAmbiguous = false,
  } = opts;

  const filter = (s: string) =>
    excludeAmbiguous ? [...s].filter((c) => !AMBIGUOUS.has(c)).join('') : s;

  const classes = [
    includeLower ? filter(LOWERCASE) : '',
    includeUpper ? filter(UPPERCASE) : '',
    includeNumbers ? filter(NUMBERS) : '',
    includeSpecial ? SPECIAL : '',
  ].filter(Boolean);

  if (classes.length === 0) {
    throw new Error('generatePasswordFromOptions: at least one character class must be enabled');
  }

  if (length < classes.length) {
    throw new Error(
      `generatePasswordFromOptions: length ${length} cannot cover ${classes.length} required character classes`,
    );
  }

  const charset = classes.join('');
  const chars = classes.map((cls) => randomChoice(cls));
  while (chars.length < length) {
    chars.push(randomChoice(charset));
  }

  return shuffle(chars).join('');
}
