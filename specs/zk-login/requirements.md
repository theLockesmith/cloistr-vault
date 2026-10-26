Status: DRAFT (awaiting security review and cloistr-pr review, 2026-10-20)

# Requirements: email sign-in never sends the master password

Directed by cloistr-orchestrator 2026-10-20, from cloistr-sanctuary's policy
review. Confirmed in code and on prod the same day: the web client sent the
master password to `/api/v1/auth/register` and `/api/v1/auth/login`.

1. Register and sign-in send an auth key derived on the device; the request
   body never contains the typed password.
2. The server refuses a password at registration, and refuses it for any
   upgraded account.
3. Accounts created before the change upgrade on their first sign-in from the
   new client and never send the password again.
4. Unknown email and wrong key get the same status, the same body, and the
   same scrypt work, so neither reveals which emails have accounts.
5. Changing an email must re-derive the auth key and replace the verifier in
   one step (the salt is the email). No email-change path exists today.
6. Recovery sets an auth key, not a password.
7. `docs/security.md` and the sign-in wording say what is now true, and record
   what was not.
