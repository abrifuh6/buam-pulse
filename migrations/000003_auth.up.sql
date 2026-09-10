-- User email verification. Existing users are grandfathered in as verified so
-- this migration doesn't lock anyone out.
ALTER TABLE users ADD COLUMN verified_at TIMESTAMPTZ;
UPDATE users SET verified_at = now();

-- Single-use tokens for verification and password reset. Storing a HASH, not
-- the token itself: a database leak then doesn't hand an attacker working
-- reset links, the same reasoning as password hashing.
CREATE TABLE auth_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('verify','reset')),
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_tokens_user_idx ON auth_tokens (user_id, kind)
    WHERE used_at IS NULL;
