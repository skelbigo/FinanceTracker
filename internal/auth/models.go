package auth

import "time"

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name"`
}

type UserDTO struct {
	ID    string  `json:"id"`
	Email string  `json:"email"`
	Name  *string `json:"name,omitempty"`
}

type RegisterResponse struct {
	TokenPair
	User UserDTO `json:"user"`
}

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         *string
	CreatedAt    time.Time
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	TokenPair
	User UserDTO `json:"user"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse = TokenPair

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}
