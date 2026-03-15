-- +goose Up
CREATE TABLE IF NOT EXISTS auth_logins (
    id TEXT PRIMARY KEY,
    magic_link_id TEXT NOT NULL UNIQUE REFERENCES auth_magic_links(id) ON DELETE CASCADE,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('org_owner', 'team_admin', 'member')),
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_logins_email_created_at
    ON auth_logins (email, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_auth_logins_email_created_at;
DROP TABLE IF EXISTS auth_logins;
