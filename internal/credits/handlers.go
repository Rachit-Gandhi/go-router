package credits

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/auth"
	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/Rachit-Gandhi/go-router/internal/response"
	"github.com/google/uuid"
)

type CreditsHandler struct {
	Db  *database.Queries
	SQL *sql.DB
}

type topupRequest struct {
	Amount  int `json:"amount"`
	Credits int `json:"credits"`
}

type balanceResponse struct {
	Balance int `json:"balance"`
}

// GetBalanceHandler returns the current credit balance.
func (h *CreditsHandler) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey{}).(string)
	if !ok {
		response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", nil)
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", err)
		return
	}
	bal, err := h.Db.GetUserBalance(r.Context(), uid)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to fetch balance", err)
		return
	}
	_ = response.Wrap(w).WriteJSON(http.StatusOK, balanceResponse{Balance: int(bal)})
}

// MockTopupHandler simulates a payment and credits the user.
func (h *CreditsHandler) MockTopupHandler(w http.ResponseWriter, r *http.Request) {
	var req topupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "Failed to decode request body", err)
		return
	}
	if req.Amount <= 0 || req.Credits <= 0 {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "amount and credits must be > 0", nil)
		return
	}
	userID, ok := r.Context().Value(auth.UserIDKey{}).(string)
	if !ok {
		response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", nil)
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	tx, err := h.SQL.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to start transaction", err)
		return
	}
	defer tx.Rollback()

	qtx := h.Db.WithTx(tx)
	trx, err := qtx.CreateTransaction(r.Context(), database.CreateTransactionParams{
		UserID:  uid,
		Amt:     int32(req.Amount),
		Credits: int32(req.Credits),
		Status:  "completed",
	})
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to create transaction", err)
		return
	}
	_, err = qtx.AddUserCredits(r.Context(), database.AddUserCreditsParams{UserID: uid, BalanceCreditsInt: int32(req.Credits)})
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to update balance", err)
		return
	}
	if err := tx.Commit(); err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to commit transaction", err)
		return
	}
	_ = response.Wrap(w).WriteJSON(http.StatusCreated, trx)
}
