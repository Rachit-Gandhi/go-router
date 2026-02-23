package auth

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
