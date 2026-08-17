import { randomBytes, randomInt, shuffle, randomChoice, generatePassword } from './random';

describe('randomBytes', () => {
  it('returns the requested length', () => {
    expect(randomBytes(0)).toHaveLength(0);
    expect(randomBytes(32)).toHaveLength(32);
  });

  it('does not repeat across calls', () => {
    const a = Buffer.from(randomBytes(32)).toString('hex');
    const b = Buffer.from(randomBytes(32)).toString('hex');
    expect(a).not.toEqual(b);
  });
});

describe('randomInt', () => {
  it('rejects non-positive and non-integer bounds', () => {
    expect(() => randomInt(0)).toThrow();
    expect(() => randomInt(-1)).toThrow();
    expect(() => randomInt(1.5)).toThrow();
  });

  it('always returns 0 for max=1', () => {
    for (let i = 0; i < 50; i++) {
      expect(randomInt(1)).toBe(0);
    }
  });

  it('stays within [0, max)', () => {
    for (let i = 0; i < 2000; i++) {
      const v = randomInt(26);
      expect(v).toBeGreaterThanOrEqual(0);
      expect(v).toBeLessThan(26);
    }
  });

  it('is not detectably biased across a non-power-of-two range', () => {
    // 26 does not divide 2^32 evenly, so a naive `% 26` would over-represent
    // the low residues. With 52k samples the expected count per bucket is
    // 2000; a modulo-biased generator skews the first buckets well outside
    // the tolerance below, while rejection sampling stays inside it.
    const buckets = new Array(26).fill(0);
    const samples = 52000;
    for (let i = 0; i < samples; i++) {
      buckets[randomInt(26)]++;
    }

    const expected = samples / 26;
    for (const count of buckets) {
      expect(Math.abs(count - expected) / expected).toBeLessThan(0.15);
    }
  });
});

describe('shuffle', () => {
  it('preserves the multiset of elements', () => {
    const input = [1, 2, 3, 4, 5, 6, 7, 8];
    const out = shuffle(input);
    expect(out).toHaveLength(input.length);
    expect([...out].sort((a, b) => a - b)).toEqual(input);
  });

  it('does not mutate its input', () => {
    const input = [1, 2, 3, 4, 5];
    shuffle(input);
    expect(input).toEqual([1, 2, 3, 4, 5]);
  });

  it('actually permutes', () => {
    const input = Array.from({ length: 24 }, (_, i) => i);
    const permuted = Array.from({ length: 20 }, () => shuffle(input).join(','));
    expect(new Set(permuted).size).toBeGreaterThan(1);
  });

  it('distributes a given element across positions', () => {
    // Element 0 should reach every one of the 5 slots over enough runs. A
    // comparator-based "shuffle" leaves it heavily concentrated near its
    // starting index.
    const seen = new Set<number>();
    for (let i = 0; i < 500; i++) {
      seen.add(shuffle([0, 1, 2, 3, 4]).indexOf(0));
    }
    expect(seen.size).toBe(5);
  });
});

describe('randomChoice', () => {
  it('throws on empty input', () => {
    expect(() => randomChoice([])).toThrow();
    expect(() => randomChoice('')).toThrow();
  });

  it('picks from arrays and strings', () => {
    expect(['a', 'b', 'c']).toContain(randomChoice(['a', 'b', 'c']));
    expect('xyz').toContain(randomChoice('xyz'));
  });
});

describe('generatePassword', () => {
  const hasLower = (s: string) => /[a-z]/.test(s);
  const hasUpper = (s: string) => /[A-Z]/.test(s);
  const hasDigit = (s: string) => /[0-9]/.test(s);
  const hasSpecial = (s: string) => /[!@#$%^&*()_+\-=[\]{}|;:,.<>?]/.test(s);

  it('honours the requested length', () => {
    expect(generatePassword(16)).toHaveLength(16);
    expect(generatePassword(64)).toHaveLength(64);
    expect(generatePassword(4, false)).toHaveLength(4);
  });

  it('defaults to 16 characters with specials', () => {
    const pw = generatePassword();
    expect(pw).toHaveLength(16);
  });

  it('includes every required character class', () => {
    for (let i = 0; i < 200; i++) {
      const pw = generatePassword(8);
      expect(hasLower(pw)).toBe(true);
      expect(hasUpper(pw)).toBe(true);
      expect(hasDigit(pw)).toBe(true);
      expect(hasSpecial(pw)).toBe(true);
    }
  });

  it('omits specials when asked', () => {
    for (let i = 0; i < 100; i++) {
      expect(hasSpecial(generatePassword(20, false))).toBe(false);
    }
  });

  it('throws when the length cannot cover the required classes', () => {
    expect(() => generatePassword(3)).toThrow();
    expect(() => generatePassword(2, false)).toThrow();
  });

  it('does not place the guaranteed characters at fixed positions', () => {
    // The generator seeds one char per class then shuffles. If the shuffle
    // were absent or ineffective, index 0 would always be lowercase.
    const firstCharClasses = new Set<string>();
    for (let i = 0; i < 300; i++) {
      const c = generatePassword(16)[0];
      if (/[a-z]/.test(c)) firstCharClasses.add('lower');
      else if (/[A-Z]/.test(c)) firstCharClasses.add('upper');
      else if (/[0-9]/.test(c)) firstCharClasses.add('digit');
      else firstCharClasses.add('special');
    }
    expect(firstCharClasses.size).toBeGreaterThan(1);
  });

  it('does not repeat across calls', () => {
    const generated = new Set(Array.from({ length: 200 }, () => generatePassword(20)));
    expect(generated.size).toBe(200);
  });
});
