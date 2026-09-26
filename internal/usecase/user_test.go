package usecase

import (
	"context"
	"errors"
	"testing"

	"my-mcp/internal/domain"
)

type recordingRepository struct {
	method string
	authID string
	name   string
	err    error
}

func (r *recordingRepository) FindByAuthID(_ context.Context, authID string) (domain.User, error) {
	r.method, r.authID = "get", authID
	return domain.User{ID: "user-id", AuthID: authID}, r.err
}

func (r *recordingRepository) Create(_ context.Context, authID, name string) (domain.User, error) {
	r.method, r.authID, r.name = "create", authID, name
	return domain.User{ID: "user-id", Name: name, AuthID: authID}, r.err
}

func (r *recordingRepository) UpdateName(_ context.Context, authID, name string) (domain.User, error) {
	r.method, r.authID, r.name = "update", authID, name
	return domain.User{ID: "user-id", Name: name, AuthID: authID}, r.err
}

func (r *recordingRepository) DeleteByAuthID(_ context.Context, authID string) error {
	r.method, r.authID = "delete", authID
	return r.err
}

func TestUsers(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		input      string
		repoErr    error
		wantMethod string
		wantName   string
		wantErr    bool
	}{
		{name: "get", operation: "get", wantMethod: "get"},
		{name: "get repository error", operation: "get", repoErr: errors.New("repository failure"), wantMethod: "get", wantErr: true},
		{name: "create trims name", operation: "create", input: " Alice ", wantMethod: "create", wantName: "Alice"},
		{name: "create rejects blank name", operation: "create", input: " \t ", wantErr: true},
		{name: "create repository error", operation: "create", input: "Alice", repoErr: errors.New("repository failure"), wantMethod: "create", wantName: "Alice", wantErr: true},
		{name: "update trims name", operation: "update", input: " Bob ", wantMethod: "update", wantName: "Bob"},
		{name: "update rejects blank name", operation: "update", input: " \t ", wantErr: true},
		{name: "update repository error", operation: "update", input: "Bob", repoErr: errors.New("repository failure"), wantMethod: "update", wantName: "Bob", wantErr: true},
		{name: "delete", operation: "delete", wantMethod: "delete"},
		{name: "delete repository error", operation: "delete", repoErr: errors.New("repository failure"), wantMethod: "delete", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &recordingRepository{err: tt.repoErr}
			users := NewUsers(repository)
			var user domain.User
			var err error
			switch tt.operation {
			case "get":
				user, err = users.Get(context.Background(), "logto-sub")
			case "create":
				user, err = users.Create(context.Background(), "logto-sub", tt.input)
			case "update":
				user, err = users.Update(context.Background(), "logto-sub", tt.input)
			case "delete":
				err = users.Delete(context.Background(), "logto-sub")
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if repository.method != tt.wantMethod {
				t.Errorf("repository method = %q, want %q", repository.method, tt.wantMethod)
			}
			if tt.wantMethod != "" {
				if repository.authID != "logto-sub" {
					t.Errorf("repository authID = %q", repository.authID)
				}
				if repository.name != tt.wantName {
					t.Errorf("repository name = %q, want %q", repository.name, tt.wantName)
				}
			}
			if err == nil && tt.operation != "delete" && (user.ID != "user-id" || user.AuthID != "logto-sub" || user.Name != tt.wantName) {
				t.Errorf("user = %+v", user)
			}
		})
	}
}
