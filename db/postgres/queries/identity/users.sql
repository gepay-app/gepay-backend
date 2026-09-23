-- name: GetUserByAuthProviderID :one
SELECT * FROM identity.users WHERE auth_provider = $1 AND auth_provider_id = $2;

-- name: GetUserByID :one
SELECT * FROM identity.users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO identity.users (id, auth_provider, auth_provider_id, email, role, status, kyc_status,first_name,last_name,nickname)
VALUES ($1, $2, $3, $4, $5, $6, $7,$8,$9,$10)
    RETURNING *;

-- name: UpdateUserRole :one
UPDATE identity.users SET role = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateUserKYCStatus :one
UPDATE identity.users SET kyc_status = $2, updated_at = now() WHERE id = $1 RETURNING *;