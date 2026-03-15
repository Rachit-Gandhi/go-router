package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
)

func TestVisibilityEndpoints(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler
	queries := dbquery.New(tc.db)

	orgID, ownerUserID, ownerCookie := signupAndAuthenticateOwner(t, h)
	teamID := createTeam(t, h, orgID, ownerCookie, "vis-team")
	memberUserID := addMemberByEmail(t, h, orgID, teamID, ownerCookie, "vis-member@example.com", "Visibility Member", "")
	adminUserID := addMemberByEmail(t, h, orgID, teamID, ownerCookie, "vis-admin@example.com", "Visibility Admin", roleTeamAdmin)

	scopeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/admins/"+adminUserID, map[string]any{}, ownerCookie)
	requireStatus(t, scopeRec, http.StatusCreated)

	createKeyRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/users/"+memberUserID+"/api-keys", map[string]any{}, ownerCookie)
	requireStatus(t, createKeyRec, http.StatusCreated)
	var createKeyBody map[string]any
	decodeJSON(t, createKeyRec, &createKeyBody)
	apiKeyID, _ := createKeyBody["id"].(string)
	if apiKeyID == "" {
		t.Fatalf("expected api key id in response, got %#v", createKeyBody)
	}

	providerKeyRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/providers/openai/keys", map[string]any{
		"api_key":    "sk-live-provider-test",
		"key_kek_id": "kek-v1",
	}, ownerCookie)
	requireStatus(t, providerKeyRec, http.StatusCreated)

	orgPolicyRec := performJSONRequest(t, h, http.MethodPut, "/v1/control/orgs/"+orgID+"/policies/models", map[string]any{
		"entries": []map[string]any{
			{"provider": "openai", "model": "gpt-4o-mini", "is_allowed": true},
			{"provider": "openai", "model": "gpt-4o", "is_allowed": true},
			{"provider": "claude", "model": "sonnet-4", "is_allowed": false},
		},
	}, ownerCookie)
	requireStatus(t, orgPolicyRec, http.StatusOK)

	teamPolicyRec := performJSONRequest(t, h, http.MethodPut, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/policies/models", map[string]any{
		"entries": []map[string]any{
			{"provider": "openai", "model": "gpt-4o", "is_allowed": false},
		},
	}, ownerCookie)
	requireStatus(t, teamPolicyRec, http.StatusOK)

	revokeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/api-keys/"+apiKeyID+"/revoke", map[string]any{}, ownerCookie)
	requireStatus(t, revokeRec, http.StatusOK)

	for i, statusCode := range []int32{200, 500} {
		_, err := queries.CreateUsageLog(t.Context(), dbquery.CreateUsageLogParams{
			OrgID:              orgID,
			TeamID:             teamID,
			UserID:             memberUserID,
			ApiKeyID:           apiKeyID,
			Provider:           "openai",
			Model:              "gpt-4o-mini",
			RequestTokens:      int32(100 + i),
			ResponseTokens:     int32(50 + i),
			LatencyMs:          int32(90 + i*10),
			StatusCode:         statusCode,
			RequestFingerprint: toNullString("fp-vis-" + strconvI(i)),
		})
		if err != nil {
			t.Fatalf("create usage log %d: %v", i, err)
		}
	}
	if _, err := tc.db.ExecContext(t.Context(), `
INSERT INTO model_pricing (
	provider,
	model,
	input_price_per_mtok,
	output_price_per_mtok,
	currency,
	source,
	effective_from
) VALUES ('openai', 'gpt-4o-mini', 0.15, 0.60, 'USD', 'test-fixture', NOW() - INTERVAL '1 day');
`); err != nil {
		t.Fatalf("seed model pricing: %v", err)
	}

	sessionRec := performGETRequest(t, h, "/v1/control/session", ownerCookie)
	requireStatus(t, sessionRec, http.StatusOK)
	var sessionBody map[string]any
	decodeJSON(t, sessionRec, &sessionBody)
	if sessionBody["org_id"] != orgID || sessionBody["user_id"] != ownerUserID {
		t.Fatalf("unexpected session payload: %#v", sessionBody)
	}

	summaryRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/summary", ownerCookie)
	requireStatus(t, summaryRec, http.StatusOK)
	var summaryBody map[string]any
	decodeJSON(t, summaryRec, &summaryBody)
	if int(summaryBody["teams"].(float64)) < 1 {
		t.Fatalf("expected teams count >= 1, got %#v", summaryBody)
	}

	teamsRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/teams?limit=10", ownerCookie)
	requireStatus(t, teamsRec, http.StatusOK)
	var teamsBody map[string]any
	decodeJSON(t, teamsRec, &teamsBody)
	if len(teamsBody["items"].([]any)) < 1 {
		t.Fatalf("expected at least one team item, got %#v", teamsBody)
	}

	membersRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/members", ownerCookie)
	requireStatus(t, membersRec, http.StatusOK)
	var membersBody map[string]any
	decodeJSON(t, membersRec, &membersBody)
	if len(membersBody["items"].([]any)) < 2 {
		t.Fatalf("expected at least two team members, got %#v", membersBody)
	}

	adminsRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/admins", ownerCookie)
	requireStatus(t, adminsRec, http.StatusOK)
	var adminsBody map[string]any
	decodeJSON(t, adminsRec, &adminsBody)
	adminItems := adminsBody["items"].([]any)
	var sawOwner, sawScopedAdmin bool
	for _, raw := range adminItems {
		item := raw.(map[string]any)
		switch item["user_id"] {
		case ownerUserID:
			sawOwner = true
		case adminUserID:
			sawScopedAdmin = true
		}
	}
	if !sawOwner || !sawScopedAdmin {
		t.Fatalf("expected owner and scoped admin in admin list, got %#v", adminsBody)
	}

	apiKeysRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/api-keys?include_revoked=true", ownerCookie)
	requireStatus(t, apiKeysRec, http.StatusOK)
	var apiKeysBody map[string]any
	decodeJSON(t, apiKeysRec, &apiKeysBody)
	if len(apiKeysBody["items"].([]any)) < 1 {
		t.Fatalf("expected at least one api key row, got %#v", apiKeysBody)
	}

	providerKeysRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/providers/keys", ownerCookie)
	requireStatus(t, providerKeysRec, http.StatusOK)
	var providerKeysBody map[string]any
	decodeJSON(t, providerKeysRec, &providerKeysBody)
	if len(providerKeysBody["items"].([]any)) < 1 {
		t.Fatalf("expected at least one provider key row, got %#v", providerKeysBody)
	}

	orgPoliciesRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/policies/models", ownerCookie)
	requireStatus(t, orgPoliciesRec, http.StatusOK)
	var orgPoliciesBody map[string]any
	decodeJSON(t, orgPoliciesRec, &orgPoliciesBody)
	if len(orgPoliciesBody["items"].([]any)) < 2 {
		t.Fatalf("expected org policies, got %#v", orgPoliciesBody)
	}

	teamPoliciesRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/policies/models", ownerCookie)
	requireStatus(t, teamPoliciesRec, http.StatusOK)
	var teamPoliciesBody map[string]any
	decodeJSON(t, teamPoliciesRec, &teamPoliciesBody)
	if len(teamPoliciesBody["items"].([]any)) < 1 {
		t.Fatalf("expected team policies, got %#v", teamPoliciesBody)
	}

	effectiveRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/policies/effective-models", ownerCookie)
	requireStatus(t, effectiveRec, http.StatusOK)
	var effectiveBody map[string]any
	decodeJSON(t, effectiveRec, &effectiveBody)
	effectiveItems := effectiveBody["items"].([]any)
	if len(effectiveItems) < 1 {
		t.Fatalf("expected effective models, got %#v", effectiveBody)
	}

	from := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)

	usageSummaryRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/usage/summary?from="+from+"&to="+to, ownerCookie)
	requireStatus(t, usageSummaryRec, http.StatusOK)
	var usageSummaryBody map[string]any
	decodeJSON(t, usageSummaryRec, &usageSummaryBody)
	if int(usageSummaryBody["request_count"].(float64)) < 2 {
		t.Fatalf("expected usage summary request_count >= 2, got %#v", usageSummaryBody)
	}
	if usageSummaryBody["cost_estimate"] != true {
		t.Fatalf("expected cost_estimate=true, got %#v", usageSummaryBody["cost_estimate"])
	}
	if usageSummaryBody["estimated_cost_currency"] != "USD" {
		t.Fatalf("expected estimated_cost_currency=USD, got %#v", usageSummaryBody["estimated_cost_currency"])
	}
	if usageSummaryBody["estimated_cost"].(float64) <= 0 {
		t.Fatalf("expected estimated_cost > 0, got %#v", usageSummaryBody["estimated_cost"])
	}

	usageSeriesRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/usage/timeseries?from="+from+"&to="+to+"&bucket=hour", ownerCookie)
	requireStatus(t, usageSeriesRec, http.StatusOK)
	var usageSeriesBody map[string]any
	decodeJSON(t, usageSeriesRec, &usageSeriesBody)
	if len(usageSeriesBody["items"].([]any)) < 1 {
		t.Fatalf("expected usage timeseries rows, got %#v", usageSeriesBody)
	}
	if usageSeriesBody["estimated_cost_currency"] != "USD" {
		t.Fatalf("expected usage timeseries estimated_cost_currency=USD, got %#v", usageSeriesBody["estimated_cost_currency"])
	}

	usageByTeamRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/usage/by-team?from="+from+"&to="+to, ownerCookie)
	requireStatus(t, usageByTeamRec, http.StatusOK)
	var usageByTeamBody map[string]any
	decodeJSON(t, usageByTeamRec, &usageByTeamBody)
	if len(usageByTeamBody["items"].([]any)) < 1 {
		t.Fatalf("expected usage by-team rows, got %#v", usageByTeamBody)
	}
	firstTeamItem := usageByTeamBody["items"].([]any)[0].(map[string]any)
	if firstTeamItem["estimated_cost"].(float64) <= 0 {
		t.Fatalf("expected by-team estimated_cost > 0, got %#v", firstTeamItem["estimated_cost"])
	}

	usageByModelRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/usage/by-model?from="+from+"&to="+to, ownerCookie)
	requireStatus(t, usageByModelRec, http.StatusOK)
	var usageByModelBody map[string]any
	decodeJSON(t, usageByModelRec, &usageByModelBody)
	if len(usageByModelBody["items"].([]any)) < 1 {
		t.Fatalf("expected usage by-model rows, got %#v", usageByModelBody)
	}
	firstModelItem := usageByModelBody["items"].([]any)[0].(map[string]any)
	if firstModelItem["estimated_cost"].(float64) <= 0 {
		t.Fatalf("expected by-model estimated_cost > 0, got %#v", firstModelItem["estimated_cost"])
	}

	eventsRec := performGETRequest(t, h, "/v1/control/orgs/"+orgID+"/events?limit=20", ownerCookie)
	requireStatus(t, eventsRec, http.StatusOK)
	var eventsBody map[string]any
	decodeJSON(t, eventsRec, &eventsBody)
	if len(eventsBody["items"].([]any)) < 1 {
		t.Fatalf("expected events rows, got %#v", eventsBody)
	}
}

func performGETRequest(t *testing.T, h http.Handler, path, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func strconvI(i int) string {
	return strconv.Itoa(i)
}
