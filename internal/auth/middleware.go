package auth

import (
	"context"
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/response"
	"github.com/google/uuid"
)

func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", nil)
			return
		}
		userId, err := h.validateToken(token)
		if err != nil {
			response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", err)
			return
		}
		parsedUserId, err := uuid.Parse(userId)
		if err != nil {
			response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", err)
			return
		}
		userFound, err := h.Db.GetUserByID(r.Context(), parsedUserId)
		if err != nil {
			response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", err)
			return
		}
		if userFound.UserID == uuid.Nil {
			response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Unauthorized", nil)
			return
		}
		ctx := context.WithValue(r.Context(), "userId", userId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
