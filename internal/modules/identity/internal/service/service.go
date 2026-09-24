package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"gepay/db/postgres"
	"gepay/internal/modules/identity/api/domain"
	"gepay/internal/modules/identity/internal/repository"
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

// ResolveAuthContext memverifikasi token lalu mengembalikan AuthContext
// (role/status/KYC) yang dipakai semua layer di atas.
//
// Alur: verify token → cek cache → kalau miss, ambil user (auto-provision saat
// first login) → isi cache → kembalikan.
func (s *service) ResolveAuthContext(ctx context.Context, rawToken string) (*domain.AuthContext, error) {
	ext, err := s.provider.VerifyToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	// Provider auth (Firebase/Auth0/Clerk) selalu membawa email, dan email itu
	// identitas utama user (NOT NULL + UNIQUE). Token tanpa email tidak bisa
	// dipakai auto-provision → tolak 401, jangan bikin baris tanpa identitas.
	ext.Email = normalizeEmail(ext.Email)
	if ext.Email == "" {
		return nil, apperror.Unauthorized("token does not contain an email")
	}

	// cached == nil berarti cache miss; error Redis sudah di-log dan sengaja
	// tidak menggagalkan request (cache hanya percepatan, bukan sumber kebenaran).
	if cached := s.cachedAuthContext(ctx, ext.ProviderUID); cached != nil {
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
		return nil, apperror.Internal("failed to resolve identity").WithCause(err)
	}

	authCtx := toAuthContext(user)

	if err := s.authCache.SetByProviderID(ctx, ext.ProviderUID, authCtx); err != nil {
		logger.FromContext(ctx).WarnContext(ctx, "failed to cache identity auth context", slog.Any("error", err))
	}

	return authCtx, nil
}

func (s *service) PromoteToAdmin(ctx context.Context, actorID, targetUserID uuid.UUID) error {
	actor, err := s.q.GetUserByID(ctx, actorID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperror.NotFound("actor not found")
	case err != nil:
		return apperror.Internal("failed to load actor").WithCause(err)
	}
	if domain.Role(actor.Role) != domain.RoleSuperAdmin {
		return apperror.Forbidden("insufficient privileges")
	}

	target, err := s.q.UpdateUserRole(ctx, repository.UpdateUserRoleParams{
		ID:   targetUserID,
		Role: string(domain.RoleAdmin),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperror.NotFound("user not found")
	case err != nil:
		return apperror.Internal("failed to update user role").WithCause(err)
	}

	s.evictAuthContext(ctx, target.AuthProviderID)
	return nil
}

func (s *service) UpdateKYCStatus(ctx context.Context, actorID, targetUserID uuid.UUID, status domain.KYCStatus) error {
	actor, err := s.q.GetUserByID(ctx, actorID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperror.NotFound("actor not found")
	case err != nil:
		return apperror.Internal("failed to load actor").WithCause(err)
	}

	if domain.Role(actor.Role) != domain.RoleAdmin && domain.Role(actor.Role) != domain.RoleSuperAdmin {
		return apperror.Forbidden("insufficient privileges")
	}

	target, err := s.q.UpdateUserKYCStatus(ctx, repository.UpdateUserKYCStatusParams{
		ID:        targetUserID,
		KycStatus: string(status),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperror.NotFound("user not found")
	case err != nil:
		return apperror.Internal("failed to update user kyc status").WithCause(err)
	}

	s.evictAuthContext(ctx, target.AuthProviderID)
	return nil
}

// cachedAuthContext mengembalikan nil saat cache miss ATAU saat Redis
// bermasalah — pemanggil selalu lanjut ke DB. Error Redis di-log sebagai Warn
// karena cache bersifat best-effort (data basi akan kedaluwarsa lewat TTL).
func (s *service) cachedAuthContext(ctx context.Context, providerID string) *domain.AuthContext {
	cached, err := s.authCache.GetByProviderID(ctx, providerID)
	if err != nil {
		logger.FromContext(ctx).WarnContext(ctx, "failed to read identity cache", slog.Any("error", err))
		return nil
	}
	return cached
}

// evictAuthContext membuang cache setelah role/KYC berubah. Best-effort: kalau
// gagal, TTL 15 menit tetap membuat data basi hilang dengan sendirinya.
func (s *service) evictAuthContext(ctx context.Context, providerID string) {
	if err := s.authCache.EvictByProviderID(ctx, providerID); err != nil {
		logger.FromContext(ctx).WarnContext(ctx, "failed to evict identity cache", slog.Any("error", err))
	}
}

// UpdateProfile memperbarui profil user (first_name, last_name, nickname).
// Semua field wajib diisi (non-empty), validasi max 50 sudah dilakukan di handler.
func (s *service) UpdateProfile(ctx context.Context, userID uuid.UUID, firstName, lastName, nickname string) error {
	// Ambil user dulu untuk dapatkan AuthProviderID (untuk evict cache).
	user, err := s.q.GetUserByID(ctx, userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperror.NotFound("user not found")
	case err != nil:
		return apperror.Internal("failed to load user").WithCause(err)
	}

	_, err = s.q.UpdateUserProfile(ctx, repository.UpdateUserProfileParams{
		ID:        userID,
		FirstName: &firstName,
		LastName:  &lastName,
		Nickname:  &nickname,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperror.NotFound("user not found")
	case err != nil:
		return apperror.Internal("failed to update user profile").WithCause(err)
	}

	s.evictAuthContext(ctx, user.AuthProviderID)
	return nil
}

// createUserFirstLogin membuat baris user saat pertama kali token-nya valid.
// Profil (nama/nickname) sengaja dibiarkan NULL — dilengkapi lewat endpoint
// profile-completion.
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
	if err == nil {
		return user, nil
	}

	// Unique violation di sini bisa berasal dari dua constraint:
	//  1. (auth_provider, auth_provider_id) — dua request pertama user yang sama
	//     balapan; row sebenarnya sudah dibuat pemenangnya.
	//  2. email — email sudah terdaftar pada provider/UID lain.
	if postgres.IsUniqueViolation(err) {
		existing, getErr := s.q.GetUserByAuthProviderID(ctx, repository.GetUserByAuthProviderIDParams{
			AuthProvider:   domain.ProviderFirebase,
			AuthProviderID: ext.ProviderUID,
		})
		switch {
		case getErr == nil:
			return existing, nil
		case !errors.Is(getErr, pgx.ErrNoRows):
			return repository.IdentityUser{}, apperror.Internal("failed to resolve identity").WithCause(getErr)
		}

		return repository.IdentityUser{}, apperror.Conflict("email is already registered")
	}

	return repository.IdentityUser{}, apperror.Internal("failed to create user identity").WithCause(err)
}

// normalizeEmail menyamakan bentuk email sebelum disimpan: trim + lowercase.
// Dengan begitu UNIQUE(email) di DB efektif case-insensitive.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// toAuthContext memetakan row user ke state otorisasi yang dicache di Redis.
// Kolom profil nullable → string kosong supaya entity domain tetap sederhana.
func toAuthContext(u repository.IdentityUser) *domain.AuthContext {
	return &domain.AuthContext{
		UserID:    u.ID,
		Role:      domain.Role(u.Role),
		Status:    domain.Status(u.Status),
		KYCStatus: domain.KYCStatus(u.KycStatus),
		FirstName: derefString(u.FirstName),
		LastName:  derefString(u.LastName),
		Nickname:  derefString(u.Nickname),
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
