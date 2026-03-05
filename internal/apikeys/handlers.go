package apikeys

import (
	"encoding/json"
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/Rachit-Gandhi/go-router/internal/response"
	"github.com/google/uuid"
)

type ApiKeysHandler struct {
	Db *database.Queries
}

func (h *ApiKeysHandler) CreateApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var newApiRequest requestNewApiKey
	err := json.NewDecoder(r.Body).Decode(&newApiRequest)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "Failed to decode request body", err)
		return
	}
	apiKey, apiKeyShowString := CreateApiKey()
	apiKeyHash, err := CreateApiKeyHash(apiKey)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to create api key hash", err)
		return
	}
	userId := r.Context().Value("userId").(string)
	requestNewApiKey := database.CreateApiKeyParams{
		Name:             newApiRequest.Name,
		KeyHash:          apiKeyHash,
		UserID:           uuid.MustParse(userId),
		ApiKeyShowString: apiKeyShowString,
	}
	newCreatedApiKey, err := h.Db.CreateApiKey(r.Context(), requestNewApiKey)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to create api key", err)
		return
	}
	returnedNewApiKey := newApiKey{
		ApiKeyName: newCreatedApiKey.Name,
		ApiKeyHash: newCreatedApiKey.KeyHash,
	}
	_ = response.Wrap(w).WriteJSON(http.StatusCreated, returnedNewApiKey)
}

func (h *ApiKeysHandler) GetApiKeysHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value("userId").(string)
	apiKeys, err := h.Db.GetApiKeys(r.Context(), uuid.MustParse(userId))
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to get api keys", err)
		return
	}
	_ = response.Wrap(w).WriteJSON(http.StatusOK, apiKeys)
}

func (h *ApiKeysHandler) RevokeApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var req apiKeyAction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "Failed to decode request body", err)
		return
	}
	if req.ApiKeyID == "" {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "api_key_id is required", nil)
		return
	}
	userId := r.Context().Value("userId").(string)
	apiKeyID := uuid.MustParse(req.ApiKeyID)
	updated, err := h.Db.RevokeApiKey(r.Context(), database.RevokeApiKeyParams{ID: apiKeyID, UserID: uuid.MustParse(userId)})
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to revoke api key", err)
		return
	}
	_ = response.Wrap(w).WriteJSON(http.StatusOK, updated)
}

func (h *ApiKeysHandler) DeleteApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var req apiKeyAction
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "Failed to decode request body", err)
		return
	}
	if req.ApiKeyID == "" {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "api_key_id is required", nil)
		return
	}
	userId := r.Context().Value("userId").(string)
	apiKeyID := uuid.MustParse(req.ApiKeyID)
	updated, err := h.Db.DeleteApiKey(r.Context(), database.DeleteApiKeyParams{ID: apiKeyID, UserID: uuid.MustParse(userId)})
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to delete api key", err)
		return
	}
	_ = response.Wrap(w).WriteJSON(http.StatusOK, updated)
}
