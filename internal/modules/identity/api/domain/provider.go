package domain

import "context"

// ExternalIdentity = hasil verifikasi token, murni identitas — TIDAK ada role/status di sini. authz urusan internal ajha
type ExternalIdentity struct {
	ProviderUID   string
	Email         string
	EmailVerified bool
}

type IdentityProvider interface {
	VerifyToken(ctx context.Context, rawToken string) (*ExternalIdentity, error)
}
