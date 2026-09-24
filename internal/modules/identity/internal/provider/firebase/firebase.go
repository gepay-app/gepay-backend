// Package firebase mengimplementasikan domain.IdentityProvider memakai
// Firebase Admin SDK. Tugasnya HANYA memverifikasi ID token dan menerjemahkannya
// menjadi domain.ExternalIdentity — tanpa role/status/DB (itu urusan service).
package firebase

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"gepay/internal/modules/identity/api/domain"
	"gepay/platform/apperror"
)

type Provider struct {
	client *auth.Client
}

// New membangun Firebase Auth client dari service account JSON yang di-encode
// base64 (satu baris, aman untuk env/secret manager).
//
// Error dikembalikan — bukan di-panic — supaya bootstrap yang memutuskan
// fail-fast saat startup.
func New(ctx context.Context, encodedCredentials string) (*Provider, error) {
	// TrimSpace penting: nilai dari .env/secret manager sering ikut newline.
	credentialsJSON, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedCredentials))
	if err != nil {
		return nil, fmt.Errorf("firebase: decode base64 credentials: %w", err)
	}

	app, err := firebase.NewApp(ctx, nil,
		option.WithAuthCredentialsJSON(option.ServiceAccount, credentialsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("firebase: init app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: init auth client: %w", err)
	}

	return &Provider{client: client}, nil
}

// VerifyToken memvalidasi signature + expiry ID token dan mengubahnya menjadi
// ExternalIdentity.
//
// Semua kegagalan verifikasi dipetakan ke 401 dengan pesan generik supaya
// penyerang tidak bisa membedakan "token kedaluwarsa" vs "signature salah";
// cause aslinya tetap dibawa untuk log.
func (p *Provider) VerifyToken(ctx context.Context, rawToken string) (*domain.ExternalIdentity, error) {
	token, err := p.client.VerifyIDToken(ctx, rawToken)
	if err != nil {
		return nil, apperror.Unauthorized("invalid or expired token").WithCause(err)
	}

	email, _ := token.Claims["email"].(string)
	emailVerified, _ := token.Claims["email_verified"].(bool)

	return &domain.ExternalIdentity{
		ProviderUID:   token.UID,
		Email:         email,
		EmailVerified: emailVerified,
	}, nil
}
