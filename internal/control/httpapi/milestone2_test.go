package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrgSignupAndOwnerCanCreateTeam(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, h)

	createTeamReq := map[string]any{
		"name": "core-team",
	}
	rec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", createTeamReq, ownerCookie)
	requireStatus(t, rec, http.StatusCreated)

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
	tc := newTestHandler(t)
	h := tc.handler

	orgID, _, _ := signupAndAuthenticateOwner(t, h)

	linkReq := map[string]any{
		"org_id": orgID,
		"email":  "owner@example.com",
	}
	linkRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", linkReq, "")
	requireStatus(t, linkRec, http.StatusOK)

	var linkBody map[string]any
	decodeJSON(t, linkRec, &linkBody)

	exchangeReq := map[string]any{
		"magic_link_id": linkBody["magic_link_id"],
		"code":          linkBody["code"],
	}
	exchangeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", exchangeReq, "")
	requireStatus(t, exchangeRec, http.StatusOK)
	authCookie := exchangeRec.Header().Get("Set-Cookie")
	if authCookie == "" {
		t.Fatal("expected auth exchange to set cookie")
	}

	refreshRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/refresh", map[string]any{}, authCookie)
	requireStatus(t, refreshRec, http.StatusOK)
	refreshedCookie := refreshRec.Header().Get("Set-Cookie")
	if refreshedCookie == "" {
		t.Fatal("expected refresh to rotate cookie")
	}

	logoutRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/logout", map[string]any{}, refreshedCookie)
	requireStatus(t, logoutRec, http.StatusOK)
	if logoutRec.Header().Get("Set-Cookie") == "" {
		t.Fatal("expected logout to clear cookie")
	}

	unauthorizedCreateReq := map[string]any{
		"name": "post-logout-team",
	}
	unauthorizedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", unauthorizedCreateReq, refreshedCookie)
	requireStatus(t, unauthorizedRec, http.StatusUnauthorized)
}

func TestTeamAdminScopeEnforcement(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, h)

	teamAID := createTeam(t, h, orgID, ownerCookie, "team-a")
	teamBID := createTeam(t, h, orgID, ownerCookie, "team-b")

	memberRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamAID+"/members", map[string]any{
		"email": "admin@example.com",
		"name":  "Admin User",
		"role":  "team_admin",
	}, ownerCookie)
	requireStatus(t, memberRec, http.StatusCreated)
	var memberBody map[string]any
	decodeJSON(t, memberRec, &memberBody)
	adminUserID, _ := memberBody["user_id"].(string)
	if adminUserID == "" {
		t.Fatal("expected created team member user_id")
	}

	scopeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamAID+"/admins/"+adminUserID, map[string]any{}, ownerCookie)
	requireStatus(t, scopeRec, http.StatusCreated)

	adminMagicLink := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", map[string]any{
		"org_id": orgID,
		"email":  "admin@example.com",
	}, "")
	requireStatus(t, adminMagicLink, http.StatusOK)
	var adminMagicBody map[string]any
	decodeJSON(t, adminMagicLink, &adminMagicBody)

	adminExchange := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", map[string]any{
		"magic_link_id": adminMagicBody["magic_link_id"],
		"code":          adminMagicBody["code"],
	}, "")
	requireStatus(t, adminExchange, http.StatusOK)
	adminCookie := adminExchange.Header().Get("Set-Cookie")
	if adminCookie == "" {
		t.Fatal("expected admin exchange to set cookie")
	}

	allowedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamAID+"/members", map[string]any{
		"email": "member1@example.com",
		"name":  "Scoped Member",
	}, adminCookie)
	requireStatus(t, allowedRec, http.StatusCreated)

	deniedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamBID+"/members", map[string]any{
		"email": "member2@example.com",
		"name":  "Unscoped Member",
	}, adminCookie)
	requireStatus(t, deniedRec, http.StatusForbidden)

	ownerAllowedRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamBID+"/members", map[string]any{
		"email": "member3@example.com",
		"name":  "Owner Managed",
	}, ownerCookie)
	requireStatus(t, ownerAllowedRec, http.StatusCreated)

	adminCreateTeamRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams", map[string]any{
		"name": "forbidden-team-create",
	}, adminCookie)
	requireStatus(t, adminCreateTeamRec, http.StatusForbidden)
}

func signupAndAuthenticateOwner(t *testing.T, h http.Handler) (orgID string, ownerUserID string, authCookie string) {
	t.Helper()

	signupReq := map[string]any{
		"org_name":    "acme",
		"owner_email": "owner@example.com",
		"owner_name":  "Owner User",
	}
	signupRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs", signupReq, "")
	requireStatus(t, signupRec, http.StatusCreated)

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
	requireStatus(t, linkRec, http.StatusOK)

	var linkBody map[string]any
	decodeJSON(t, linkRec, &linkBody)

	exchangeReq := map[string]any{
		"magic_link_id": linkBody["magic_link_id"],
		"code":          linkBody["code"],
	}
	exchangeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", exchangeReq, "")
	requireStatus(t, exchangeRec, http.StatusOK)
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
	requireStatus(t, rec, http.StatusCreated)

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

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if rec.Code != expected {
		t.Fatalf("expected status %d, got %d: %s", expected, rec.Code, rec.Body.String())
	}
}
