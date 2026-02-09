package auth

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/stringsx"
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
	bcryptCost       int
}

func NewService(repo *Repo, jwt JWT, refreshTTL, resetTTL time.Duration, returnResetToken bool, bcryptCost int) *Service {
	return &Service{
		repo:             repo,
		jwt:              jwt,
		refreshTTL:       refreshTTL,
		resetTTL:         resetTTL,
		returnResetToken: returnResetToken,
		bcryptCost:       bcryptCost,
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
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}

	h := HashResetToken(token)
	userID, ok, err := s.repo.ConsumePasswordResetToken(ctx, h)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidResetToken
	}

	passHash, err := HashPassword(newPassword, s.bcryptCost)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateUserPassword(ctx, userID, passHash); err != nil {
		return err
	}
	return nil
}

func (s *Service) Register(ctx context.Context, req RegisterRequest, meta TokenMeta) (RegisterResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password
	nameTrim := stringsx.TrimStrip(req.Name)

	if email == "" {
		return RegisterResponse{}, errors.New("email is required")
	}
	if err := ValidatePasswordPolicy(password); err != nil {
		return RegisterResponse{}, err
	}

	var namePtr *string
	if nameTrim != "" {
		if utf8.RuneCountInString(nameTrim) > 64 {
			return RegisterResponse{}, ErrInvalidName
		}
		namePtr = &nameTrim
	}

	passHash, err := HashPassword(password, s.bcryptCost)
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
	if _, err := s.repo.InsertRefreshToken(ctx, u.ID, refreshHash, expiresAt, meta); err != nil {
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

func (s *Service) Login(ctx context.Context, req LoginRequest, meta TokenMeta) (LoginResponse, error) {
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

	if _, err := s.repo.InsertRefreshToken(ctx, u.ID, refreshHash, expiresAt, meta); err != nil {
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

func (s *Service) Refresh(ctx context.Context, req RefreshRequest, meta TokenMeta) (RefreshResponse, error) {
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

	userID, ok, reused, err := s.repo.RotateRefreshToken(ctx, oldHash, newHash, newExpires, meta)
	if err != nil {
		return RefreshResponse{}, err
	}
	if reused {
		return RefreshResponse{}, ErrInvalidRefreshToken
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

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	return s.repo.RevokeAllRefreshTokensForUser(ctx, userID)
}

func (s *Service) Me(ctx context.Context, userID string) (UserDTO, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}

	dto := UserDTO{ID: u.ID, Email: u.Email, Name: u.Name}
	return dto, nil
}
