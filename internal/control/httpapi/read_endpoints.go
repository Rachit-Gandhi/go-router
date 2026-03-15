package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
	"github.com/Rachit-Gandhi/go-router/internal/auth"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
	maxUsageWindow   = 90 * 24 * time.Hour
)

func (h *controlHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}

	membership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: claims.OrgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "session user is not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve org membership")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":     claims.OrgID,
		"user_id":    claims.UserID,
		"role":       membership.Role,
		"expires_at": time.Unix(claims.ExpiresAtUnix, 0).UTC().Format(time.RFC3339),
	})
}

func (h *controlHandler) handleGetOrgSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}

	const summarySQL = `
SELECT
	(SELECT COUNT(*) FROM teams WHERE org_id = $1) AS teams,
	(SELECT COUNT(*) FROM org_memberships WHERE org_id = $1) AS members,
	(SELECT COUNT(*) FROM org_memberships WHERE org_id = $1 AND role IN ('org_owner', 'team_admin')) AS admins,
	(SELECT COUNT(*) FROM user_team_api_keys WHERE org_id = $1 AND is_active = TRUE AND revoked_at IS NULL) AS active_keys,
	(SELECT COUNT(*) FROM org_provider_keys WHERE org_id = $1 AND is_active = TRUE) AS provider_keys,
	(
		(SELECT COUNT(*) FROM org_model_policies WHERE org_id = $1) +
		(SELECT COUNT(*) FROM team_model_policies WHERE org_id = $1)
	) AS policy_entries;
`

	var teams, members, admins, activeKeys, providerKeys, policyEntries int64
	if err := h.db.QueryRowContext(ctx, summarySQL, orgID).Scan(&teams, &members, &admins, &activeKeys, &providerKeys, &policyEntries); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load org summary")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":         orgID,
		"teams":          teams,
		"members":        members,
		"admins":         admins,
		"active_keys":    activeKeys,
		"provider_keys":  providerKeys,
		"policy_entries": policyEntries,
	})
}

func (h *controlHandler) handleListTeams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}

	offset, limit, ok := parseOffsetLimit(w, r, defaultPageLimit, maxPageLimit)
	if !ok {
		return
	}

	const listTeamsSQL = `
SELECT id, org_id, name, profile_jsonb, rate_limit_per_minute, created_at, updated_at
FROM teams
WHERE org_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;
`

	rows, err := h.db.QueryContext(ctx, listTeamsSQL, orgID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, rowOrgID, name string
		var profile []byte
		var rateLimit sql.NullInt32
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &rowOrgID, &name, &profile, &rateLimit, &createdAt, &updatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse teams")
			return
		}
		if len(profile) == 0 {
			profile = []byte(`{}`)
		}
		var profilePayload any
		if err := json.Unmarshal(profile, &profilePayload); err != nil {
			profilePayload = map[string]any{}
		}

		item := map[string]any{
			"id":            id,
			"org_id":        rowOrgID,
			"name":          name,
			"profile_jsonb": profilePayload,
			"created_at":    createdAt.UTC().Format(time.RFC3339),
			"updated_at":    updatedAt.UTC().Format(time.RFC3339),
		}
		if rateLimit.Valid {
			item["rate_limit_per_minute"] = rateLimit.Int32
		} else {
			item["rate_limit_per_minute"] = nil
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"limit":       limit,
		"next_cursor": nextCursor(offset, limit, len(items)),
	})
}

func (h *controlHandler) handleListTeamMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	claims, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if _, err := h.queries.GetTeamByID(ctx, dbquery.GetTeamByIDParams{ID: teamID, OrgID: orgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve team")
		return
	}

	allowed, err := h.canManageTeamScopedResource(ctx, orgID, teamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for team member visibility")
		return
	}

	offset, limit, ok := parseOffsetLimit(w, r, defaultPageLimit, maxPageLimit)
	if !ok {
		return
	}

	const listMembersSQL = `
SELECT
	tm.user_id,
	u.email,
	u.name,
	om.role AS org_role,
	CASE
		WHEN om.role = 'org_owner' THEN 'org_owner'
		WHEN tas.admin_user_id IS NOT NULL THEN 'team_admin'
		ELSE 'member'
	END AS team_role,
	(om.role = 'org_owner' OR tas.admin_user_id IS NOT NULL) AS has_scope,
	tm.created_at
FROM team_memberships tm
JOIN users u
	ON u.id = tm.user_id
JOIN org_memberships om
	ON om.org_id = tm.org_id AND om.user_id = tm.user_id
LEFT JOIN team_admin_scopes tas
	ON tas.org_id = tm.org_id AND tas.team_id = tm.team_id AND tas.admin_user_id = tm.user_id
WHERE tm.org_id = $1 AND tm.team_id = $2
ORDER BY tm.created_at DESC, tm.user_id ASC
LIMIT $3 OFFSET $4;
`

	rows, err := h.db.QueryContext(ctx, listMembersSQL, orgID, teamID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list team members")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var userID, email, name, orgRole, teamRole string
		var hasScope bool
		var createdAt time.Time
		if err := rows.Scan(&userID, &email, &name, &orgRole, &teamRole, &hasScope, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse team members")
			return
		}
		items = append(items, map[string]any{
			"user_id":    userID,
			"email":      email,
			"name":       name,
			"org_role":   orgRole,
			"team_role":  teamRole,
			"has_scope":  hasScope,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list team members")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"limit":       limit,
		"next_cursor": nextCursor(offset, limit, len(items)),
	})
}

func (h *controlHandler) handleListTeamAdmins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	claims, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if _, err := h.queries.GetTeamByID(ctx, dbquery.GetTeamByIDParams{ID: teamID, OrgID: orgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve team")
		return
	}

	allowed, err := h.canManageTeamScopedResource(ctx, orgID, teamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for team admin visibility")
		return
	}

	offset, limit, ok := parseOffsetLimit(w, r, defaultPageLimit, maxPageLimit)
	if !ok {
		return
	}

	const listAdminsSQL = `
WITH admins AS (
	SELECT om.user_id, u.email, u.name, om.role, om.created_at
	FROM org_memberships om
	JOIN users u
		ON u.id = om.user_id
	WHERE om.org_id = $1 AND om.role = 'org_owner'

	UNION ALL

	SELECT tas.admin_user_id, u.email, u.name, om.role, tas.created_at
	FROM team_admin_scopes tas
	JOIN users u
		ON u.id = tas.admin_user_id
	JOIN org_memberships om
		ON om.org_id = tas.org_id AND om.user_id = tas.admin_user_id
	WHERE tas.org_id = $1 AND tas.team_id = $2
),
deduped AS (
	SELECT DISTINCT ON (user_id) user_id, email, name, role, created_at
	FROM admins
	ORDER BY user_id, created_at DESC
)
SELECT user_id, email, name, role, created_at
FROM deduped
ORDER BY created_at DESC, user_id ASC
LIMIT $3 OFFSET $4;
`

	rows, err := h.db.QueryContext(ctx, listAdminsSQL, orgID, teamID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list team admins")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var userID, email, name, orgRole string
		var createdAt time.Time
		if err := rows.Scan(&userID, &email, &name, &orgRole, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse team admins")
			return
		}
		items = append(items, map[string]any{
			"user_id":    userID,
			"email":      email,
			"name":       name,
			"org_role":   orgRole,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list team admins")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"limit":       limit,
		"next_cursor": nextCursor(offset, limit, len(items)),
	})
}

func (h *controlHandler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can list api keys")
		return
	}

	teamID := strings.TrimSpace(r.URL.Query().Get("team_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	includeRevoked := false
	if raw := strings.TrimSpace(r.URL.Query().Get("include_revoked")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "include_revoked must be a boolean")
			return
		}
		includeRevoked = parsed
	}

	offset, limit, ok := parseOffsetLimit(w, r, defaultPageLimit, maxPageLimit)
	if !ok {
		return
	}

	const listAPIKeysSQL = `
SELECT id, key_prefix, user_id, team_id, is_active, last_used_at, created_at
FROM user_team_api_keys
WHERE org_id = $1
  AND ($2 = '' OR team_id = $2)
  AND ($3 = '' OR user_id = $3)
  AND ($4 OR revoked_at IS NULL)
ORDER BY created_at DESC, id DESC
LIMIT $5 OFFSET $6;
`

	rows, err := h.db.QueryContext(ctx, listAPIKeysSQL, orgID, teamID, userID, includeRevoked, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, keyPrefix, keyUserID, keyTeamID string
		var isActive bool
		var lastUsedAt sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(&id, &keyPrefix, &keyUserID, &keyTeamID, &isActive, &lastUsedAt, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse api keys")
			return
		}

		item := map[string]any{
			"id":         id,
			"key_prefix": keyPrefix,
			"user_id":    keyUserID,
			"team_id":    keyTeamID,
			"is_active":  isActive,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		}
		if lastUsedAt.Valid {
			item["last_used_at"] = lastUsedAt.Time.UTC().Format(time.RFC3339)
		} else {
			item["last_used_at"] = nil
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"limit":       limit,
		"next_cursor": nextCursor(offset, limit, len(items)),
	})
}

func (h *controlHandler) handleListProviderKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can list provider keys")
		return
	}

	const listProviderKeysSQL = `
SELECT id, provider, key_kek_id, is_active, created_at, updated_at
FROM org_provider_keys
WHERE org_id = $1
ORDER BY created_at DESC, id DESC;
`

	rows, err := h.db.QueryContext(ctx, listProviderKeysSQL, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list provider keys")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, provider, keyKekID string
		var isActive bool
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &provider, &keyKekID, &isActive, &createdAt, &updatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse provider keys")
			return
		}
		items = append(items, map[string]any{
			"id":         id,
			"provider":   provider,
			"key_kek_id": keyKekID,
			"is_active":  isActive,
			"created_at": createdAt.UTC().Format(time.RFC3339),
			"updated_at": updatedAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list provider keys")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *controlHandler) handleListOrgModelPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}

	const listOrgPoliciesSQL = `
SELECT provider, model, is_allowed, created_at
FROM org_model_policies
WHERE org_id = $1
ORDER BY provider, model;
`

	rows, err := h.db.QueryContext(ctx, listOrgPoliciesSQL, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list org policies")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var provider, model string
		var isAllowed bool
		var createdAt time.Time
		if err := rows.Scan(&provider, &model, &isAllowed, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse org policies")
			return
		}
		items = append(items, map[string]any{
			"provider":   provider,
			"model":      model,
			"is_allowed": isAllowed,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list org policies")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *controlHandler) handleListTeamModelPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	claims, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if _, err := h.queries.GetTeamByID(ctx, dbquery.GetTeamByIDParams{ID: teamID, OrgID: orgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve team")
		return
	}
	allowed, err := h.canManageTeamScopedResource(ctx, orgID, teamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for team policy visibility")
		return
	}

	rows, err := h.queries.ListTeamModelPolicies(ctx, dbquery.ListTeamModelPoliciesParams{OrgID: orgID, TeamID: teamID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list team policies")
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"provider":   row.Provider,
			"model":      row.Model,
			"is_allowed": row.IsAllowed,
			"created_at": row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *controlHandler) handleListEffectiveModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	claims, _, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if _, err := h.queries.GetTeamByID(ctx, dbquery.GetTeamByIDParams{ID: teamID, OrgID: orgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve team")
		return
	}
	allowed, err := h.canManageTeamScopedResource(ctx, orgID, teamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for effective model visibility")
		return
	}

	offset, limit, ok := parseOffsetLimit(w, r, defaultPageLimit, maxPageLimit)
	if !ok {
		return
	}

	rows, err := h.queries.ListEffectiveAllowedModels(ctx, dbquery.ListEffectiveAllowedModelsParams{
		OrgID:      orgID,
		TeamID:     teamID,
		OffsetRows: sql.NullInt32{Int32: int32(offset), Valid: true},
		LimitRows:  int32(limit),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list effective models")
		return
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"provider": row.Provider,
			"model":    row.Model,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"limit":       limit,
		"next_cursor": nextCursor(offset, limit, len(items)),
	})
}

func (h *controlHandler) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can view usage summary")
		return
	}

	from, to, ok := parseTimeRange(w, r, h.now())
	if !ok {
		return
	}

	const usageSummarySQL = `
SELECT
	COUNT(*)::BIGINT,
	COALESCE(SUM(request_tokens), 0)::BIGINT,
	COALESCE(SUM(response_tokens), 0)::BIGINT,
	COALESCE(AVG(CASE WHEN status_code >= 400 THEN 1.0 ELSE 0.0 END), 0.0),
	COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY latency_ms), 0.0),
	COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms), 0.0)
FROM usage_logs
WHERE org_id = $1
  AND created_at >= $2
  AND created_at < $3;
`

	var requestCount, requestTokens, responseTokens int64
	var errorRate, p50, p95 float64
	if err := h.db.QueryRowContext(ctx, usageSummarySQL, orgID, from, to).Scan(
		&requestCount,
		&requestTokens,
		&responseTokens,
		&errorRate,
		&p50,
		&p95,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage summary")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":            from.UTC().Format(time.RFC3339),
		"to":              to.UTC().Format(time.RFC3339),
		"request_count":   requestCount,
		"request_tokens":  requestTokens,
		"response_tokens": responseTokens,
		"error_rate":      errorRate,
		"latency_p50_ms":  p50,
		"latency_p95_ms":  p95,
	})
}

func (h *controlHandler) handleUsageTimeSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can view usage timeseries")
		return
	}

	from, to, ok := parseTimeRange(w, r, h.now())
	if !ok {
		return
	}

	bucket := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("bucket")))
	if bucket == "" {
		bucket = "hour"
	}
	if bucket != "hour" && bucket != "day" {
		writeError(w, http.StatusBadRequest, "bucket must be hour or day")
		return
	}

	query := fmt.Sprintf(`
SELECT
	date_trunc('%s', created_at) AS bucket_start,
	COUNT(*)::BIGINT,
	COALESCE(SUM(request_tokens), 0)::BIGINT,
	COALESCE(SUM(response_tokens), 0)::BIGINT,
	COALESCE(AVG(CASE WHEN status_code >= 400 THEN 1.0 ELSE 0.0 END), 0.0)
FROM usage_logs
WHERE org_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY 1
ORDER BY 1 ASC;
`, bucket)

	rows, err := h.db.QueryContext(ctx, query, orgID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage timeseries")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var bucketStart time.Time
		var requestCount, requestTokens, responseTokens int64
		var errorRate float64
		if err := rows.Scan(&bucketStart, &requestCount, &requestTokens, &responseTokens, &errorRate); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse usage timeseries")
			return
		}
		items = append(items, map[string]any{
			"bucket_start":    bucketStart.UTC().Format(time.RFC3339),
			"request_count":   requestCount,
			"request_tokens":  requestTokens,
			"response_tokens": responseTokens,
			"error_rate":      errorRate,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage timeseries")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":   from.UTC().Format(time.RFC3339),
		"to":     to.UTC().Format(time.RFC3339),
		"bucket": bucket,
		"items":  items,
	})
}

func (h *controlHandler) handleUsageByTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can view usage by team")
		return
	}

	from, to, ok := parseTimeRange(w, r, h.now())
	if !ok {
		return
	}

	const usageByTeamSQL = `
SELECT
	team_id,
	COUNT(*)::BIGINT,
	COALESCE(SUM(request_tokens), 0)::BIGINT,
	COALESCE(SUM(response_tokens), 0)::BIGINT,
	COALESCE(AVG(CASE WHEN status_code >= 400 THEN 1.0 ELSE 0.0 END), 0.0)
FROM usage_logs
WHERE org_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY team_id
ORDER BY COUNT(*) DESC, team_id ASC;
`

	rows, err := h.db.QueryContext(ctx, usageByTeamSQL, orgID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage by team")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var teamID string
		var requestCount, requestTokens, responseTokens int64
		var errorRate float64
		if err := rows.Scan(&teamID, &requestCount, &requestTokens, &responseTokens, &errorRate); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse usage by team")
			return
		}
		items = append(items, map[string]any{
			"team_id":         teamID,
			"request_count":   requestCount,
			"request_tokens":  requestTokens,
			"response_tokens": responseTokens,
			"error_rate":      errorRate,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage by team")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":  from.UTC().Format(time.RFC3339),
		"to":    to.UTC().Format(time.RFC3339),
		"items": items,
	})
}

func (h *controlHandler) handleUsageByModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can view usage by model")
		return
	}

	from, to, ok := parseTimeRange(w, r, h.now())
	if !ok {
		return
	}

	const usageByModelSQL = `
SELECT
	provider,
	model,
	COUNT(*)::BIGINT,
	COALESCE(SUM(request_tokens), 0)::BIGINT,
	COALESCE(SUM(response_tokens), 0)::BIGINT,
	COALESCE(AVG(CASE WHEN status_code >= 400 THEN 1.0 ELSE 0.0 END), 0.0)
FROM usage_logs
WHERE org_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY provider, model
ORDER BY COUNT(*) DESC, provider ASC, model ASC;
`

	rows, err := h.db.QueryContext(ctx, usageByModelSQL, orgID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage by model")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var provider, model string
		var requestCount, requestTokens, responseTokens int64
		var errorRate float64
		if err := rows.Scan(&provider, &model, &requestCount, &requestTokens, &responseTokens, &errorRate); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse usage by model")
			return
		}
		items = append(items, map[string]any{
			"provider":        provider,
			"model":           model,
			"request_count":   requestCount,
			"request_tokens":  requestTokens,
			"response_tokens": responseTokens,
			"error_rate":      errorRate,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load usage by model")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":  from.UTC().Format(time.RFC3339),
		"to":    to.UTC().Format(time.RFC3339),
		"items": items,
	})
}

func (h *controlHandler) handleListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	_, membership, ok := h.requireOrgContext(ctx, w, r, orgID)
	if !ok {
		return
	}
	if !isOrgOwner(membership) {
		writeError(w, http.StatusForbidden, "only org owner can view events")
		return
	}

	offset, limit, ok := parseOffsetLimit(w, r, defaultPageLimit, maxPageLimit)
	if !ok {
		return
	}

	const eventsSQL = `
WITH events AS (
	SELECT 'org_created'::TEXT AS event_type, o.id AS org_id, NULL::TEXT AS team_id, o.owner_user_id AS user_id, o.id AS resource_id, o.created_at AS occurred_at
	FROM orgs o
	WHERE o.id = $1

	UNION ALL
	SELECT 'team_created'::TEXT, t.org_id, t.id, NULL::TEXT, t.id, t.created_at
	FROM teams t
	WHERE t.org_id = $1

	UNION ALL
	SELECT 'team_member_added'::TEXT, tm.org_id, tm.team_id, tm.user_id, tm.team_id || ':' || tm.user_id, tm.created_at
	FROM team_memberships tm
	WHERE tm.org_id = $1

	UNION ALL
	SELECT 'team_admin_scoped'::TEXT, tas.org_id, tas.team_id, tas.admin_user_id, tas.team_id || ':' || tas.admin_user_id, tas.created_at
	FROM team_admin_scopes tas
	WHERE tas.org_id = $1

	UNION ALL
	SELECT 'api_key_created'::TEXT, k.org_id, k.team_id, k.user_id, k.id, k.created_at
	FROM user_team_api_keys k
	WHERE k.org_id = $1

	UNION ALL
	SELECT 'api_key_revoked'::TEXT, k.org_id, k.team_id, k.user_id, k.id, k.revoked_at
	FROM user_team_api_keys k
	WHERE k.org_id = $1 AND k.revoked_at IS NOT NULL

	UNION ALL
	SELECT 'provider_key_created'::TEXT, pk.org_id, NULL::TEXT, NULL::TEXT, pk.id, pk.created_at
	FROM org_provider_keys pk
	WHERE pk.org_id = $1

	UNION ALL
	SELECT 'org_policy_upserted'::TEXT, omp.org_id, NULL::TEXT, NULL::TEXT, omp.provider || ':' || omp.model, omp.created_at
	FROM org_model_policies omp
	WHERE omp.org_id = $1

	UNION ALL
	SELECT 'team_policy_upserted'::TEXT, tmp.org_id, tmp.team_id, NULL::TEXT, tmp.provider || ':' || tmp.model, tmp.created_at
	FROM team_model_policies tmp
	WHERE tmp.org_id = $1
)
SELECT event_type, org_id, team_id, user_id, resource_id, occurred_at
FROM events
ORDER BY occurred_at DESC, event_type DESC, resource_id DESC
LIMIT $2 OFFSET $3;
`

	rows, err := h.db.QueryContext(ctx, eventsSQL, orgID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load events")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var eventType, eventOrgID, resourceID string
		var teamID, userID sql.NullString
		var occurredAt time.Time
		if err := rows.Scan(&eventType, &eventOrgID, &teamID, &userID, &resourceID, &occurredAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse events")
			return
		}

		item := map[string]any{
			"id":          fmt.Sprintf("%s:%s:%d", eventType, resourceID, occurredAt.UnixNano()),
			"type":        eventType,
			"org_id":      eventOrgID,
			"resource_id": resourceID,
			"occurred_at": occurredAt.UTC().Format(time.RFC3339),
		}
		if teamID.Valid {
			item["team_id"] = teamID.String
		} else {
			item["team_id"] = nil
		}
		if userID.Valid {
			item["user_id"] = userID.String
		} else {
			item["user_id"] = nil
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"limit":       limit,
		"next_cursor": nextCursor(offset, limit, len(items)),
	})
}

func (h *controlHandler) requireOrgContext(ctx context.Context, w http.ResponseWriter, r *http.Request, orgID string) (auth.SessionClaims, dbquery.OrgMembership, bool) {
	var zeroClaims auth.SessionClaims
	var zeroMembership dbquery.OrgMembership

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return zeroClaims, zeroMembership, false
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return zeroClaims, zeroMembership, false
	}

	membership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return zeroClaims, zeroMembership, false
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve requester membership")
		return zeroClaims, zeroMembership, false
	}
	return claims, membership, true
}

func isOrgOwner(membership dbquery.OrgMembership) bool {
	return membership.Role == roleOrgOwner
}

func parseOffsetLimit(w http.ResponseWriter, r *http.Request, defaultLimit, maxLimit int) (int, int, bool) {
	limit := defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return 0, 0, false
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "cursor must be a non-negative integer")
			return 0, 0, false
		}
		offset = parsed
	}

	return offset, limit, true
}

func parseTimeRange(w http.ResponseWriter, r *http.Request, now time.Time) (time.Time, time.Time, bool) {
	to := now.UTC()

	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "to must be RFC3339")
			return time.Time{}, time.Time{}, false
		}
		to = parsed.UTC()
	}

	from := to.Add(-30 * 24 * time.Hour)

	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "from must be RFC3339")
			return time.Time{}, time.Time{}, false
		}
		from = parsed.UTC()
	}

	if !from.Before(to) {
		writeError(w, http.StatusBadRequest, "from must be before to")
		return time.Time{}, time.Time{}, false
	}
	if to.Sub(from) > maxUsageWindow {
		writeError(w, http.StatusBadRequest, "time range exceeds 90 days")
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func nextCursor(offset, limit, count int) string {
	if count < limit {
		return ""
	}
	return strconv.Itoa(offset + count)
}
