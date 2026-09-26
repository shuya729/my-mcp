package repository

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUserRepository(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	repository := NewUserRepository(pool)
	authID := "repository-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DELETE FROM users WHERE auth_id = $1", authID)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})

	if _, err := repository.FindByAuthID(ctx, authID); err == nil {
		t.Fatalf("find before create: %v", err)
	}
	if _, err := repository.UpdateName(ctx, authID, "Bob"); err == nil {
		t.Fatalf("update before create: expected error")
	}
	if err := repository.DeleteByAuthID(ctx, authID); err == nil {
		t.Fatalf("delete before create: expected error")
	}
	created, err := repository.Create(ctx, authID, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(created.ID) != 36 || created.ID[14] != '7' || created.Name != "Alice" || created.AuthID != authID || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Errorf("created user = %+v", created)
	}
	if _, err := repository.Create(ctx, authID, "Duplicate"); err == nil {
		t.Errorf("duplicate create: %v", err)
	}
	found, err := repository.FindByAuthID(ctx, authID)
	if err != nil || found.ID != created.ID {
		t.Errorf("find after create = %+v, %v", found, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET updated_at = created_at - interval '1 day' WHERE auth_id = $1", authID); err != nil {
		t.Fatal(err)
	}
	updated, err := repository.UpdateName(ctx, authID, "Bob")
	if err != nil || updated.ID != created.ID || updated.Name != "Bob" || updated.UpdatedAt.Before(created.CreatedAt) {
		t.Errorf("update = %+v, %v", updated, err)
	}
	if err := repository.DeleteByAuthID(ctx, authID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindByAuthID(ctx, authID); err == nil {
		t.Errorf("find after delete: %v", err)
	}
	if err := repository.DeleteByAuthID(ctx, authID); err == nil {
		t.Errorf("delete after delete: expected error")
	}
}
