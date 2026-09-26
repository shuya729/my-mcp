package usecase

import (
	"context"
	"errors"
	"strings"

	"my-mcp/internal/domain"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidName  = errors.New("name is required")
)

type UserRepository interface {
	FindByAuthID(context.Context, string) (domain.User, error)
	Create(context.Context, string, string) (domain.User, error)
	UpdateName(context.Context, string, string) (domain.User, error)
	DeleteByAuthID(context.Context, string) error
}

type Users struct {
	repository UserRepository
}

func NewUsers(repository UserRepository) Users {
	return Users{repository: repository}
}

func (u Users) Get(ctx context.Context, authID string) (domain.User, error) {
	return u.repository.FindByAuthID(ctx, authID)
}

func (u Users) Create(ctx context.Context, authID, name string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.User{}, ErrInvalidName
	}
	return u.repository.Create(ctx, authID, name)
}

func (u Users) Update(ctx context.Context, authID, name string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.User{}, ErrInvalidName
	}
	return u.repository.UpdateName(ctx, authID, name)
}

func (u Users) Delete(ctx context.Context, authID string) error {
	return u.repository.DeleteByAuthID(ctx, authID)
}
