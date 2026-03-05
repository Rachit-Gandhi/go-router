package apikeys

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/google/uuid"
)

func setupMockDB(t *testing.T) (*database.Queries, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return database.New(db), mock, func() { db.Close() }
}

func TestCreateApiKeyHandler(t *testing.T) {
	queries, mock, cleanup := setupMockDB(t)
	defer cleanup()

	userID := uuid.New()
	mock.ExpectQuery("INSERT INTO api_keys").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "key_hash", "disabled", "deleted", "last_used_at", "disabled_at", "deleted_at", "api_key_show_string",
		}).AddRow(uuid.New(), userID, "test", "hash", false, false, sql.NullTime{}, sql.NullTime{}, sql.NullTime{}, "go-**"))

	reqBody, _ := json.Marshal(map[string]string{"name": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api-keys", bytes.NewReader(reqBody))
	req = req.WithContext(withUser(req.Context(), userID.String()))
	w := httptest.NewRecorder()

	h := ApiKeysHandler{Db: queries}
	h.CreateApiKeyHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
}

func TestGetApiKeysHandler(t *testing.T) {
	queries, mock, cleanup := setupMockDB(t)
	defer cleanup()

	userID := uuid.New()
	mock.ExpectQuery("SELECT name,api_key_show_string").
		WillReturnRows(sqlmock.NewRows([]string{
			"name", "api_key_show_string", "disabled", "deleted", "last_used_at", "disabled_at", "deleted_at",
		}).AddRow("k1", "go-**", false, false, sql.NullTime{}, sql.NullTime{}, sql.NullTime{}))

	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	req = req.WithContext(withUser(req.Context(), userID.String()))
	w := httptest.NewRecorder()

	h := ApiKeysHandler{Db: queries}
	h.GetApiKeysHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestRevokeApiKeyHandler(t *testing.T) {
	queries, mock, cleanup := setupMockDB(t)
	defer cleanup()

	userID := uuid.New()
	apiID := uuid.New()
	mock.ExpectQuery("UPDATE api_keys").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "key_hash", "disabled", "deleted", "last_used_at", "disabled_at", "deleted_at", "api_key_show_string",
		}).AddRow(apiID, userID, "test", "hash", true, false, sql.NullTime{}, sql.NullTime{}, sql.NullTime{}, "go-**"))

	reqBody, _ := json.Marshal(map[string]string{"api_key_id": apiID.String()})
	req := httptest.NewRequest(http.MethodPatch, "/api-keys/revoke", bytes.NewReader(reqBody))
	req = req.WithContext(withUser(req.Context(), userID.String()))
	w := httptest.NewRecorder()

	h := ApiKeysHandler{Db: queries}
	h.RevokeApiKeyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestDeleteApiKeyHandler(t *testing.T) {
	queries, mock, cleanup := setupMockDB(t)
	defer cleanup()

	userID := uuid.New()
	apiID := uuid.New()
	mock.ExpectQuery("UPDATE api_keys").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "name", "key_hash", "disabled", "deleted", "last_used_at", "disabled_at", "deleted_at", "api_key_show_string",
		}).AddRow(apiID, userID, "test", "hash", false, true, sql.NullTime{}, sql.NullTime{}, sql.NullTime{}, "go-**"))

	reqBody, _ := json.Marshal(map[string]string{"api_key_id": apiID.String()})
	req := httptest.NewRequest(http.MethodDelete, "/api-keys", bytes.NewReader(reqBody))
	req = req.WithContext(withUser(req.Context(), userID.String()))
	w := httptest.NewRecorder()

	h := ApiKeysHandler{Db: queries}
	h.DeleteApiKeyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func withUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "userId", userID)
}
