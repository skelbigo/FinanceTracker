package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

type JWT interface {
	GenerateAccessToken(userID string, email string) (string, error)
}

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	repo       *Repo
	jwt        JWT
	refreshTTL time.Duration
}

func NewService(repo *Repo, jwt JWT, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, jwt: jwt, refreshTTL: refreshTTL}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password
	nameTrim := strings.TrimSpace(req.Name)

	if email == "" {
		return RegisterResponse{}, errors.New("email is required")
	}
	if len(password) < 8 {
		return RegisterResponse{}, errors.New("password must be at least 8 characters")
	}

	var namePtr *string
	if nameTrim != "" {
		namePtr = &nameTrim
	}

	passHash, err := HashPassword(password)
	if err != nil {
		return RegisterResponse{}, err
	}

	u, err := s.repo.CreateUser(ctx, email, passHash, namePtr)
	if err != nil {
		return RegisterResponse{}, err
	}

	access, err := s.jwt.GenerateAccessToken(u.ID, u.Email)
	if err != nil {
		return RegisterResponse{}, err
	}

	refreshPlain, err := GenerateRefreshToken()
	if err != nil {
		return RegisterResponse{}, err
	}
	refreshHash := HashRefreshToken(refreshPlain)

	expiresAt := time.Now().Add(s.refreshTTL)
	if err := s.repo.InsertRefreshTokenTime(ctx, u.ID, refreshHash, expiresAt); err != nil {
		return RegisterResponse{}, err
	}

	dto := UserDTO{ID: u.ID, Email: u.Email}
	if u.Name != nil {
		dto.Name = *u.Name
	}

	return RegisterResponse{
		AccessToken:  access,
		RefreshToken: refreshPlain,
		User:         dto,
	}, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if email == "" {
		return LoginResponse{}, errors.New("email is required")
	}
	if password == "" {
		return LoginResponse{}, errors.New("password is required")
	}

	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return LoginResponse{}, err
	}

	if err := CheckPassword(u.PasswordHash, password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}

	_ = s.repo.RevokeExpiredRefreshTokens(ctx, u.ID)

	access, err := s.jwt.GenerateAccessToken(u.ID, u.Email)
	if err != nil {
		return LoginResponse{}, err
	}

	refreshPlain, err := GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, err
	}
	refreshHash := HashRefreshToken(refreshPlain)
	expiresAt := time.Now().Add(s.refreshTTL)

	if err := s.repo.InsertRefreshTokenTime(ctx, u.ID, refreshHash, expiresAt); err != nil {
		return LoginResponse{}, err
	}

	dto := UserDTO{ID: u.ID, Email: u.Email}
	if u.Name != nil {
		dto.Name = *u.Name
	}

	return LoginResponse{
		AccessToken:  access,
		RefreshToken: refreshPlain,
		User:         dto,
	}, nil
}
