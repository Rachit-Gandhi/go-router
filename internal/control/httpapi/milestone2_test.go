package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
	"github.com/Rachit-Gandhi/go-router/internal/auth"
)

func TestOrgSignupAndOwnerCanCreateTeam(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, tc)

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

	orgID, _, _ := signupAndAuthenticateOwner(t, tc)

	linkReq := map[string]any{
		"email": "owner@example.com",
	}
	linkRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", linkReq, "")
	requireStatus(t, linkRec, http.StatusOK)

	var linkBody map[string]any
	decodeJSON(t, linkRec, &linkBody)
	magicLinkID, _ := linkBody["magic_link_id"].(string)
	if magicLinkID == "" {
		t.Fatalf("expected magic_link_id in response, got %#v", linkBody)
	}

	exchangeReq := map[string]any{
		"magic_link_id": magicLinkID,
		"code":          mustMagicCode(t, tc, magicLinkID),
	}
	exchangeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", exchangeReq, "")
	requireStatus(t, exchangeRec, http.StatusOK)
	authCookie := exchangeRec.Header().Get("Set-Cookie")
	if authCookie == "" {
		t.Fatal("expected auth exchange to set cookie")
	}
	var exchangeBody map[string]any
	decodeJSON(t, exchangeRec, &exchangeBody)
	if exchangeBody["role"] != roleOrgOwner {
		t.Fatalf("expected role %q in exchange response, got %#v", roleOrgOwner, exchangeBody["role"])
	}
	sessionClaims := decodeSessionClaimsFromSetCookie(t, tc, exchangeRec, sessionCookieName)
	if sessionClaims.Role != roleOrgOwner {
		t.Fatalf("expected session cookie role %q, got %q", roleOrgOwner, sessionClaims.Role)
	}

	refreshRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/refresh", map[string]any{}, authCookie)
	requireStatus(t, refreshRec, http.StatusOK)
	refreshedCookie := refreshRec.Header().Get("Set-Cookie")
	if refreshedCookie == "" {
		t.Fatal("expected refresh to rotate cookie")
	}
	var refreshBody map[string]any
	decodeJSON(t, refreshRec, &refreshBody)
	if refreshBody["role"] != roleOrgOwner {
		t.Fatalf("expected role %q in refresh response, got %#v", roleOrgOwner, refreshBody["role"])
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

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, tc)

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
		"email": "admin@example.com",
	}, "")
	requireStatus(t, adminMagicLink, http.StatusOK)
	var adminMagicBody map[string]any
	decodeJSON(t, adminMagicLink, &adminMagicBody)
	adminMagicLinkID, _ := adminMagicBody["magic_link_id"].(string)
	if adminMagicLinkID == "" {
		t.Fatalf("expected admin magic_link_id, got %#v", adminMagicBody)
	}

	adminExchange := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", map[string]any{
		"magic_link_id": adminMagicLinkID,
		"code":          mustMagicCode(t, tc, adminMagicLinkID),
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

func TestMagicLinkRequestLocalizesLoginToOrgMembership(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler
	queries := dbquery.New(tc.db)

	orgAID, _, _ := signupAndAuthenticateOwner(t, tc)

	orgBSignup := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs", map[string]any{
		"org_name":    "other-org",
		"owner_email": "other-owner@example.com",
		"owner_name":  "Other Owner",
	}, "")
	requireStatus(t, orgBSignup, http.StatusCreated)
	var orgBBody map[string]any
	decodeJSON(t, orgBSignup, &orgBBody)
	orgBID, _ := orgBBody["org_id"].(string)
	if orgBID == "" {
		t.Fatalf("expected org B id in signup response, got %#v", orgBBody)
	}

	otherLoginReq := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", map[string]any{
		"email": "other-owner@example.com",
	}, "")
	requireStatus(t, otherLoginReq, http.StatusOK)
	var otherLoginBody map[string]any
	decodeJSON(t, otherLoginReq, &otherLoginBody)
	otherMagicLinkID, _ := otherLoginBody["magic_link_id"].(string)
	if otherMagicLinkID == "" {
		t.Fatalf("expected other owner magic_link_id, got %#v", otherLoginBody)
	}

	otherExchange := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", map[string]any{
		"magic_link_id": otherMagicLinkID,
		"code":          mustMagicCode(t, tc, otherMagicLinkID),
	}, "")
	requireStatus(t, otherExchange, http.StatusOK)
	otherOwnerCookie := otherExchange.Header().Get("Set-Cookie")
	if otherOwnerCookie == "" {
		t.Fatal("expected other owner auth cookie")
	}

	orgBTeamID := createTeam(t, h, orgBID, otherOwnerCookie, "other-team")
	memberAdd := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgBID+"/teams/"+orgBTeamID+"/members", map[string]any{
		"email": "owner@example.com",
		"name":  "Cross Org Member",
		"role":  roleMember,
	}, otherOwnerCookie)
	requireStatus(t, memberAdd, http.StatusCreated)

	requestRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", map[string]any{
		"email": "owner@example.com",
	}, "")
	requireStatus(t, requestRec, http.StatusOK)
	var requestBody map[string]any
	decodeJSON(t, requestRec, &requestBody)
	magicLinkID, _ := requestBody["magic_link_id"].(string)
	if magicLinkID == "" {
		t.Fatalf("expected owner magic_link_id in response, got %#v", requestBody)
	}

	loginContext, err := queries.GetAuthLoginByMagicLinkID(t.Context(), magicLinkID)
	if err != nil {
		t.Fatalf("expected auth login context row: %v", err)
	}
	if loginContext.OrgID != orgAID {
		t.Fatalf("expected localized login org %q, got %q", orgAID, loginContext.OrgID)
	}
	if loginContext.Role != roleOrgOwner {
		t.Fatalf("expected localized role %q, got %q", roleOrgOwner, loginContext.Role)
	}
}

func TestMagicLinkRequestRespectsRequestedOrgID(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler
	queries := dbquery.New(tc.db)

	_, _, _ = signupAndAuthenticateOwner(t, tc)

	orgBSignup := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs", map[string]any{
		"org_name":    "other-org",
		"owner_email": "other-owner@example.com",
		"owner_name":  "Other Owner",
	}, "")
	requireStatus(t, orgBSignup, http.StatusCreated)
	var orgBBody map[string]any
	decodeJSON(t, orgBSignup, &orgBBody)
	orgBID, _ := orgBBody["org_id"].(string)
	if orgBID == "" {
		t.Fatalf("expected org B id in signup response, got %#v", orgBBody)
	}

	otherLoginReq := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", map[string]any{
		"email": "other-owner@example.com",
	}, "")
	requireStatus(t, otherLoginReq, http.StatusOK)
	var otherLoginBody map[string]any
	decodeJSON(t, otherLoginReq, &otherLoginBody)
	otherMagicLinkID, _ := otherLoginBody["magic_link_id"].(string)
	if otherMagicLinkID == "" {
		t.Fatalf("expected other owner magic_link_id, got %#v", otherLoginBody)
	}

	otherExchange := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", map[string]any{
		"magic_link_id": otherMagicLinkID,
		"code":          mustMagicCode(t, tc, otherMagicLinkID),
	}, "")
	requireStatus(t, otherExchange, http.StatusOK)
	otherOwnerCookie := otherExchange.Header().Get("Set-Cookie")
	if otherOwnerCookie == "" {
		t.Fatal("expected other owner auth cookie")
	}

	orgBTeamID := createTeam(t, h, orgBID, otherOwnerCookie, "other-team")
	memberAdd := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgBID+"/teams/"+orgBTeamID+"/members", map[string]any{
		"email": "owner@example.com",
		"name":  "Cross Org Member",
		"role":  roleMember,
	}, otherOwnerCookie)
	requireStatus(t, memberAdd, http.StatusCreated)

	requestRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", map[string]any{
		"org_id": orgBID,
		"email":  "owner@example.com",
	}, "")
	requireStatus(t, requestRec, http.StatusOK)
	var requestBody map[string]any
	decodeJSON(t, requestRec, &requestBody)
	magicLinkID, _ := requestBody["magic_link_id"].(string)
	if magicLinkID == "" {
		t.Fatalf("expected owner magic_link_id in response, got %#v", requestBody)
	}

	loginContext, err := queries.GetAuthLoginByMagicLinkID(t.Context(), magicLinkID)
	if err != nil {
		t.Fatalf("expected auth login context row: %v", err)
	}
	if loginContext.OrgID != orgBID {
		t.Fatalf("expected localized login org %q, got %q", orgBID, loginContext.OrgID)
	}
	if loginContext.Role != roleMember {
		t.Fatalf("expected localized role %q, got %q", roleMember, loginContext.Role)
	}
}

func signupAndAuthenticateOwner(t *testing.T, tc *testControlHandler) (orgID string, ownerUserID string, authCookie string) {
	t.Helper()
	h := tc.handler

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
		"email": "owner@example.com",
	}
	linkRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/request", linkReq, "")
	requireStatus(t, linkRec, http.StatusOK)

	var linkBody map[string]any
	decodeJSON(t, linkRec, &linkBody)
	magicLinkID, _ := linkBody["magic_link_id"].(string)
	if magicLinkID == "" {
		t.Fatalf("expected magic_link_id in response, got %#v", linkBody)
	}

	exchangeReq := map[string]any{
		"magic_link_id": magicLinkID,
		"code":          mustMagicCode(t, tc, magicLinkID),
	}
	exchangeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/auth/magic-link/exchange", exchangeReq, "")
	requireStatus(t, exchangeRec, http.StatusOK)
	authCookie = exchangeRec.Header().Get("Set-Cookie")
	if authCookie == "" {
		t.Fatal("expected auth cookie from exchange")
	}
	return orgID, ownerUserID, authCookie
}

func mustMagicCode(t *testing.T, tc *testControlHandler, magicLinkID string) string {
	t.Helper()
	code := tc.mailer.CodeForMagicLink(magicLinkID)
	if code == "" {
		t.Fatalf("expected magic code to be captured for %s", magicLinkID)
	}
	return code
}

func decodeSessionClaimsFromSetCookie(t *testing.T, tc *testControlHandler, rec *httptest.ResponseRecorder, cookieName string) auth.SessionClaims {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name != cookieName {
			continue
		}
		claims, err := tc.codec.Open(c.Value)
		if err != nil {
			t.Fatalf("open session cookie claims: %v", err)
		}
		return claims
	}
	t.Fatalf("expected cookie %q in response", cookieName)
	return auth.SessionClaims{}
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
