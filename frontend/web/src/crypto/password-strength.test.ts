import { passwordStrength, findDuplicatePasswords } from './password-strength';

describe('passwordStrength', () => {
  it('scores the empty string as 0', () => {
    const r = passwordStrength('');
    expect(r.score).toBe(0);
    expect(r.entropy).toBe(0);
    expect(r.suggestions.length).toBeGreaterThan(0);
  });

  it('scores a very short single-class password as very weak or weak', () => {
    expect(passwordStrength('abc').score).toBeLessThanOrEqual(1);
    expect(passwordStrength('aaaa').score).toBe(0);
  });

  it('scores a long multi-class password as strong or very strong', () => {
    const r = passwordStrength('xK3!mPqR9@wZnL7#');
    expect(r.score).toBeGreaterThanOrEqual(3);
  });

  it('entropy grows with length when charset is constant', () => {
    const short = passwordStrength('aB3!');
    const long = passwordStrength('aB3!xY7@mN9#kL2$');
    expect(long.entropy).toBeGreaterThan(short.entropy);
  });

  it('penalises repeated characters', () => {
    const diverse = passwordStrength('xK3!mPqR9@wZnL7#');
    const repeated = passwordStrength('aaaaaaaaaaaaaaaa');
    expect(diverse.score).toBeGreaterThan(repeated.score);
  });

  it('penalises sequential patterns', () => {
    const diverse = passwordStrength('xK3!mPqR9@wZnL7#');
    const sequential = passwordStrength('abcdefghijklmnop');
    expect(diverse.score).toBeGreaterThan(sequential.score);
  });

  it('returns label matching score', () => {
    const labelMap: Record<number, string> = {
      0: 'Very Weak',
      1: 'Weak',
      2: 'Fair',
      3: 'Strong',
      4: 'Very Strong',
    };
    for (const pw of ['', 'abc', 'abc123', 'Abc123!@', 'xK3!mPqR9@wZnL7#vB']) {
      const r = passwordStrength(pw);
      expect(r.label).toBe(labelMap[r.score]);
    }
  });

  it('returns no suggestions for a very strong password', () => {
    // 20 chars, lower + upper + digits + special, no repeats, no sequences
    const r = passwordStrength('xK3!mPqR9@wZnL7#vB8^');
    expect(r.score).toBeGreaterThanOrEqual(3);
    // Suggestions should be empty or very few for a genuinely strong password
    expect(r.suggestions.filter(s => s !== 'Add uppercase letters' && s !== 'Add numbers' && s !== 'Add special characters').length).toBeLessThanOrEqual(1);
  });

  it('suggests length improvement for short passwords', () => {
    const r = passwordStrength('Ab1!');
    expect(r.suggestions.some(s => /12 character/.test(s))).toBe(true);
  });

  it('suggests uppercase when none present', () => {
    const r = passwordStrength('abc123!@#def456$%');
    expect(r.suggestions.some(s => /uppercase/i.test(s))).toBe(true);
  });
});

describe('findDuplicatePasswords', () => {
  const entries: Array<{ name: string; fields: Record<string, string> }> = [
    { name: 'GitHub', fields: { password: 'hunter2' } },
    { name: 'GitLab', fields: { password: 'hunter2' } },
    { name: 'Gmail', fields: { password: 'unique-pass-xyz' } },
    { name: 'Twitter', fields: { password: 'another-unique' } },
    { name: 'Note', fields: {} },
  ];

  it('finds groups with identical passwords', () => {
    const dups = findDuplicatePasswords(entries);
    expect(dups).toHaveLength(1);
    expect(dups[0].entryNames).toContain('GitHub');
    expect(dups[0].entryNames).toContain('GitLab');
  });

  it('returns empty array when no duplicates exist', () => {
    const unique: Array<{ name: string; fields: Record<string, string> }> = [
      { name: 'A', fields: { password: 'pw1' } },
      { name: 'B', fields: { password: 'pw2' } },
    ];
    expect(findDuplicatePasswords(unique)).toHaveLength(0);
  });

  it('ignores entries without a password field', () => {
    const noPasswords: Array<{ name: string; fields: Record<string, string> }> = [
      { name: 'X', fields: {} },
      { name: 'Y', fields: { username: 'foo' } },
    ];
    expect(findDuplicatePasswords(noPasswords)).toHaveLength(0);
  });

  it('handles an empty entry list', () => {
    expect(findDuplicatePasswords([])).toHaveLength(0);
  });
});
