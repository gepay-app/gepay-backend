package domain

import (
	"context"
	"uuid"
)

type Service interface {
	ResolveAuthContext(ctx context.Context, rawToken string) (*AuthContext, error)
	PromoteToAdmin(ctx context.Context, actorID, targetUserID uuid.UUID) error
	UpdateKYCStatus(ctx context.Context, actorID, targetUserID uuid.UUID, status KYCStatus) error
}
