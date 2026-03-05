package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/Rachit-Gandhi/go-router/internal/response"
	"github.com/google/uuid"
)

// SignupHandler registers a new user.
func (h *AuthHandler) SignupHandler(w http.ResponseWriter, r *http.Request) {
	var newUser requestNewUser
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&newUser)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "Failed to decode request body", err)
		return
	}
	err = validateSignupInput(newUser.Username, newUser.Email, newUser.Password)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, err.Error(), err)
		return
	}
	prexistingUser, err := h.Db.GetUserByEmail(r.Context(), newUser.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to check existing user", err)
		return
	}
	if err == nil && prexistingUser.UserID != uuid.Nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "User already exists", nil)
		return
	}
	hashedPassword, err := hashPassword(newUser.Password)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to hash password", err)
		return
	}
	user := database.CreateUserParams{
		Username: newUser.Username,
		Email:    newUser.Email,
		Password: hashedPassword,
	}
	_, err = h.Db.CreateUser(r.Context(), user)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to create user", err)
		return
	}

	newCreatedUser := createdNewUser{
		Message:  "User created successfully",
		Username: user.Username,
		Email:    user.Email,
	}
	if err := response.Wrap(w).WriteJSON(http.StatusCreated, newCreatedUser); err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to marshal user data", err)
		return
	}
}

// LoginHandler authenticates a user and sets a token cookie.
func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var loginUser requestLoginUser
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&loginUser)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, "Failed to decode request body", err)
		return
	}
	err = validateLoginInput(loginUser.Email, loginUser.Password)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusBadRequest, err.Error(), err)
		return
	}
	user, err := h.Db.GetUserByEmail(r.Context(), loginUser.Email)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusNotFound, "User not found", err)
		return
	}

	valid, err := verifyPassword(loginUser.Password, user.Password)
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to verify password", err)
		return
	}
	if !valid {
		response.WriteError(response.Wrap(w), http.StatusUnauthorized, "Invalid password", nil)
		return
	}
	token, err := h.makeToken(user.UserID.String())
	if err != nil {
		response.WriteError(response.Wrap(w), http.StatusInternalServerError, "Failed to make token", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(time.Duration(h.TokenExpiry) * time.Hour),
		HttpOnly: true,
		Secure:   true,
	})
	_ = response.Wrap(w).WriteJSON(http.StatusOK, map[string]string{"message": "Login successful"})
}
