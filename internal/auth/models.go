package auth

import "github.com/Rachit-Gandhi/go-router/internal/database"

type createdNewUser struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type requestNewUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type requestLoginUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthHandler contains dependencies for auth endpoints.
type AuthHandler struct {
	Db          *database.Queries
	JwtSecret   string
	TokenExpiry int
}
