-- Pending invitations. A row exists only until it is accepted or revoked.
-- Like auth tokens, we store a hash: a database leak must not hand an
-- attacker working invite links into someone's tenant.
CREATE TABLE invitations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    role       TEXT NOT NULL CHECK (role IN ('admin','member')),
    token_hash TEXT NOT NULL UNIQUE,
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One outstanding invite per address per tenant. Re-inviting replaces
    -- rather than accumulates.
    UNIQUE (tenant_id, email)
);

CREATE INDEX invitations_pending_idx ON invitations (tenant_id)
    WHERE accepted_at IS NULL;

-- Every tenant must keep exactly one owner. Enforced in application logic on
-- role change and member removal; this index makes the invariant checkable.
CREATE INDEX users_owner_idx ON users (tenant_id) WHERE role = 'owner';
