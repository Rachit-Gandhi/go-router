package credits

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Rachit-Gandhi/go-router/internal/auth"
	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/google/uuid"
)

func setupMockDB(t *testing.T) (*database.Queries, *sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return database.New(db), db, mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
		_ = db.Close()
	}
}

func TestGetBalanceHandler(t *testing.T) {
	queries, db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	userID := uuid.New()
	mock.ExpectQuery("SELECT balance_credits_int FROM users").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance_credits_int"}).AddRow(100))

	req := httptest.NewRequest(http.MethodGet, "/credits", nil)
	req = req.WithContext(withUser(req.Context(), userID.String()))
	w := httptest.NewRecorder()

	h := CreditsHandler{Db: queries, SQL: db}
	h.GetBalanceHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestMockTopupHandler(t *testing.T) {
	queries, db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	userID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO razorpay_transactions").
		WillReturnRows(sqlmock.NewRows([]string{
			"transaction_id", "user_id", "amt", "credits", "status", "started_at", "completed_at",
		}).AddRow(uuid.New(), userID, 100, 1000, "completed", time.Now(), sql.NullTime{}))

	mock.ExpectQuery("UPDATE users").
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "username", "email", "password", "balance_credits_int",
		}).AddRow(userID, "u", "e", "p", 1000))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]int{"amount": 100, "credits": 1000})
	req := httptest.NewRequest(http.MethodPost, "/credits/topup", bytes.NewReader(body))
	req = req.WithContext(withUser(req.Context(), userID.String()))
	w := httptest.NewRecorder()

	h := CreditsHandler{Db: queries, SQL: db}
	h.MockTopupHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
}

func withUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, auth.UserIDKey{}, userID)
}
