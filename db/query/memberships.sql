-- name: UpsertOrgMembership :one
INSERT INTO org_memberships (org_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (org_id, user_id)
DO UPDATE SET role = EXCLUDED.role
RETURNING org_id, user_id, role, created_at;

-- name: GetOrgMembership :one
SELECT org_id, user_id, role, created_at
FROM org_memberships
WHERE org_id = $1 AND user_id = $2;

-- name: ListOrgMembershipsByOrg :many
SELECT org_id, user_id, role, created_at
FROM org_memberships
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: AddTeamMembership :one
INSERT INTO team_memberships (org_id, team_id, user_id)
VALUES ($1, $2, $3)
RETURNING org_id, team_id, user_id, created_at;

-- name: UpsertTeamMembership :one
WITH inserted AS (
    INSERT INTO team_memberships (org_id, team_id, user_id)
    VALUES ($1, $2, $3)
    ON CONFLICT (team_id, user_id) DO NOTHING
    RETURNING org_id, team_id, user_id, created_at
)
SELECT org_id, team_id, user_id, created_at
FROM inserted
UNION ALL
SELECT org_id, team_id, user_id, created_at
FROM team_memberships
WHERE org_id = $1 AND team_id = $2 AND user_id = $3
LIMIT 1;

-- name: ListTeamMemberships :many
SELECT org_id, team_id, user_id, created_at
FROM team_memberships
WHERE org_id = $1 AND team_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: AddTeamAdminScope :one
INSERT INTO team_admin_scopes (org_id, team_id, admin_user_id)
VALUES ($1, $2, $3)
RETURNING org_id, team_id, admin_user_id, created_at;

-- name: UpsertTeamAdminScope :one
WITH inserted AS (
    INSERT INTO team_admin_scopes (org_id, team_id, admin_user_id)
    VALUES ($1, $2, $3)
    ON CONFLICT (team_id, admin_user_id) DO NOTHING
    RETURNING org_id, team_id, admin_user_id, created_at
)
SELECT org_id, team_id, admin_user_id, created_at
FROM inserted
UNION ALL
SELECT org_id, team_id, admin_user_id, created_at
FROM team_admin_scopes
WHERE org_id = $1 AND team_id = $2 AND admin_user_id = $3
LIMIT 1;

-- name: HasTeamAdminScope :one
SELECT EXISTS (
    SELECT 1
    FROM team_admin_scopes
    WHERE org_id = $1 AND team_id = $2 AND admin_user_id = $3
);

-- name: ListTeamAdminScopes :many
SELECT org_id, team_id, admin_user_id, created_at
FROM team_admin_scopes
WHERE org_id = $1 AND admin_user_id = $2
ORDER BY created_at DESC
LIMIT $3;
