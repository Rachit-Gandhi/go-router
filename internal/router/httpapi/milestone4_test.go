package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
)

type modelPolicy struct {
	Provider string
	Model    string
	Allowed  bool
}

type routerFixture struct {
	OrgID          string
	TeamID         string
	OwnerUserID    string
	PlaintextKey   string
	StoredAPIKeyID string
}

func TestChatCompletionsContract(t *testing.T) {
	tr := newTestRouterHandler(t)
	fixture := seedRouterFixture(t, tr.db,
		[]modelPolicy{
			{Provider: "openai", Model: "gpt-4o-mini", Allowed: true},
			{Provider: "openai", Model: "gpt-4o", Allowed: true},
		},
		nil,
	)

	rec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "user", "content": "hello router"},
		},
	})
	requireStatus(t, rec, http.StatusOK)

	var body struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	decodeJSONResponse(t, rec, &body)

	if body.ID == "" || !strings.HasPrefix(body.ID, "chatcmpl_") {
		t.Fatalf("expected chat completion id with prefix chatcmpl_, got %q", body.ID)
	}
	if body.Object != "chat.completion" {
		t.Fatalf("expected object chat.completion, got %q", body.Object)
	}
	if body.Created <= 0 {
		t.Fatalf("expected positive created unix timestamp, got %d", body.Created)
	}
	if body.Model == "" {
		t.Fatal("expected response model to be populated")
	}
	if len(body.Choices) != 1 {
		t.Fatalf("expected a single choice, got %d", len(body.Choices))
	}
	if body.Choices[0].Message.Role != "assistant" {
		t.Fatalf("expected assistant role, got %q", body.Choices[0].Message.Role)
	}
	if strings.TrimSpace(body.Choices[0].Message.Content) == "" {
		t.Fatal("expected non-empty assistant content")
	}
	if body.Usage.PromptTokens < 1 || body.Usage.CompletionTokens < 1 {
		t.Fatalf("expected positive usage token counts, got prompt=%d completion=%d", body.Usage.PromptTokens, body.Usage.CompletionTokens)
	}
	if body.Usage.TotalTokens != body.Usage.PromptTokens+body.Usage.CompletionTokens {
		t.Fatalf("expected total_tokens=%d, got %d", body.Usage.PromptTokens+body.Usage.CompletionTokens, body.Usage.TotalTokens)
	}
}

func TestChatCompletionsPolicyDenialReturnsDeterministicForbidden(t *testing.T) {
	tr := newTestRouterHandler(t)
	fixture := seedRouterFixture(t, tr.db,
		[]modelPolicy{{Provider: "openai", Model: "gpt-4o-mini", Allowed: true}},
		[]modelPolicy{{Provider: "openai", Model: "gpt-4o-mini", Allowed: false}},
	)

	rec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "user", "content": "should fail by policy"},
		},
	})
	requireStatus(t, rec, http.StatusForbidden)

	var body map[string]any
	decodeJSONResponse(t, rec, &body)
	if got, _ := body["error"].(string); got != policyDeniedErrorMessage {
		t.Fatalf("expected deterministic policy denial error %q, got %#v", policyDeniedErrorMessage, body["error"])
	}
}

func TestChatCompletionsComplexityRoutingSelectsFastAndStrongModels(t *testing.T) {
	tr := newTestRouterHandler(t)
	fixture := seedRouterFixture(t, tr.db,
		[]modelPolicy{
			{Provider: "openai", Model: "gpt-4o-mini", Allowed: true},
			{Provider: "openai", Model: "gpt-4o", Allowed: true},
		},
		nil,
	)

	shortRec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "user", "content": "quick summary"},
		},
	})
	requireStatus(t, shortRec, http.StatusOK)
	var shortBody map[string]any
	decodeJSONResponse(t, shortRec, &shortBody)
	if shortBody["model"] != "gpt-4o-mini" {
		t.Fatalf("expected short prompt to route to fast model gpt-4o-mini, got %#v", shortBody["model"])
	}

	longRec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "user", "content": strings.Repeat("very long prompt ", 80)},
		},
	})
	requireStatus(t, longRec, http.StatusOK)
	var longBody map[string]any
	decodeJSONResponse(t, longRec, &longBody)
	if longBody["model"] != "gpt-4o" {
		t.Fatalf("expected long prompt to route to stronger model gpt-4o, got %#v", longBody["model"])
	}
}

func TestChatCompletionsAutoSkipsModelsWithoutConfiguredAdapters(t *testing.T) {
	tr := newTestRouterHandler(t)
	fixture := seedRouterFixture(t, tr.db,
		[]modelPolicy{
			{Provider: "mistral", Model: "mistral-small", Allowed: true},
			{Provider: "openai", Model: "gpt-4o", Allowed: true},
		},
		nil,
	)

	rec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "user", "content": "pick a model with configured adapter"},
		},
	})
	requireStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSONResponse(t, rec, &body)
	if body["model"] != "gpt-4o" {
		t.Fatalf("expected auto selection to skip unsupported provider and choose gpt-4o, got %#v", body["model"])
	}
}

func TestChatCompletionsExplicitUnsupportedProviderReturnsBadGateway(t *testing.T) {
	tr := newTestRouterHandler(t)
	fixture := seedRouterFixture(t, tr.db,
		[]modelPolicy{
			{Provider: "mistral", Model: "mistral-small", Allowed: true},
			{Provider: "openai", Model: "gpt-4o", Allowed: true},
		},
		nil,
	)

	rec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": "mistral-small",
		"messages": []map[string]any{
			{"role": "user", "content": "explicit model should keep provider and fail without adapter"},
		},
	})
	requireStatus(t, rec, http.StatusBadGateway)

	var body map[string]any
	decodeJSONResponse(t, rec, &body)
	if body["error"] != "provider adapter not configured" {
		t.Fatalf("expected explicit unsupported provider to return adapter error, got %#v", body["error"])
	}
}

func TestChatCompletionsRejectsInvalidAPIKey(t *testing.T) {
	tr := newTestRouterHandler(t)
	_ = seedRouterFixture(t, tr.db,
		[]modelPolicy{{Provider: "openai", Model: "gpt-4o-mini", Allowed: true}},
		nil,
	)

	rec := performRouterChatRequest(t, tr.handler, "not-a-real-key", map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	})
	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestChatCompletionsAllowsRequestedModelBeyondFirstPolicyPage(t *testing.T) {
	tr := newTestRouterHandler(t)

	orgPolicies := make([]modelPolicy, 0, 140)
	for i := 0; i < 140; i++ {
		orgPolicies = append(orgPolicies, modelPolicy{
			Provider: "openai",
			Model:    "model-" + strconv.Itoa(1000+i),
			Allowed:  true,
		})
	}
	requestedModel := orgPolicies[len(orgPolicies)-1].Model

	fixture := seedRouterFixture(t, tr.db, orgPolicies, nil)

	rec := performRouterChatRequest(t, tr.handler, fixture.PlaintextKey, map[string]any{
		"model": requestedModel,
		"messages": []map[string]any{
			{"role": "user", "content": "choose explicitly requested model"},
		},
	})
	requireStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSONResponse(t, rec, &body)
	if body["model"] != requestedModel {
		t.Fatalf("expected requested model %q, got %#v", requestedModel, body["model"])
	}
}

func seedRouterFixture(t *testing.T, db *sql.DB, orgPolicies, teamPolicies []modelPolicy) routerFixture {
	t.Helper()

	q := dbquery.New(db)
	ctx := t.Context()

	ownerUserID := "usr_owner"
	orgID := "org_router"
	teamID := "team_router"
	plaintextKey := "router-fixture-token"
	apiKeyID := "ukey_router_owner"

	if _, err := q.CreateUser(ctx, dbquery.CreateUserParams{ID: ownerUserID, Email: "owner-router@example.com", Name: "Owner Router"}); err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	if _, err := q.CreateOrg(ctx, dbquery.CreateOrgParams{ID: orgID, Name: "Router Org", OwnerUserID: ownerUserID}); err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := q.UpsertOrgMembership(ctx, dbquery.UpsertOrgMembershipParams{OrgID: orgID, UserID: ownerUserID, Role: "org_owner"}); err != nil {
		t.Fatalf("create owner membership: %v", err)
	}
	if _, err := q.CreateTeam(ctx, dbquery.CreateTeamParams{ID: teamID, OrgID: orgID, Name: "router-team", ProfileJsonb: nil, RateLimitPerMinute: sql.NullInt32{}}); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if _, err := q.UpsertTeamMembership(ctx, dbquery.UpsertTeamMembershipParams{OrgID: orgID, TeamID: teamID, UserID: ownerUserID}); err != nil {
		t.Fatalf("create team membership: %v", err)
	}

	if _, err := q.CreateUserTeamAPIKey(ctx, dbquery.CreateUserTeamAPIKeyParams{
		ID:        apiKeyID,
		OrgID:     orgID,
		TeamID:    teamID,
		UserID:    ownerUserID,
		KeyHash:   hashValue(plaintextKey),
		KeyPrefix: plaintextKey[:8],
	}); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	for _, policy := range orgPolicies {
		if _, err := q.UpsertOrgModelPolicy(ctx, dbquery.UpsertOrgModelPolicyParams{
			OrgID:     orgID,
			Provider:  policy.Provider,
			Model:     policy.Model,
			IsAllowed: policy.Allowed,
		}); err != nil {
			t.Fatalf("upsert org model policy (%s/%s): %v", policy.Provider, policy.Model, err)
		}
	}

	for _, policy := range teamPolicies {
		if _, err := q.UpsertTeamModelPolicy(ctx, dbquery.UpsertTeamModelPolicyParams{
			OrgID:     orgID,
			TeamID:    teamID,
			Provider:  policy.Provider,
			Model:     policy.Model,
			IsAllowed: policy.Allowed,
		}); err != nil {
			t.Fatalf("upsert team model policy (%s/%s): %v", policy.Provider, policy.Model, err)
		}
	}

	return routerFixture{
		OrgID:          orgID,
		TeamID:         teamID,
		OwnerUserID:    ownerUserID,
		PlaintextKey:   plaintextKey,
		StoredAPIKeyID: apiKeyID,
	}
}

func performRouterChatRequest(t *testing.T, h http.Handler, apiKey string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/router/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, out any) {
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
