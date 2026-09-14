package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
)

var (
	ErrInvalidInput       = errors.New("invalid login or password format")
	ErrLoginTaken         = errors.New("login already exists")
	ErrInvalidCredentials = errors.New("invalid user credentials")
)

type UserService struct {
	users  storage.UserRepository
	tokens *auth.TokenManager
}

func NewUserService(users storage.UserRepository, tokens *auth.TokenManager) *UserService {
	return &UserService{
		users:  users,
		tokens: tokens,
	}
}

func (s *UserService) Register(ctx context.Context, login, password string) (string, error) {
	if login == "" || password == "" {
		return "", ErrInvalidInput
	}

	passwordHash, err := auth.HashPassword(password)
	if errors.Is(err, auth.ErrPasswordTooLong) {
		return "", ErrInvalidInput
	}
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	userID, err := s.users.CreateUser(ctx, models.User{
		Login:        login,
		PasswordHash: passwordHash,
	})
	if errors.Is(err, storage.ErrLoginExists) {
		return "", ErrLoginTaken
	}
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Issue(userID)
	if err != nil {
		return "", fmt.Errorf("issue token: %w", err)
	}
	return token, nil
}

func (s *UserService) Login(ctx context.Context, login, password string) (string, error) {

	user, err := s.users.GetUserByLogin(ctx, login)
	if errors.Is(err, storage.ErrUserNotFound) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	err = auth.CheckPassword(user.PasswordHash, password)
	if errors.Is(err, auth.ErrInvalidPassword) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("check password: %w", err)
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return "", fmt.Errorf("issue token: %w", err)
	}

	return token, nil
}
