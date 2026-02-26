package apikeys

import (
	"encoding/json"
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/google/uuid"
)

type ApiKeysHandler struct {
	Db *database.Queries
}

func (h *ApiKeysHandler) CreateApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var newApiRequest requestNewApiKey
	err := json.NewDecoder(r.Body).Decode(&newApiRequest)
	if err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}
	apiKey := CreateApiKey()
	apiKeyHash, err := CreateApiKeyHash(apiKey)
	if err != nil {
		http.Error(w, "Failed to create api key hash", http.StatusInternalServerError)
		return
	}
	userId := r.Context().Value("userId").(string)
	requestNewApiKey := database.CreateApiKeyParams{
		Name:    newApiRequest.Name,
		KeyHash: apiKeyHash,
		UserID:  uuid.MustParse(userId),
	}
	newCreatedApiKey, err := h.Db.CreateApiKey(r.Context(), requestNewApiKey)
	if err != nil {
		http.Error(w, "Failed to create api key", http.StatusInternalServerError)
		return
	}
	returnedNewApiKey := newApiKey{
		ApiKeyName: newCreatedApiKey.Name,
		ApiKeyHash: newCreatedApiKey.KeyHash,
	}
	json.NewEncoder(w).Encode(returnedNewApiKey)
}
