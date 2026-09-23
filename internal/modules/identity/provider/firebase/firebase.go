package firebase

import (
	"context"
	"encoding/base64"
	"fmt"
	"gepay/internal/modules/identity/domain"
	"gepay/platform/apperror"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Provider struct {
	client *auth.Client
}

func New(ctx context.Context, encodedCredentials string) (*Provider, error) {
	credentialsJSON, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return nil, fmt.Errorf("firebase: decode credentials: %w", err)
	}
	app, err := firebase.NewApp(ctx, nil, option.WithAuthCredentialsJSON(option.ServiceAccount, credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("firebase: init app: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: init auth client: %w", err)
	}
	return &Provider{client: client}, nil
}

func (p *Provider) VerifyToken(ctx context.Context, rawToken string) (*domain.ExternalIdentity, error) {
	token, err := p.client.VerifyIDToken(ctx, rawToken)
	if err != nil {
		return nil, apperror.Unauthorized("invalid or expired token")
	}

	emial, _ := token.Claims["email"].(string)
	emailVerified, _ := token.Claims["email_verified"].(bool)

	return &domain.ExternalIdentity{
		Email:         emial,
		EmailVerified: emailVerified,
		ProviderUID:   token.UID,
	}, err
}
