package mcptools

import (
	"context"
	"errors"
	"testing"

	"my-mcp/internal/domain"
	"my-mcp/internal/usecase"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeUsers struct {
	method string
	authID string
	name   string
	err    error
}

func (f *fakeUsers) Get(_ context.Context, authID string) (domain.User, error) {
	f.method, f.authID = "get", authID
	return domain.User{ID: "user-id", Name: "Alice"}, f.err
}

func (f *fakeUsers) Create(_ context.Context, authID, name string) (domain.User, error) {
	f.method, f.authID, f.name = "create", authID, name
	return domain.User{ID: "user-id", Name: name}, f.err
}

func (f *fakeUsers) Update(_ context.Context, authID, name string) (domain.User, error) {
	f.method, f.authID, f.name = "update", authID, name
	return domain.User{ID: "user-id", Name: name}, f.err
}

func (f *fakeUsers) Delete(_ context.Context, authID string) error {
	f.method, f.authID = "delete", authID
	return f.err
}

func TestUserTools(t *testing.T) {
	read := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{UserID: "logto-sub", Scopes: []string{ReadScope}}}}
	write := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{UserID: "logto-sub", Scopes: []string{WriteScope}}}}
	missingSubject := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{Scopes: []string{ReadScope}}}}
	tests := []struct {
		name       string
		operation  string
		request    *mcp.CallToolRequest
		input      string
		serviceErr error
		wantMethod string
		wantOutput UserOutput
		wantErr    bool
	}{
		{name: "get", operation: "get", request: read, wantMethod: "get", wantOutput: UserOutput{ID: "user-id", Name: "Alice"}},
		{name: "get not found", operation: "get", request: read, serviceErr: usecase.ErrUserNotFound, wantMethod: "get", wantErr: true},
		{name: "get internal error", operation: "get", request: read, serviceErr: errors.New("database failure"), wantMethod: "get", wantErr: true},
		{name: "get needs read scope", operation: "get", request: write, wantErr: true},
		{name: "get needs identity", operation: "get", request: missingSubject, wantErr: true},
		{name: "get needs token info", operation: "get", request: &mcp.CallToolRequest{}, wantErr: true},
		{name: "create", operation: "create", request: write, input: "Bob", wantMethod: "create", wantOutput: UserOutput{ID: "user-id", Name: "Bob"}},
		{name: "create duplicate", operation: "create", request: write, input: "Bob", serviceErr: usecase.ErrUserExists, wantMethod: "create", wantErr: true},
		{name: "create invalid name", operation: "create", request: write, serviceErr: usecase.ErrInvalidName, wantMethod: "create", wantErr: true},
		{name: "create needs write scope", operation: "create", request: read, input: "Bob", wantErr: true},
		{name: "update", operation: "update", request: write, input: "Carol", wantMethod: "update", wantOutput: UserOutput{ID: "user-id", Name: "Carol"}},
		{name: "update not found", operation: "update", request: write, input: "Carol", serviceErr: usecase.ErrUserNotFound, wantMethod: "update", wantErr: true},
		{name: "update invalid name", operation: "update", request: write, serviceErr: usecase.ErrInvalidName, wantMethod: "update", wantErr: true},
		{name: "update needs write scope", operation: "update", request: read, input: "Carol", wantErr: true},
		{name: "delete", operation: "delete", request: write, wantMethod: "delete"},
		{name: "delete not found", operation: "delete", request: write, serviceErr: usecase.ErrUserNotFound, wantMethod: "delete", wantErr: true},
		{name: "delete needs write scope", operation: "delete", request: read, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUsers{err: tt.serviceErr}
			tools := UserTools{Users: users}
			var output UserOutput
			var err error
			switch tt.operation {
			case "get":
				_, output, err = tools.GetUser(context.Background(), tt.request, struct{}{})
			case "create":
				_, output, err = tools.CreateUser(context.Background(), tt.request, NameInput{Name: tt.input})
			case "update":
				_, output, err = tools.UpdateUser(context.Background(), tt.request, NameInput{Name: tt.input})
			case "delete":
				_, _, err = tools.DeleteUser(context.Background(), tt.request, struct{}{})
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if users.method != tt.wantMethod {
				t.Errorf("service method = %q, want %q", users.method, tt.wantMethod)
			}
			if tt.wantMethod != "" {
				if users.authID != "logto-sub" {
					t.Errorf("service authID = %q", users.authID)
				}
				if (tt.operation == "create" || tt.operation == "update") && users.name != tt.input {
					t.Errorf("service name = %q, want %q", users.name, tt.input)
				}
			}
			if output != tt.wantOutput {
				t.Errorf("output = %+v, want %+v", output, tt.wantOutput)
			}
		})
	}
}
