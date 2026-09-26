package repository

import (
	"context"
	"errors"
	"fmt"

	"my-mcp/internal/domain"
	"my-mcp/internal/usecase"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return UserRepository{pool: pool}
}

func (r UserRepository) FindByAuthID(ctx context.Context, authID string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, name, auth_id, created_at, updated_at
		FROM users WHERE auth_id = $1`, authID,
	).Scan(&user.ID, &user.Name, &user.AuthID, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, usecase.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (r UserRepository) Create(ctx context.Context, authID, name string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (auth_id, name) VALUES ($1, $2)
		RETURNING id::text, name, auth_id, created_at, updated_at`, authID, name,
	).Scan(&user.ID, &user.Name, &user.AuthID, &user.CreatedAt, &user.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.User{}, usecase.ErrUserExists
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r UserRepository) UpdateName(ctx context.Context, authID, name string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		UPDATE users SET name = $2, updated_at = now()
		WHERE auth_id = $1
		RETURNING id::text, name, auth_id, created_at, updated_at`, authID, name,
	).Scan(&user.ID, &user.Name, &user.AuthID, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, usecase.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (r UserRepository) DeleteByAuthID(ctx context.Context, authID string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM users WHERE auth_id = $1`, authID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return usecase.ErrUserNotFound
	}
	return nil
}
