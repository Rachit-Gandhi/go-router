package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Rachit-Gandhi/go-router/internal/auth"
	"github.com/Rachit-Gandhi/go-router/internal/httputil"
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

type controlState struct {
	mu sync.Mutex

	users         map[string]userRecord
	usersByEmail  map[string]string
	orgs          map[string]orgRecord
	orgMembership map[string]membershipRecord

	teams           map[string]teamRecord
	teamMemberships map[string]teamMembershipRecord
	teamAdminScopes map[string]struct{}

	magicLinks    map[string]magicLinkRecord
	refreshTokens map[string]refreshTokenRecord
}

type userRecord struct {
	ID    string
	Email string
	Name  string
}

type orgRecord struct {
	ID          string
	Name        string
	OwnerUserID string
}

type membershipRecord struct {
	OrgID  string
	UserID string
	Role   string
}

type teamRecord struct {
	ID    string
	OrgID string
	Name  string
}

type teamMembershipRecord struct {
	OrgID  string
	TeamID string
	UserID string
}

type magicLinkRecord struct {
	ID        string
	OrgID     string
	Email     string
	CodeHash  string
	ExpiresAt time.Time
	Consumed  bool
}

type refreshTokenRecord struct {
	ID         string
	OrgID      string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt time.Time
}

type controlHandler struct {
	state    *controlState
	sessions *auth.SessionCodec
	now      func() time.Time
}

// NewHandler builds the control-plane HTTP router.
func NewHandler() http.Handler {
	secret := os.Getenv("CONTROL_SESSION_SECRET")
	if secret == "" {
		secret = "dev-control-session-secret"
	}
	sessionCodec, err := auth.NewSessionCodec(secret)
	if err != nil {
		panic(fmt.Sprintf("create session codec: %v", err))
	}

	return NewHandlerWithDeps(newControlState(), sessionCodec, time.Now)
}

// NewHandlerWithDeps builds the control-plane router with explicit dependencies.
func NewHandlerWithDeps(state *controlState, sessionCodec *auth.SessionCodec, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}

	h := &controlHandler{
		state:    state,
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

	return mux
}

func newControlState() *controlState {
	return &controlState{
		users:           make(map[string]userRecord),
		usersByEmail:    make(map[string]string),
		orgs:            make(map[string]orgRecord),
		orgMembership:   make(map[string]membershipRecord),
		teams:           make(map[string]teamRecord),
		teamMemberships: make(map[string]teamMembershipRecord),
		teamAdminScopes: make(map[string]struct{}),
		magicLinks:      make(map[string]magicLinkRecord),
		refreshTokens:   make(map[string]refreshTokenRecord),
	}
}

func (h *controlHandler) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
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

	ownerUserID, err := randomID("usr")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate owner ID")
		return
	}
	orgID, err := randomID("org")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate org ID")
		return
	}

	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	if existingUserID, ok := h.state.usersByEmail[req.OwnerEmail]; ok {
		ownerUserID = existingUserID
	} else {
		h.state.users[ownerUserID] = userRecord{
			ID:    ownerUserID,
			Email: req.OwnerEmail,
			Name:  req.OwnerName,
		}
		h.state.usersByEmail[req.OwnerEmail] = ownerUserID
	}

	h.state.orgs[orgID] = orgRecord{
		ID:          orgID,
		Name:        req.OrgName,
		OwnerUserID: ownerUserID,
	}
	h.state.orgMembership[orgUserKey(orgID, ownerUserID)] = membershipRecord{
		OrgID:  orgID,
		UserID: ownerUserID,
		Role:   roleOrgOwner,
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"org_id":        orgID,
		"owner_user_id": ownerUserID,
		"role":          roleOrgOwner,
	})
}

func (h *controlHandler) handleMagicLinkRequest(w http.ResponseWriter, r *http.Request) {
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

	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	userID, userExists := h.state.usersByEmail[req.Email]
	if !userExists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if _, ok := h.state.orgMembership[orgUserKey(req.OrgID, userID)]; !ok {
		writeError(w, http.StatusForbidden, "user is not a member of the org")
		return
	}

	h.state.magicLinks[magicLinkID] = magicLinkRecord{
		ID:        magicLinkID,
		OrgID:     req.OrgID,
		Email:     req.Email,
		CodeHash:  hashValue(code),
		ExpiresAt: now.Add(magicLinkTTL),
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"magic_link_id": magicLinkID,
		"code":          code,
		"expires_at":    now.Add(magicLinkTTL).UTC().Format(time.RFC3339),
	})
}

func (h *controlHandler) handleMagicLinkExchange(w http.ResponseWriter, r *http.Request) {
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

	var (
		userID      string
		orgID       string
		refreshID   string
		sessionEnds = now.Add(sessionTTL)
	)

	h.state.mu.Lock()
	link, ok := h.state.magicLinks[req.MagicLinkID]
	if !ok || link.Consumed || now.After(link.ExpiresAt) {
		h.state.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "invalid or expired magic link")
		return
	}
	if hashValue(req.Code) != link.CodeHash {
		h.state.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "invalid or expired magic link")
		return
	}

	link.Consumed = true
	h.state.magicLinks[req.MagicLinkID] = link

	existingUserID, ok := h.state.usersByEmail[link.Email]
	if !ok {
		h.state.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}
	if _, ok := h.state.orgMembership[orgUserKey(link.OrgID, existingUserID)]; !ok {
		h.state.mu.Unlock()
		writeError(w, http.StatusForbidden, "user is not a member of the org")
		return
	}

	refreshID, _ = randomID("rt")
	tokenMaterial, _ := randomID("rtk")
	h.state.refreshTokens[refreshID] = refreshTokenRecord{
		ID:         refreshID,
		OrgID:      link.OrgID,
		UserID:     existingUserID,
		TokenHash:  hashValue(tokenMaterial),
		ExpiresAt:  now.Add(refreshTTL),
		LastUsedAt: now,
	}
	userID = existingUserID
	orgID = link.OrgID
	h.state.mu.Unlock()

	claims := auth.SessionClaims{
		OrgID:          orgID,
		UserID:         userID,
		RefreshTokenID: refreshID,
		ExpiresAtUnix:  sessionEnds.Unix(),
	}
	if err := h.setSessionCookie(w, claims, sessionEnds); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":     orgID,
		"user_id":    userID,
		"expires_at": sessionEnds.UTC().Format(time.RFC3339),
	})
}

func (h *controlHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSession(w, r)
	if !ok {
		return
	}

	now := h.now()
	newRefreshID, err := randomID("rt")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}
	newTokenMaterial, err := randomID("rtk")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate refresh token")
		return
	}

	h.state.mu.Lock()
	current, exists := h.state.refreshTokens[claims.RefreshTokenID]
	if !exists || current.OrgID != claims.OrgID || current.UserID != claims.UserID || current.RevokedAt != nil || now.After(current.ExpiresAt) {
		h.state.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	current.LastUsedAt = now
	current.RevokedAt = &now
	h.state.refreshTokens[claims.RefreshTokenID] = current
	h.state.refreshTokens[newRefreshID] = refreshTokenRecord{
		ID:         newRefreshID,
		OrgID:      claims.OrgID,
		UserID:     claims.UserID,
		TokenHash:  hashValue(newTokenMaterial),
		ExpiresAt:  now.Add(refreshTTL),
		LastUsedAt: now,
	}
	h.state.mu.Unlock()

	sessionEnds := now.Add(sessionTTL)
	newClaims := auth.SessionClaims{
		OrgID:          claims.OrgID,
		UserID:         claims.UserID,
		RefreshTokenID: newRefreshID,
		ExpiresAtUnix:  sessionEnds.Unix(),
	}
	if err := h.setSessionCookie(w, newClaims, sessionEnds); err != nil {
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
	claims, ok := h.requireSession(w, r)
	if !ok {
		return
	}

	now := h.now()
	h.state.mu.Lock()
	if token, exists := h.state.refreshTokens[claims.RefreshTokenID]; exists && token.OrgID == claims.OrgID && token.UserID == claims.UserID && token.RevokedAt == nil {
		token.RevokedAt = &now
		token.LastUsedAt = now
		h.state.refreshTokens[claims.RefreshTokenID] = token
	}
	h.state.mu.Unlock()

	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (h *controlHandler) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	claims, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
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

	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	role, ok := h.state.orgRoleLocked(orgID, claims.UserID)
	if !ok || role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can create teams")
		return
	}

	h.state.teams[teamID] = teamRecord{
		ID:    teamID,
		OrgID: orgID,
		Name:  req.Name,
	}
	h.state.teamMemberships[teamUserKey(orgID, teamID, claims.UserID)] = teamMembershipRecord{
		OrgID:  orgID,
		TeamID: teamID,
		UserID: claims.UserID,
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     teamID,
		"org_id": orgID,
		"name":   req.Name,
	})
}

func (h *controlHandler) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	teamID := r.PathValue("team_id")

	claims, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
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

	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	team, exists := h.state.teams[teamID]
	if !exists || team.OrgID != orgID {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}

	requesterRole, requesterExists := h.state.orgRoleLocked(orgID, claims.UserID)
	if !requesterExists {
		writeError(w, http.StatusForbidden, "not a member of org")
		return
	}

	isAllowed := requesterRole == roleOrgOwner || (requesterRole == roleTeamAdmin && h.state.hasTeamAdminScopeLocked(orgID, teamID, claims.UserID))
	if !isAllowed {
		writeError(w, http.StatusForbidden, "insufficient permissions for team membership changes")
		return
	}
	if requesterRole != roleOrgOwner && req.Role != roleMember {
		writeError(w, http.StatusForbidden, "team_admin can only add members with role member")
		return
	}

	targetUserID := req.UserID
	if targetUserID == "" {
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "user_id or email is required")
			return
		}
		if existingID, ok := h.state.usersByEmail[req.Email]; ok {
			targetUserID = existingID
		} else {
			newUserID, err := randomID("usr")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to generate user ID")
				return
			}
			name := req.Name
			if name == "" {
				name = strings.Split(req.Email, "@")[0]
			}
			h.state.users[newUserID] = userRecord{
				ID:    newUserID,
				Email: req.Email,
				Name:  name,
			}
			h.state.usersByEmail[req.Email] = newUserID
			targetUserID = newUserID
		}
	} else {
		if _, ok := h.state.users[targetUserID]; !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
	}

	h.state.orgMembership[orgUserKey(orgID, targetUserID)] = membershipRecord{
		OrgID:  orgID,
		UserID: targetUserID,
		Role:   req.Role,
	}
	h.state.teamMemberships[teamUserKey(orgID, teamID, targetUserID)] = teamMembershipRecord{
		OrgID:  orgID,
		TeamID: teamID,
		UserID: targetUserID,
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"org_id":  orgID,
		"team_id": teamID,
		"user_id": targetUserID,
		"role":    req.Role,
	})
}

func (h *controlHandler) handleAddTeamAdminScope(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	teamID := r.PathValue("team_id")
	adminUserID := r.PathValue("user_id")

	claims, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if claims.OrgID != orgID {
		writeError(w, http.StatusForbidden, "session org mismatch")
		return
	}

	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	role, ok := h.state.orgRoleLocked(orgID, claims.UserID)
	if !ok || role != roleOrgOwner {
		writeError(w, http.StatusForbidden, "only org owner can assign team admin scopes")
		return
	}
	team, teamExists := h.state.teams[teamID]
	if !teamExists || team.OrgID != orgID {
		writeError(w, http.StatusNotFound, "team not found")
		return
	}
	if _, userExists := h.state.users[adminUserID]; !userExists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	h.state.orgMembership[orgUserKey(orgID, adminUserID)] = membershipRecord{
		OrgID:  orgID,
		UserID: adminUserID,
		Role:   roleTeamAdmin,
	}
	h.state.teamMemberships[teamUserKey(orgID, teamID, adminUserID)] = teamMembershipRecord{
		OrgID:  orgID,
		TeamID: teamID,
		UserID: adminUserID,
	}
	h.state.teamAdminScopes[scopeKey(orgID, teamID, adminUserID)] = struct{}{}

	writeJSON(w, http.StatusCreated, map[string]any{
		"org_id":        orgID,
		"team_id":       teamID,
		"admin_user_id": adminUserID,
	})
}

func (h *controlHandler) requireSession(w http.ResponseWriter, r *http.Request) (auth.SessionClaims, bool) {
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
	h.state.mu.Lock()
	token, ok := h.state.refreshTokens[claims.RefreshTokenID]
	h.state.mu.Unlock()
	if !ok || token.OrgID != claims.OrgID || token.UserID != claims.UserID || token.RevokedAt != nil || h.now().After(token.ExpiresAt) {
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

func (s *controlState) orgRoleLocked(orgID, userID string) (string, bool) {
	m, ok := s.orgMembership[orgUserKey(orgID, userID)]
	if !ok {
		return "", false
	}
	return m.Role, true
}

func (s *controlState) hasTeamAdminScopeLocked(orgID, teamID, userID string) bool {
	_, ok := s.teamAdminScopes[scopeKey(orgID, teamID, userID)]
	return ok
}

func orgUserKey(orgID, userID string) string {
	return orgID + ":" + userID
}

func teamUserKey(orgID, teamID, userID string) string {
	return orgID + ":" + teamID + ":" + userID
}

func scopeKey(orgID, teamID, userID string) string {
	return orgID + ":" + teamID + ":" + userID
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

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
