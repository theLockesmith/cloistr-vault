Status: DRAFT (awaiting security review and cloistr-pr review, 2026-10-20)

# Design: email sign-in never sends the master password

- Client: `deriveAuthKey(email, password)` in `packages/vault-crypto` =
  scrypt(password, sha256("cloistr-vault/auth/v1|" + lowercase(trim(email))),
  N=32768 r=8 p=1), 32 bytes as hex. The KEK keeps its random envelope salt, so
  the two are independent.
- Server: `auth_methods.auth_version` (migration 008). 1 = verifier over the
  password (all accounts before this change), 2 = verifier over the auth key.
  Register writes 2 and returns 400 if a password is sent.
- Sign-in: v2 takes only `auth_key`. v1 with `password` verifies it, and if
  `auth_key` came too, swaps the verifier (`WHERE auth_version = 1`, so a
  concurrent upgrade is a no-op). v1 with only `auth_key` returns 409
  `AUTH_UPGRADE_REQUIRED`; the client retries once with both. A v1 account
  sent only a password (a tab loaded before the deploy) still signs in and
  stays v1.
- Unknown email: scrypt against a dummy salt, then the same 401 as a wrong
  key. The 409 does reveal not-yet-upgraded accounts; that set only shrinks,
  and prod had none when this shipped.
- Recovery: `new_auth_key` replaces `new_password`; unknown email and bad code
  return the same 401. Timing there is not equalised (code checks run up to
  eight scrypts); recovery has no UI.
