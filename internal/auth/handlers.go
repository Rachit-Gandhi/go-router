package auth

import (
	"encoding/json"
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/database"
)

type AuthHandler struct {
	Db *database.Queries
}

func (cfg *AuthHandler) SignupHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err = cfg.Db.CreateUser(r.Context(), user)
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
