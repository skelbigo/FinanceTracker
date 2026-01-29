package auth

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type JWT interface {
	GenerateAccessToken(userID string) (string, error)
}

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidRefreshToken = errors.New("invalid refresh token")
var ErrInvalidResetToken = errors.New("invalid reset token")

type Service struct {
	repo             *Repo
	jwt              JWT
	refreshTTL       time.Duration
	resetTTL         time.Duration
	returnResetToken bool
}

func NewService(repo *Repo, jwt JWT, refreshTTL, resetTTL time.Duration, returnResetToken bool) *Service {
	return &Service{
		repo:             repo,
		jwt:              jwt,
		refreshTTL:       refreshTTL,
		resetTTL:         resetTTL,
		returnResetToken: returnResetToken,
	}
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("email is required")
	}

	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	plainToken, err := GenerateResetToken()
	if err != nil {
		return "", err
	}
	h := HashResetToken(plainToken)
	expiresAt := time.Now().Add(s.resetTTL)
	if err := s.repo.InsertPasswordResetToken(ctx, u.ID, h, expiresAt); err != nil {
		return "", err
	}

	if s.returnResetToken {
		return plainToken, nil
	}
	return "", nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	token = strings.TrimSpace(token)
	newPassword = strings.TrimSpace(newPassword)
	if token == "" {
		return errors.New("token is required")
	}
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	h := HashResetToken(token)
	userID, ok, err := s.repo.ConsumePasswordResetToken(ctx, h)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidResetToken
	}

	passHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateUserPassword(ctx, userID, passHash); err != nil {
		return err
	}
	return nil
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

	access, err := s.jwt.GenerateAccessToken(u.ID)
	if err != nil {
		return RegisterResponse{}, err
	}

	refreshPlain, err := GenerateRefreshToken()
	if err != nil {
		return RegisterResponse{}, err
	}
	refreshHash := HashRefreshToken(refreshPlain)

	expiresAt := time.Now().Add(s.refreshTTL)
	if err := s.repo.InsertRefreshToken(ctx, u.ID, refreshHash, expiresAt); err != nil {
		return RegisterResponse{}, err
	}

	dto := UserDTO{ID: u.ID, Email: u.Email, Name: u.Name}

	return RegisterResponse{
		TokenPair: TokenPair{
			AccessToken:  access,
			RefreshToken: refreshPlain,
		},
		User: dto,
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

	access, err := s.jwt.GenerateAccessToken(u.ID)
	if err != nil {
		return LoginResponse{}, err
	}

	refreshPlain, err := GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, err
	}
	refreshHash := HashRefreshToken(refreshPlain)
	expiresAt := time.Now().Add(s.refreshTTL)

	if err := s.repo.InsertRefreshToken(ctx, u.ID, refreshHash, expiresAt); err != nil {
		return LoginResponse{}, err
	}

	dto := UserDTO{ID: u.ID, Email: u.Email, Name: u.Name}

	return LoginResponse{
		TokenPair: TokenPair{
			AccessToken:  access,
			RefreshToken: refreshPlain,
		},
		User: dto,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (RefreshResponse, error) {
	plain := strings.TrimSpace(req.RefreshToken)
	if plain == "" {
		return RefreshResponse{}, errors.New("refresh token is required")
	}

	oldHash := HashRefreshToken(plain)

	newPlain, err := GenerateRefreshToken()
	if err != nil {
		return RefreshResponse{}, err
	}
	newHash := HashRefreshToken(newPlain)
	newExpires := time.Now().Add(s.refreshTTL)

	userID, ok, err := s.repo.RotateRefreshToken(ctx, oldHash, newHash, newExpires)
	if err != nil {
		return RefreshResponse{}, err
	}
	if !ok {
		return RefreshResponse{}, ErrInvalidRefreshToken
	}

	access, err := s.jwt.GenerateAccessToken(userID)
	if err != nil {
		return RefreshResponse{}, err
	}

	return RefreshResponse{
		AccessToken:  access,
		RefreshToken: newPlain,
	}, nil
}

func (s *Service) Logout(ctx context.Context, req LogoutRequest) error {
	plain := strings.TrimSpace(req.RefreshToken)
	if plain == "" {
		return errors.New("refresh token is required")
	}

	hash := HashRefreshToken(plain)

	_, _, err := s.repo.ConsumeRefreshToken(ctx, hash)
	return err
}

func (s *Service) Me(ctx context.Context, userID string) (UserDTO, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}

	dto := UserDTO{ID: u.ID, Email: u.Email, Name: u.Name}
	return dto, nil
}
