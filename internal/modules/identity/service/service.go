package service

import (
	"context"
	"errors"
	"fmt"
	"gepay/internal/modules/identity/domain"
	"gepay/internal/modules/identity/repository"
	"gepay/platform/apperror"
	"gepay/platform/logger"
	"uuid"

	"github.com/jackc/pgx/v5"
)

type service struct {
	provider  domain.IdentityProvider
	q         *repository.Queries
	authCache domain.AuthContextCache
}

func New(provider domain.IdentityProvider, q *repository.Queries, authCache domain.AuthContextCache) domain.Service {
	return &service{
		provider:  provider,
		q:         q,
		authCache: authCache,
	}
}

func (s *service) ResolveAuthContext(ctx context.Context, rawToken string) (*domain.AuthContext, error) {
	log := logger.FromContext(ctx)
	ext, err := s.provider.VerifyToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if cached, err := s.authCache.GetByProviderID(ctx, ext.ProviderUID); err == nil {
		return cached, nil
	}

	user, err := s.q.GetUserByAuthProviderID(ctx, repository.GetUserByAuthProviderIDParams{
		AuthProvider:   domain.ProviderFirebase,
		AuthProviderID: ext.ProviderUID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		user, err = s.createUserFirstLogin(ctx, *ext)
		if err != nil {
			return nil, err
		}
	case err != nil:
		return nil, apperror.Unauthorized("unauthorized")
	}
	authCtx := &domain.AuthContext{
		UserID:    user.ID,
		Role:      domain.Role(user.Role),
		Status:    domain.Status(user.Status),
		KYCStatus: domain.KYCStatus(user.KycStatus),
	}

	if err := s.authCache.SetByProviderID(ctx, ext.ProviderUID, authCtx); err != nil {
		log.Error("failed to cache identity auth context", err)
	}

	return authCtx, nil
}

func (s *service) PromoteToAdmin(ctx context.Context, actorID, targetUserID uuid.UUID) error {
	log := logger.FromContext(ctx)
	actor, err := s.q.GetUserByID(ctx, actorID)
	if err != nil {
		log.Warn("failed to get user by id for promote admin", err)
		return apperror.NotFound("actor not found")
	}
	if domain.Role(actor.Role) != domain.RoleSuperAdmin {
		return apperror.Forbidden("insufficient privileges")
	}

	targetUser, err := s.q.UpdateUserRole(ctx, repository.UpdateUserRoleParams{
		ID:   targetUserID,
		Role: string(domain.RoleAdmin),
	})
	if err != nil {
		return apperror.Internal("failed to update user")
	}

	if err := s.authCache.EvictByProviderID(ctx, targetUser.AuthProviderID); err != nil {
		log.Warn("failed to evict user", err)
	}

	return nil
}

func (s *service) UpdateKYCStatus(ctx context.Context, actorID, targetUserID uuid.UUID, status domain.KYCStatus) error {
	log := logger.FromContext(ctx)
	actor, err := s.q.GetUserByID(ctx, actorID)
	if err != nil {
		log.Warn("failed to get user by id for promote admin", err)
		return apperror.NotFound("actor not found")
	}

	if domain.Role(actor.Role) != domain.RoleAdmin && domain.Role(actor.Role) != domain.RoleSuperAdmin {
		return apperror.Forbidden("insufficient privileges")
	}

	targetUser, err := s.q.UpdateUserKYCStatus(ctx, repository.UpdateUserKYCStatusParams{
		ID:        targetUserID,
		KycStatus: string(status),
	})
	if err != nil {
		return apperror.Internal("failed to update user")
	}

	if err := s.authCache.EvictByProviderID(ctx, targetUser.AuthProviderID); err != nil {
		log.Warn("failed to evict user", err)
	}

	return nil
}

// private
func (s *service) createUserFirstLogin(ctx context.Context, ext domain.ExternalIdentity) (repository.IdentityUser, error) {

	user, err := s.q.CreateUser(ctx, repository.CreateUserParams{
		ID:             uuid.NewV7(),
		AuthProvider:   domain.ProviderFirebase,
		AuthProviderID: ext.ProviderUID,
		Email:          ext.Email,
		Role:           string(domain.RoleUser),
		Status:         string(domain.StatusActive),
		KycStatus:      string(domain.KYCUnverified),
	})
	if err != nil {
		return repository.IdentityUser{}, apperror.Internal(fmt.Sprintf("failed to create user identity:%s", err.Error()))
	}
	return user, nil
}
