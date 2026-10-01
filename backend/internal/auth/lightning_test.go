package auth

import "testing"

func TestLightningLogin_Disabled(t *testing.T) {
	a := newTestAuth()
	if ch, err := a.GenerateLightningChallenge("victim@example.com"); err != ErrLightningDisabled {
		t.Fatalf("GenerateLightningChallenge = %+v, %v; want ErrLightningDisabled", ch, err)
	}
	// The old path admitted any freshly minted linking key for any address.
	if u, _, err := a.AuthenticateWithLightning("victim@example.com", "00", "11", "02"); err != ErrLightningDisabled {
		t.Fatalf("AuthenticateWithLightning = %+v, %v; want ErrLightningDisabled", u, err)
	}
}
