package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/google/uuid"
)

func setupMockDB(t *testing.T) (*database.Queries, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return database.New(db), mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
		_ = db.Close()
	}
}

func TestSignupHandler(t *testing.T) {
	queries, mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT user_id, username, email, password, balance_credits_int FROM users").
		WithArgs("rachit@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "password", "balance_credits_int"}))

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("rachit", "rachit@example.com", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "password", "balance_credits_int"}).
			AddRow(uuid.New(), "rachit", "rachit@example.com", "hash", 0))

	body, _ := json.Marshal(map[string]string{"username": "rachit", "email": "rachit@example.com", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h := AuthHandler{Db: queries}
	h.SignupHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
}

func TestLoginHandler(t *testing.T) {
	queries, mock, cleanup := setupMockDB(t)
	defer cleanup()

	os.Setenv("JWT_SECRET", "testsecret")
	defer os.Unsetenv("JWT_SECRET")

	passwordHash, _ := hashPassword("password123")
	mock.ExpectQuery("SELECT user_id, username, email, password, balance_credits_int FROM users").
		WithArgs("rachit@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "password", "balance_credits_int"}).
			AddRow(uuid.New(), "rachit", "rachit@example.com", passwordHash, 0))

	body, _ := json.Marshal(map[string]string{"email": "rachit@example.com", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h := AuthHandler{Db: queries, TokenExpiry: 1}
	h.LoginHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func withUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey{}, userID)
}
