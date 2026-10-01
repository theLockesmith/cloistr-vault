package auth

import "github.com/coldforge/vault/internal/models"

// Lightning (LNURL-auth) login is switched off.
//
// The previous implementation looked the user up by the Lightning Address the
// client claimed and only checked that the signature matched the linking key
// sent alongside it. Nothing tied that key to the address, so anyone could mint
// a key, name someone else's address, and receive a session for that account.
// It also verified sha256(k1) with a compact signature, which no LUD-04 wallet
// produces (wallets sign k1 itself, DER-encoded), and the web client never sent
// lightning_address, so it could not have worked for a real user either.
//
// A working version needs the LUD-04 callback flow, with the account keyed by
// the linking key. Until then both entry points refuse.

// GenerateLightningChallenge refuses: Lightning login is disabled.
func (a *AuthService) GenerateLightningChallenge(lightningAddress string) (*Challenge, error) {
	return nil, ErrLightningDisabled
}

// AuthenticateWithLightning refuses: Lightning login is disabled.
func (a *AuthService) AuthenticateWithLightning(lightningAddress, signature, k1, linkingKey string) (*models.User, string, error) {
	return nil, "", ErrLightningDisabled
}
