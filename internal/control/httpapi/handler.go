package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
	"github.com/Rachit-Gandhi/go-router/internal/auth"
	internalcrypto "github.com/Rachit-Gandhi/go-router/internal/crypto"
	"github.com/Rachit-Gandhi/go-router/internal/httputil"
	"github.com/Rachit-Gandhi/go-router/internal/store"
)

const (
	roleOrgOwner  = "org_owner"
	roleTeamAdmin = "team_admin"
	roleMember    = "member"

	sessionCookieName = "control_session"
	magicLinkTTL      = 10 * time.Minute
	sessionTTL        = 15 * time.Minute
	refreshTTL        = 24 * time.Hour
)

type controlHandler struct {
	db       *sql.DB
	queries  *dbquery.Queries
	sessions *auth.SessionCodec
	now      func() time.Time
}

// NewHandler builds the control-plane HTTP router using Postgres config from env.
func NewHandler() http.Handler {
	h, _, err := NewHandlerWithPostgresFromEnv(time.Now)
	if err != nil {
		panic(err)
	}
	return h
}

// NewHandlerWithPostgresFromEnv creates a postgres-backed handler and returns the opened DB.
func NewHandlerWithPostgresFromEnv(now func() time.Time) (http.Handler, *sql.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_DB_DSN"))
	if dsn == "" {
		return nil, nil, errors.New("CONTROL_DB_DSN is required")
	}

	db, err := store.OpenPostgres(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}

	secret := os.Getenv("CONTROL_SESSION_SECRET")
	if secret == "" {
		secret = "dev-control-session-secret"
	}
	codec, err := auth.NewSessionCodec(secret)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("new session codec: %w", err)
	}

	return NewHandlerWithDB(db, codec, now), db, nil
}

// NewHandlerWithDB builds the control-plane router with explicit DB/session dependencies.
func NewHandlerWithDB(db *sql.DB, sessionCodec *auth.SessionCodec, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	if sessionCodec == nil {
		panic("session codec is required")
	}
	if db == nil {
		panic("db is required")
	}

	h := &controlHandler{
		db:       db,
		queries:  dbquery.New(db),
		sessions: sessionCodec,
		now:      now,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/control/healthz", httputil.HealthHandler())
	mux.HandleFunc("POST /v1/control/orgs", h.handleCreateOrg)
	mux.HandleFunc("POST /v1/control/auth/magic-link/request", h.handleMagicLinkRequest)
	mux.HandleFunc("POST /v1/control/auth/magic-link/exchange", h.handleMagicLinkExchange)
	mux.HandleFunc("POST /v1/control/auth/refresh", h.handleRefresh)
	mux.HandleFunc("POST /v1/control/auth/logout", h.handleLogout)
	mux.HandleFunc("POST /v1/control/orgs/{org_id}/teams", h.handleCreateTeam)
	mux.HandleFunc("POST /v1/control/orgs/{org_id}/teams/{team_id}/members", h.handleAddTeamMember)
	mux.HandleFunc("POST /v1/control/orgs/{org_id}/teams/{team_id}/admins/{user_id}", h.handleAddTeamAdminScope)
	mux.HandleFunc("POST /v1/control/orgs/{org_id}/teams/{team_id}/users/{user_id}/api-keys", h.handleCreateUserTeamAPIKey)
	mux.HandleFunc("POST /v1/control/orgs/{org_id}/api-keys/{key_id}/revoke", h.handleRevokeUserTeamAPIKey)
	mux.HandleFunc("POST /v1/control/orgs/{org_id}/providers/{provider}/keys", h.handleCreateOrgProviderKey)
	mux.HandleFunc("PUT /v1/control/orgs/{org_id}/policies/models", h.handleUpsertOrgModelPolicies)
	mux.HandleFunc("PUT /v1/control/orgs/{org_id}/teams/{team_id}/policies/models", h.handleUpsertTeamModelPolicies)

	return mux
}

func (h *controlHandler) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		OrgName    string `json:"org_name"`
		OwnerEmail string `json:"owner_email"`
		OwnerName  string `json:"owner_name"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.OrgName = strings.TrimSpace(req.OrgName)
	req.OwnerEmail = strings.ToLower(strings.TrimSpace(req.OwnerEmail))
	req.OwnerName = strings.TrimSpace(req.OwnerName)
	if req.OrgName == "" || req.OwnerEmail == "" {
		writeError(w, http.StatusBadRequest, "org_name and owner_email are required")
		return
	}
	if req.OwnerName == "" {
		req.OwnerName = "Owner"
	}

	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	ownerUserID, err := h.ensureUserByEmail(ctx, tx.Queries, req.OwnerEmail, req.OwnerName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve owner user")
		return
	}

	orgID, err := randomID("org")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate org ID")
		return
	}
	if _, err := tx.Queries.CreateOrg(ctx, dbquery.CreateOrgParams{
		ID:          orgID,
		Name:        req.OrgName,
		OwnerUserID: ownerUserID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create org")
		return
	}
	if _, err := tx.Queries.UpsertOrgMembership(ctx, dbquery.UpsertOrgMembershipParams{
		OrgID:  orgID,
		UserID: ownerUserID,
		Role:   roleOrgOwner,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create owner membership")
		return
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit org creation")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"org_id":        orgID,
		"owner_user_id": ownerUserID,
		"role":          roleOrgOwner,
	})
}

func (h *controlHandler) handleMagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		OrgID string `json:"org_id"`
		Email string `json:"email"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.OrgID = strings.TrimSpace(req.OrgID)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.OrgID == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "org_id and email are required")
		return
	}

	user, err := h.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to lookup user")
		return
	}
	if _, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: req.OrgID, UserID: user.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "user is not a member of the org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to verify org membership")
		return
	}

	now := h.now()
	magicLinkID, err := randomID("ml")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate magic link ID")
		return
	}
	code, err := randomCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate magic link code")
		return
	}

	if _, err := h.queries.CreateMagicLink(ctx, dbquery.CreateMagicLinkParams{
		ID:        magicLinkID,
		OrgID:     req.OrgID,
		Email:     req.Email,
		CodeHash:  hashValue(code),
		ExpiresAt: now.Add(magicLinkTTL),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create magic link")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"magic_link_id": magicLinkID,
		"code":          code,
		"expires_at":    now.Add(magicLinkTTL).UTC().Format(time.RFC3339),
	})
}

func (h *controlHandler) handleMagicLinkExchange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		MagicLinkID string `json:"magic_link_id"`
		Code        string `json:"code"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.MagicLinkID = strings.TrimSpace(req.MagicLinkID)
	req.Code = strings.TrimSpace(req.Code)
	if req.MagicLinkID == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "magic_link_id and code are required")
		return
	}

	now := h.now()
	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	link, err := tx.Queries.ConsumeMagicLinkByCode(ctx, dbquery.ConsumeMagicLinkByCodeParams{
		ID:       req.MagicLinkID,
		CodeHash: hashValue(req.Code),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid or expired magic link")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to consume magic link")
		return
	}

	user, err := tx.Queries.GetUserByEmail(ctx, link.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve user")
		return
	}
	if _, err := tx.Queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: link.OrgID, UserID: user.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "user is not a member of the org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to verify org membership")
		return
	}

	refreshID, err := randomID("rt")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create refresh token")
		return
	}
	sessionID, err := randomID("sess")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	tokenMaterial, err := randomID("rtk")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create refresh token")
		return
	}
	if _, err := tx.Queries.CreateRefreshToken(ctx, dbquery.CreateRefreshTokenParams{
		ID:         refreshID,
		OrgID:      link.OrgID,
		UserID:     user.ID,
		TokenHash:  hashValue(tokenMaterial),
		SessionID:  sessionID,
		DeviceInfo: sql.NullString{String: truncateDeviceInfo(r.UserAgent()), Valid: r.UserAgent() != ""},
		ExpiresAt:  now.Add(refreshTTL),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist refresh token")
		return
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finalize login")
		return
	}

	sessionEnds := now.Add(sessionTTL)
	claims := auth.SessionClaims{
		OrgID:          link.OrgID,
		UserID:         user.ID,
		RefreshTokenID: refreshID,
		ExpiresAtUnix:  sessionEnds.Unix(),
	}
	if err := h.setSessionCookie(w, claims, sessionEnds); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":     link.OrgID,
		"user_id":    user.ID,
		"expires_at": sessionEnds.UTC().Format(time.RFC3339),
	})
}

func (h *controlHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}

	now := h.now()
	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	rows, err := tx.Queries.RevokeRefreshToken(ctx, dbquery.RevokeRefreshTokenParams{ID: claims.RefreshTokenID, OrgID: claims.OrgID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	newRefreshID, err := randomID("rt")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}
	sessionID, err := randomID("sess")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}
	tokenMaterial, err := randomID("rtk")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}
	if _, err := tx.Queries.CreateRefreshToken(ctx, dbquery.CreateRefreshTokenParams{
		ID:         newRefreshID,
		OrgID:      claims.OrgID,
		UserID:     claims.UserID,
		TokenHash:  hashValue(tokenMaterial),
		SessionID:  sessionID,
		DeviceInfo: sql.NullString{String: truncateDeviceInfo(r.UserAgent()), Valid: r.UserAgent() != ""},
		ExpiresAt:  now.Add(refreshTTL),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist rotated refresh token")
		return
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finalize refresh")
		return
	}

	sessionEnds := now.Add(sessionTTL)
	if err := h.setSessionCookie(w, auth.SessionClaims{
		OrgID:          claims.OrgID,
		UserID:         claims.UserID,
		RefreshTokenID: newRefreshID,
		ExpiresAtUnix:  sessionEnds.Unix(),
	}, sessionEnds); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to refresh session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":     claims.OrgID,
		"user_id":    claims.UserID,
		"expires_at": sessionEnds.UTC().Format(time.RFC3339),
	})
}

func (h *controlHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}

	_, _ = h.queries.RevokeRefreshToken(ctx, dbquery.RevokeRefreshTokenParams{ID: claims.RefreshTokenID, OrgID: claims.OrgID})
	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (h *controlHandler) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	membership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve membership")
		return
	}
	if membership.Role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can create teams")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	teamID, err := randomID("team")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate team ID")
		return
	}

	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	team, err := tx.Queries.CreateTeam(ctx, dbquery.CreateTeamParams{
		ID:                 teamID,
		OrgID:              orgID,
		Name:               req.Name,
		Column4:            nil,
		RateLimitPerMinute: sql.NullInt32{},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create team")
		return
	}
	if _, err := tx.Queries.UpsertTeamMembership(ctx, dbquery.UpsertTeamMembershipParams{OrgID: orgID, TeamID: teamID, UserID: claims.UserID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to attach owner to team")
		return
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit team creation")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     team.ID,
		"org_id": team.OrgID,
		"name":   team.Name,
	})
}

func (h *controlHandler) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
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

	requesterMembership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve requester membership")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Name   string `json:"name"`
		Role   string `json:"role"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.Role = strings.TrimSpace(req.Role)
	if req.Role == "" {
		req.Role = roleMember
	}
	if req.Role != roleMember && req.Role != roleTeamAdmin {
		writeError(w, http.StatusBadRequest, "role must be member or team_admin")
		return
	}

	isOwner := requesterMembership.Role == roleOrgOwner
	if !isOwner {
		if requesterMembership.Role != roleTeamAdmin {
			writeError(w, http.StatusForbidden, "insufficient permissions for team membership changes")
			return
		}
		hasScope, err := h.queries.HasTeamAdminScope(ctx, dbquery.HasTeamAdminScopeParams{OrgID: orgID, TeamID: teamID, AdminUserID: claims.UserID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to validate team admin scope")
			return
		}
		if !hasScope {
			writeError(w, http.StatusForbidden, "insufficient permissions for team membership changes")
			return
		}
		if req.Role != roleMember {
			writeError(w, http.StatusForbidden, "team_admin can only add members with role member")
			return
		}
	}

	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	targetUserID, err := h.resolveTargetUser(ctx, tx.Queries, req.UserID, req.Email, req.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		if err == errMissingUserReference {
			writeError(w, http.StatusBadRequest, "user_id or email is required")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve target user")
		return
	}

	if _, err := tx.Queries.UpsertOrgMembership(ctx, dbquery.UpsertOrgMembershipParams{
		OrgID:  orgID,
		UserID: targetUserID,
		Role:   req.Role,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert org membership")
		return
	}
	if _, err := tx.Queries.UpsertTeamMembership(ctx, dbquery.UpsertTeamMembershipParams{
		OrgID:  orgID,
		TeamID: teamID,
		UserID: targetUserID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert team membership")
		return
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit team membership change")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"org_id":  orgID,
		"team_id": teamID,
		"user_id": targetUserID,
		"role":    req.Role,
	})
}

func (h *controlHandler) handleAddTeamAdminScope(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	adminUserID := strings.TrimSpace(r.PathValue("user_id"))

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	membership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve requester membership")
		return
	}
	if membership.Role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can assign team admin scopes")
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
	if _, err := h.queries.GetUserByID(ctx, adminUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve user")
		return
	}

	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	if _, err := tx.Queries.UpsertOrgMembership(ctx, dbquery.UpsertOrgMembershipParams{OrgID: orgID, UserID: adminUserID, Role: roleTeamAdmin}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to promote team admin")
		return
	}
	if _, err := tx.Queries.UpsertTeamMembership(ctx, dbquery.UpsertTeamMembershipParams{OrgID: orgID, TeamID: teamID, UserID: adminUserID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to attach team admin membership")
		return
	}
	if _, err := tx.Queries.UpsertTeamAdminScope(ctx, dbquery.UpsertTeamAdminScopeParams{OrgID: orgID, TeamID: teamID, AdminUserID: adminUserID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert team admin scope")
		return
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit team admin scope")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"org_id":        orgID,
		"team_id":       teamID,
		"admin_user_id": adminUserID,
	})
}

func (h *controlHandler) handleCreateUserTeamAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))
	targetUserID := strings.TrimSpace(r.PathValue("user_id"))

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	allowed, err := h.canManageTeamScopedResource(ctx, orgID, teamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for team key management")
		return
	}

	if _, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: targetUserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "target user is not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to verify target membership")
		return
	}

	keyID, err := randomID("ukey")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate key id")
		return
	}
	plaintextKey, keyPrefix, err := generateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate key")
		return
	}

	row, err := h.queries.CreateUserTeamAPIKey(ctx, dbquery.CreateUserTeamAPIKeyParams{
		ID:        keyID,
		OrgID:     orgID,
		TeamID:    teamID,
		UserID:    targetUserID,
		KeyHash:   hashValue(plaintextKey),
		KeyPrefix: keyPrefix,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "active key already exists for org/team/user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         row.ID,
		"org_id":     row.OrgID,
		"team_id":    row.TeamID,
		"user_id":    row.UserID,
		"key_prefix": row.KeyPrefix,
		"api_key":    plaintextKey,
	})
}

func (h *controlHandler) handleRevokeUserTeamAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	keyID := strings.TrimSpace(r.PathValue("key_id"))

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	requesterMembership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve requester membership")
		return
	}
	if requesterMembership.Role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can revoke api keys")
		return
	}

	rows, err := h.queries.RevokeUserTeamAPIKey(ctx, dbquery.RevokeUserTeamAPIKeyParams{ID: keyID, OrgID: orgID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, "api key not found or already revoked")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "revoked",
		"id":     keyID,
	})
}

func (h *controlHandler) handleCreateOrgProviderKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	provider := strings.TrimSpace(r.PathValue("provider"))

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	requesterMembership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve requester membership")
		return
	}
	if requesterMembership.Role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can manage provider keys")
		return
	}

	var req struct {
		APIKey   string `json:"api_key"`
		KeyKekID string `json:"key_kek_id"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.KeyKekID = strings.TrimSpace(req.KeyKekID)
	if req.APIKey == "" || req.KeyKekID == "" {
		writeError(w, http.StatusBadRequest, "api_key and key_kek_id are required")
		return
	}
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}

	secret := os.Getenv("CONTROL_PROVIDER_KEY_SECRET")
	if secret == "" {
		secret = "dev-provider-key-secret"
	}
	ciphertext, nonce, err := internalcrypto.SealString(secret, req.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt provider key")
		return
	}

	keyID, err := randomID("opk")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate provider key id")
		return
	}

	row, err := h.queries.CreateOrgProviderKey(ctx, dbquery.CreateOrgProviderKeyParams{
		ID:            keyID,
		OrgID:         orgID,
		Provider:      provider,
		KeyCiphertext: ciphertext,
		KeyNonce:      nonce,
		KeyKekID:      req.KeyKekID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store provider key")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        row.ID,
		"org_id":    row.OrgID,
		"provider":  row.Provider,
		"is_active": row.IsActive,
	})
}

func (h *controlHandler) handleUpsertOrgModelPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	requesterMembership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: claims.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusForbidden, "not a member of org")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve requester membership")
		return
	}
	if requesterMembership.Role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can manage org policies")
		return
	}

	entries, ok := parsePolicyEntries(r, w)
	if !ok {
		return
	}

	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	for _, entry := range entries {
		if _, err := tx.Queries.UpsertOrgModelPolicy(ctx, dbquery.UpsertOrgModelPolicyParams{
			OrgID:     orgID,
			Provider:  entry.Provider,
			Model:     entry.Model,
			IsAllowed: entry.IsAllowed,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to upsert org policy")
			return
		}
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit org policies")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"updated": len(entries)})
}

func (h *controlHandler) handleUpsertTeamModelPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	teamID := strings.TrimSpace(r.PathValue("team_id"))

	claims, ok := h.requireSession(ctx, w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	allowed, err := h.canManageTeamScopedResource(ctx, orgID, teamID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for team policy management")
		return
	}

	entries, ok := parsePolicyEntries(r, w)
	if !ok {
		return
	}

	orgAllowed, err := h.queries.ListOrgAllowedModels(ctx, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load org allowlist")
		return
	}
	orgAllowSet := make(map[string]struct{}, len(orgAllowed))
	for _, row := range orgAllowed {
		orgAllowSet[row.Provider+":"+row.Model] = struct{}{}
	}
	for _, entry := range entries {
		if entry.IsAllowed {
			if _, ok := orgAllowSet[entry.Provider+":"+entry.Model]; !ok {
				writeError(w, http.StatusBadRequest, "team policy cannot allow model absent from org allowlist")
				return
			}
		}
	}

	tx, err := h.beginTx(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Tx.Rollback()

	for _, entry := range entries {
		if _, err := tx.Queries.UpsertTeamModelPolicy(ctx, dbquery.UpsertTeamModelPolicyParams{
			OrgID:     orgID,
			TeamID:    teamID,
			Provider:  entry.Provider,
			Model:     entry.Model,
			IsAllowed: entry.IsAllowed,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to upsert team policy")
			return
		}
	}

	if err := tx.Tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit team policies")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"updated": len(entries)})
}

func (h *controlHandler) requireSession(ctx context.Context, w http.ResponseWriter, r *http.Request) (auth.SessionClaims, bool) {
	var zero auth.SessionClaims

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing session cookie")
		return zero, false
	}

	claims, err := h.sessions.Open(cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session cookie")
		return zero, false
	}
	if h.now().After(time.Unix(claims.ExpiresAtUnix, 0)) {
		writeError(w, http.StatusUnauthorized, "session expired")
		return zero, false
	}

	refreshToken, err := h.queries.GetRefreshTokenByID(ctx, dbquery.GetRefreshTokenByIDParams{ID: claims.RefreshTokenID, OrgID: claims.OrgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "session invalidated")
			return zero, false
		}
		writeError(w, http.StatusInternalServerError, "failed to validate session")
		return zero, false
	}
	if refreshToken.UserID != claims.UserID || refreshToken.RevokedAt.Valid || h.now().After(refreshToken.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "session invalidated")
		return zero, false
	}

	return claims, true
}

func (h *controlHandler) setSessionCookie(w http.ResponseWriter, claims auth.SessionClaims, expiresAt time.Time) error {
	token, err := h.sessions.Seal(claims)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/v1/control",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
		Expires:  expiresAt,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	return nil
}

func (h *controlHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/v1/control",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

var errMissingUserReference = errors.New("missing user reference")

func (h *controlHandler) resolveTargetUser(ctx context.Context, q *dbquery.Queries, userID, email, name string) (string, error) {
	if userID != "" {
		user, err := q.GetUserByID(ctx, userID)
		if err != nil {
			return "", err
		}
		return user.ID, nil
	}
	if email == "" {
		return "", errMissingUserReference
	}

	user, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		return user.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	newUserID, err := randomID("usr")
	if err != nil {
		return "", err
	}
	created, err := q.CreateUser(ctx, dbquery.CreateUserParams{
		ID:    newUserID,
		Email: email,
		Name:  name,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (h *controlHandler) ensureUserByEmail(ctx context.Context, q *dbquery.Queries, email, name string) (string, error) {
	user, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		return user.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	newUserID, err := randomID("usr")
	if err != nil {
		return "", err
	}
	created, err := q.CreateUser(ctx, dbquery.CreateUserParams{ID: newUserID, Email: email, Name: name})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

type txQueries struct {
	Tx      *sql.Tx
	Queries *dbquery.Queries
}

func (h *controlHandler) beginTx(ctx context.Context) (*txQueries, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &txQueries{Tx: tx, Queries: h.queries.WithTx(tx)}, nil
}

func (h *controlHandler) canManageTeamScopedResource(ctx context.Context, orgID, teamID, userID string) (bool, error) {
	requesterMembership, err := h.queries.GetOrgMembership(ctx, dbquery.GetOrgMembershipParams{OrgID: orgID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if requesterMembership.Role == roleOrgOwner {
		return true, nil
	}
	if requesterMembership.Role != roleTeamAdmin {
		return false, nil
	}
	hasScope, err := h.queries.HasTeamAdminScope(ctx, dbquery.HasTeamAdminScopeParams{
		OrgID:       orgID,
		TeamID:      teamID,
		AdminUserID: userID,
	})
	if err != nil {
		return false, err
	}
	return hasScope, nil
}

type modelPolicyEntry struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	IsAllowed bool   `json:"is_allowed"`
}

func parsePolicyEntries(r *http.Request, w http.ResponseWriter) ([]modelPolicyEntry, bool) {
	var req struct {
		Entries []modelPolicyEntry `json:"entries"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return nil, false
	}
	if len(req.Entries) == 0 {
		writeError(w, http.StatusBadRequest, "entries are required")
		return nil, false
	}
	for i := range req.Entries {
		req.Entries[i].Provider = strings.TrimSpace(req.Entries[i].Provider)
		req.Entries[i].Model = strings.TrimSpace(req.Entries[i].Model)
		if req.Entries[i].Provider == "" || req.Entries[i].Model == "" {
			writeError(w, http.StatusBadRequest, "each policy entry requires provider and model")
			return nil, false
		}
	}
	return req.Entries, true
}

func decodeJSONBody(r *http.Request, out any) error {
	if r.Body == nil {
		return errors.New("missing body")
	}
	defer r.Body.Close()

	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}

func randomCode() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func generateAPIKey() (string, string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	key := "grk_" + hex.EncodeToString(b[:])
	prefixLen := 12
	if len(key) < prefixLen {
		prefixLen = len(key)
	}
	return key, key[:prefixLen], nil
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func truncateDeviceInfo(input string) string {
	if len(input) <= 256 {
		return input
	}
	return input[:256]
}
