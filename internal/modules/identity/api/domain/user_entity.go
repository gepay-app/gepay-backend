package domain

import (
	"context"
	"uuid"
)

type Role string

const (
	RoleAnonymous  Role = "ANONYMOUS"
	RoleUser       Role = "USER"
	RoleAdmin      Role = "ADMIN"
	RoleSuperAdmin Role = "SUPER_ADMIN"
)

type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusSuspended Status = "SUSPENDED"
)

type KYCStatus string

const (
	KYCUnverified KYCStatus = "UNVERIFIED"
	KYCPending    KYCStatus = "PENDING"
	KYCVerified   KYCStatus = "VERIFIED"
	KYCRejected   KYCStatus = "REJECTED"
)

const ProviderFirebase = "FIREBASE"

type User struct {
	ID             uuid.UUID
	AuthProvider   string
	AuthProviderID string
	Email          string
	Role           Role
	Status         Status
	KYCStatus      KYCStatus
	FirstName      string
	LastName       string
	Nickname       string
}

// AuthContext = state otorisasi yang dicache di Redis, dipakai di semua layer atas.
// Anonymous request punya AuthContext{Role: RoleAnonymous}, UserID kosong.
type AuthContext struct {
	UserID    uuid.UUID `json:"user_id"`
	Role      Role      `json:"role"`
	Status    Status    `json:"status"`
	KYCStatus KYCStatus `json:"kyc_status"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Nickname  string    `json:"nickname"`
}

func (a AuthContext) IsAnonymous() bool { return a.Role == RoleAnonymous }

// caching
type AuthContextCache interface {
	GetByProviderID(ctx context.Context, id string) (*AuthContext, error) // nil, nil = cache miss
	SetByProviderID(ctx context.Context, id string, u *AuthContext) error
	EvictByProviderID(ctx context.Context, id string) error
}
