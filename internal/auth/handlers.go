package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Rachit-Gandhi/go-router/internal/database"
)

func (h *AuthHandler) SignupHandler(w http.ResponseWriter, r *http.Request) {
	var newUser requestNewUser
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&newUser)
	if err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}
	err = validateSignupInput(newUser.Username, newUser.Email, newUser.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hashedPassword, err := hashPassword(newUser.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	user := database.CreateUserParams{
		Username: newUser.Username,
		Email:    newUser.Email,
		Password: hashedPassword,
	}
	_, err = h.Db.CreateUser(r.Context(), user)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	newCreatedUser := createdNewUser{
		Message:  "User created successfully",
		Username: user.Username,
		Email:    user.Email,
	}
	data, err := json.Marshal(newCreatedUser)
	if err != nil {
		http.Error(w, "Failed to marshal user data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var loginUser requestLoginUser
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&loginUser)
	if err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}
	err = validateLoginInput(loginUser.Email, loginUser.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := h.Db.GetUserByEmail(r.Context(), loginUser.Email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	valid, err := verifyPassword(loginUser.Password, user.Password)
	if err != nil {
		http.Error(w, "Failed to verify password", http.StatusInternalServerError)
		return
	}
	if !valid {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}
	token, err := h.makeToken(user.UserID.String())
	if err != nil {
		http.Error(w, "Failed to make token", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(time.Duration(h.TokenExpiry) * time.Hour),
		HttpOnly: true,
		Secure:   true,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Login successful"))
}
