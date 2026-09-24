-- name: GetUserByAuthProviderID :one
SELECT * FROM identity.users WHERE auth_provider = $1 AND auth_provider_id = $2;

-- name: GetUserByID :one
SELECT * FROM identity.users WHERE id = $1;

-- name: CreateUser :one
-- first_name/last_name/nickname sengaja TIDAK diisi: Firebase first login hanya
-- membawa email + UID, profil dilengkapi lewat endpoint profile-completion.
INSERT INTO identity.users (id, auth_provider, auth_provider_id, email, role, status, kyc_status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING *;

-- name: UpdateUserRole :one
UPDATE identity.users SET role = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateUserKYCStatus :one
UPDATE identity.users SET kyc_status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateUserProfile :one
UPDATE identity.users 
SET first_name = $2, last_name = $3, nickname = $4, updated_at = now() 
WHERE id = $1 RETURNING *;