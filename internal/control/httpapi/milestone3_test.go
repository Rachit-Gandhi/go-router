package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"testing"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
)

func TestCreateAPIKeyReturnsPlaintextOnceAndStoresHash(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler
	queries := dbquery.New(tc.db)

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, tc)
	teamID := createTeam(t, h, orgID, ownerCookie, "key-team")
	memberUserID := addMemberByEmail(t, h, orgID, teamID, ownerCookie, "member@example.com", "Key Member", "")

	createKeyRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/users/"+memberUserID+"/api-keys", map[string]any{}, ownerCookie)
	if createKeyRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createKeyRec.Code)
	}
	var createKeyBody map[string]any
	decodeJSON(t, createKeyRec, &createKeyBody)

	keyID, _ := createKeyBody["id"].(string)
	apiKey, _ := createKeyBody["api_key"].(string)
	keyPrefix, _ := createKeyBody["key_prefix"].(string)
	if keyID == "" || apiKey == "" || keyPrefix == "" {
		t.Fatalf("expected id/api_key/key_prefix in response, got %#v", createKeyBody)
	}

	hashed := sha256.Sum256([]byte(apiKey))
	lookup, err := queries.GetActiveUserTeamAPIKeyByHash(t.Context(), hex.EncodeToString(hashed[:]))
	if err != nil {
		t.Fatalf("expected key hash lookup to work: %v", err)
	}
	if lookup.ID != keyID {
		t.Fatalf("expected persisted key id %q, got %q", keyID, lookup.ID)
	}
	if lookup.KeyPrefix != keyPrefix {
		t.Fatalf("expected persisted key prefix %q, got %q", keyPrefix, lookup.KeyPrefix)
	}
}

func TestRevokeAPIKeyDisablesIdentityLookup(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler
	queries := dbquery.New(tc.db)

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, tc)
	teamID := createTeam(t, h, orgID, ownerCookie, "revoke-team")
	memberUserID := addMemberByEmail(t, h, orgID, teamID, ownerCookie, "member2@example.com", "Revoke Member", "")

	createKeyRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/users/"+memberUserID+"/api-keys", map[string]any{}, ownerCookie)
	if createKeyRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createKeyRec.Code)
	}
	var createKeyBody map[string]any
	decodeJSON(t, createKeyRec, &createKeyBody)
	keyID, _ := createKeyBody["id"].(string)
	apiKey, _ := createKeyBody["api_key"].(string)

	hashed := sha256.Sum256([]byte(apiKey))
	hashText := hex.EncodeToString(hashed[:])
	if _, err := queries.ResolveIdentityByAPIKeyHash(t.Context(), hashText); err != nil {
		t.Fatalf("expected active key lookup before revoke: %v", err)
	}

	revokeRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/api-keys/"+keyID+"/revoke", map[string]any{}, ownerCookie)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, revokeRec.Code)
	}
	if _, err := queries.ResolveIdentityByAPIKeyHash(t.Context(), hashText); err == nil || err != sql.ErrNoRows {
		t.Fatalf("expected no rows for revoked key lookup, got %v", err)
	}
}

func TestCreateAPIKeyReturnsConflictWhenActiveKeyExists(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, tc)
	teamID := createTeam(t, h, orgID, ownerCookie, "duplicate-key-team")
	memberUserID := addMemberByEmail(t, h, orgID, teamID, ownerCookie, "dup-key@example.com", "Duplicate Key Member", "")

	firstCreateRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/users/"+memberUserID+"/api-keys", map[string]any{}, ownerCookie)
	if firstCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, firstCreateRec.Code)
	}

	secondCreateRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/users/"+memberUserID+"/api-keys", map[string]any{}, ownerCookie)
	if secondCreateRec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, secondCreateRec.Code)
	}
}

func TestProviderKeyStoredEncryptedAndTeamPolicyReduceOnly(t *testing.T) {
	tc := newTestHandler(t)
	h := tc.handler
	queries := dbquery.New(tc.db)

	orgID, _, ownerCookie := signupAndAuthenticateOwner(t, tc)
	teamID := createTeam(t, h, orgID, ownerCookie, "policy-team")

	const providerSecret = "sk-provider-secret-value"
	createProviderKeyRec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/providers/openai/keys", map[string]any{
		"api_key":    providerSecret,
		"key_kek_id": "kek-v1",
	}, ownerCookie)
	if createProviderKeyRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createProviderKeyRec.Code)
	}
	var providerBody map[string]any
	decodeJSON(t, createProviderKeyRec, &providerBody)
	providerKeyID, _ := providerBody["id"].(string)
	if providerKeyID == "" {
		t.Fatalf("expected provider key id in response, got %#v", providerBody)
	}
	if _, has := providerBody["api_key"]; has {
		t.Fatalf("did not expect plaintext api_key in provider key response: %#v", providerBody)
	}

	storedProviderKey, err := queries.GetOrgProviderKeyByID(t.Context(), providerKeyID)
	if err != nil {
		t.Fatalf("expected provider key row lookup: %v", err)
	}
	if len(storedProviderKey.KeyCiphertext) == 0 || len(storedProviderKey.KeyNonce) == 0 {
		t.Fatalf("expected ciphertext and nonce to be persisted: %#v", storedProviderKey)
	}
	if string(storedProviderKey.KeyCiphertext) == providerSecret {
		t.Fatalf("expected encrypted storage, ciphertext should not equal plaintext")
	}

	upsertOrgPolicyRec := performJSONRequest(t, h, http.MethodPut, "/v1/control/orgs/"+orgID+"/policies/models", map[string]any{
		"entries": []map[string]any{
			{"provider": "openai", "model": "gpt-4o", "is_allowed": true},
		},
	}, ownerCookie)
	if upsertOrgPolicyRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, upsertOrgPolicyRec.Code)
	}

	reduceAllowedRec := performJSONRequest(t, h, http.MethodPut, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/policies/models", map[string]any{
		"entries": []map[string]any{
			{"provider": "openai", "model": "gpt-4o", "is_allowed": false},
		},
	}, ownerCookie)
	if reduceAllowedRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, reduceAllowedRec.Code)
	}

	reduceViolationRec := performJSONRequest(t, h, http.MethodPut, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/policies/models", map[string]any{
		"entries": []map[string]any{
			{"provider": "openai", "model": "gpt-5", "is_allowed": true},
		},
	}, ownerCookie)
	if reduceViolationRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, reduceViolationRec.Code)
	}
}

func addMemberByEmail(t *testing.T, h http.Handler, orgID, teamID, cookie, email, name, role string) string {
	t.Helper()

	payload := map[string]any{
		"email": email,
		"name":  name,
	}
	if role != "" {
		payload["role"] = role
	}
	rec := performJSONRequest(t, h, http.MethodPost, "/v1/control/orgs/"+orgID+"/teams/"+teamID+"/members", payload, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var body map[string]any
	decodeJSON(t, rec, &body)
	userID, _ := body["user_id"].(string)
	if userID == "" {
		t.Fatalf("expected user_id in add member response, got %#v", body)
	}
	return userID
}
