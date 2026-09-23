-- +goose Up
CREATE SCHEMA IF NOT EXISTS identity;
CREATE TABLE IF NOT EXISTS identity.users
(
    id               UUID PRIMARY KEY,
    auth_provider    VARCHAR(32)  NOT NULL,
    auth_provider_id VARCHAR(255) NOT NULL,
    email            VARCHAR(255) NOT NULL,
    role             VARCHAR(32)  NOT NULL DEFAULT 'USER',
    status           VARCHAR(32)  NOT NULL DEFAULT 'ACTIVE',
    kyc_status       VARCHAR(32)  NOT NULL DEFAULT 'UNVERIFIED',
    first_name       VARCHAR(50)  NOT NULL,
    last_name        VARCHAR(50)  NOT NULL,
    nickname         VARCHAR(50)  NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (auth_provider, auth_provider_id),
    UNIQUE (nickname)
);

CREATE INDEX idx_users_email ON identity.users (email);

-- cursor pagination index
CREATE INDEX idx_users_created_at_id
    ON identity.users (created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS identity.users;
DROP SCHEMA IF EXISTS identity;