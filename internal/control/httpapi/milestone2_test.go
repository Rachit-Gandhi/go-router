package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrgSignupAndOwnerCanCreateTeam(t *testing.T) {
	h := NewHandler()

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, h)

	createTeamReq := map[string]any{
		"name": "core-team",
	}
	rec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", createTeamReq, ownerCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var body map[string]any
	decodeJSON(t, rec, &body)
	if body["org_id"] != orgID {
		t.Fatalf("expected team org_id %q, got %#v", orgID, body["org_id"])
	}
	if body["name"] != "core-team" {
		t.Fatalf("expected team name %q, got %#v", "core-team", body["name"])
	}
}

func TestMagicLinkExchangeRefreshAndLogout(t *testing.T) {
	h := NewHandler()

	orgID, _, _ := signupAndAuthenticateOwner(t, h)

	linkReq := map[string]any{
		"org_id": orgID,
		"email":  "owner@example.com",
	}
	linkRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", linkReq, "")
	if linkRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, linkRec.Code)
	}

	var linkBody map[string]any
	decodeJSON(t, linkRec, &linkBody)

	exchangeReq := map[string]any{
		"magic_link_id": linkBody["magic_link_id"],
		"code":          linkBody["code"],
	}
	exchangeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", exchangeReq, "")
	if exchangeRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, exchangeRec.Code)
	}
	authCookie := exchangeRec.Header().Get("Set-Cookie")
	if authCookie == "" {
		t.Fatal("expected auth exchange to set cookie")
	}

	refreshRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/refresh", map[string]any{}, authCookie)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, refreshRec.Code)
	}
	refreshedCookie := refreshRec.Header().Get("Set-Cookie")
	if refreshedCookie == "" {
		t.Fatal("expected refresh to rotate cookie")
	}

	logoutRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/logout", map[string]any{}, refreshedCookie)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, logoutRec.Code)
	}
	if logoutRec.Header().Get("Set-Cookie") == "" {
		t.Fatal("expected logout to clear cookie")
	}

	unauthorizedCreateReq := map[string]any{
		"name": "post-logout-team",
	}
	unauthorizedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", unauthorizedCreateReq, refreshedCookie)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, unauthorizedRec.Code)
	}
}

func TestTeamAdminScopeEnforcement(t *testing.T) {
	h := NewHandler()

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, h)

	teamAID := createTeam(t, h, orgID, ownerCookie, "team-a")
	teamBID := createTeam(t, h, orgID, ownerCookie, "team-b")

	memberRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamAID+"/members", map[string]any{
		"email": "admin@example.com",
		"name":  "Admin User",
		"role":  "team_admin",
	}, ownerCookie)
	if memberRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, memberRec.Code)
	}
	var memberBody map[string]any
	decodeJSON(t, memberRec, &memberBody)
	adminUserID, _ := memberBody["user_id"].(string)
	if adminUserID == "" {
		t.Fatal("expected created team member user_id")
	}

	scopeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamAID+"/admins/"+adminUserID, map[string]any{}, ownerCookie)
	if scopeRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, scopeRec.Code)
	}

	adminMagicLink := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", map[string]any{
		"org_id": orgID,
		"email":  "admin@example.com",
	}, "")
	if adminMagicLink.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, adminMagicLink.Code)
	}
	var adminMagicBody map[string]any
	decodeJSON(t, adminMagicLink, &adminMagicBody)

	adminExchange := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", map[string]any{
		"magic_link_id": adminMagicBody["magic_link_id"],
		"code":          adminMagicBody["code"],
	}, "")
	if adminExchange.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, adminExchange.Code)
	}
	adminCookie := adminExchange.Header().Get("Set-Cookie")
	if adminCookie == "" {
		t.Fatal("expected admin exchange to set cookie")
	}

	allowedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamAID+"/members", map[string]any{
		"email": "member1@example.com",
		"name":  "Scoped Member",
	}, adminCookie)
	if allowedRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, allowedRec.Code)
	}

	deniedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamBID+"/members", map[string]any{
		"email": "member2@example.com",
		"name":  "Unscoped Member",
	}, adminCookie)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, deniedRec.Code)
	}

	ownerAllowedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamBID+"/members", map[string]any{
		"email": "member3@example.com",
		"name":  "Owner Managed",
	}, ownerCookie)
	if ownerAllowedRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, ownerAllowedRec.Code)
	}

	adminCreateTeamRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", map[string]any{
		"name": "forbidden-team-create",
	}, adminCookie)
	if adminCreateTeamRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, adminCreateTeamRec.Code)
	}
}

func signupAndAuthenticateOwner(t *testing.T, h http.Handler) (orgID string, ownerUserID string, authCookie string) {
	t.Helper()

	signupReq := map[string]any{
		"org_name":    "acme",
		"owner_email": "owner@example.com",
		"owner_name":  "Owner User",
	}
	signupRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs", signupReq, "")
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, signupRec.Code)
	}

	var signupBody map[string]any
	decodeJSON(t, signupRec, &signupBody)

	orgID, _ = signupBody["org_id"].(string)
	ownerUserID, _ = signupBody["owner_user_id"].(string)
	if orgID == "" || ownerUserID == "" {
		t.Fatalf("expected signup response to include org and owner IDs, got %#v", signupBody)
	}

	linkReq := map[string]any{
		"org_id": orgID,
		"email":  "owner@example.com",
	}
	linkRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", linkReq, "")
	if linkRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, linkRec.Code)
	}

	var linkBody map[string]any
	decodeJSON(t, linkRec, &linkBody)

	exchangeReq := map[string]any{
		"magic_link_id": linkBody["magic_link_id"],
		"code":          linkBody["code"],
	}
	exchangeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", exchangeReq, "")
	if exchangeRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, exchangeRec.Code)
	}
	authCookie = exchangeRec.Header().Get("Set-Cookie")
	if authCookie == "" {
		t.Fatal("expected auth cookie from exchange")
	}
	return orgID, ownerUserID, authCookie
}

func createTeam(t *testing.T, h http.Handler, orgID, cookie, name string) string {
	t.Helper()

	rec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", map[string]any{
		"name": name,
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var body map[string]any
	decodeJSON(t, rec, &body)

	teamID, _ := body["id"].(string)
	if teamID == "" {
		t.Fatalf("expected team id in response, got %#v", body)
	}
	return teamID
}

func performJSONRequest(t *testing.T, h http.Handler, method, path string, payload map[string]any, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode JSON response: %v (%s)", err, rec.Body.String())
	}
}
