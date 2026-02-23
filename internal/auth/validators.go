package auth

import (
	"fmt"
)

func validateSignupInput(username, email, password string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}
