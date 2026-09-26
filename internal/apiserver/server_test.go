package apiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"my-mcp/internal/domain"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

type recordingUsers struct {
	operation string
	authID    string
	name      string
}

func (u *recordingUsers) Get(_ context.Context, authID string) (domain.User, error) {
	u.operation, u.authID = "get", authID
	return domain.User{ID: "user-id", Name: "Alice"}, nil
}

func (u *recordingUsers) Create(_ context.Context, authID, name string) (domain.User, error) {
	u.operation, u.authID, u.name = "create", authID, name
	return domain.User{ID: "user-id", Name: name}, nil
}

func (u *recordingUsers) Update(_ context.Context, authID, name string) (domain.User, error) {
	u.operation, u.authID, u.name = "update", authID, name
	return domain.User{ID: "user-id", Name: name}, nil
}

func (u *recordingUsers) Delete(_ context.Context, authID string) error {
	u.operation, u.authID = "delete", authID
	return nil
}

func TestRoutesAndScopes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		token      string
		wantStatus int
		wantCall   string
		wantName   string
		isMetadata bool
	}{
		{name: "metadata is public", method: http.MethodGet, path: "/.well-known/oauth-protected-resource", wantStatus: http.StatusOK, isMetadata: true},
		{name: "bearer token required", method: http.MethodGet, path: "/v1/me", wantStatus: http.StatusUnauthorized},
		{name: "get with read scope", method: http.MethodGet, path: "/v1/me", token: "read", wantStatus: http.StatusOK, wantCall: "get"},
		{name: "get rejects write-only scope", method: http.MethodGet, path: "/v1/me", token: "write", wantStatus: http.StatusForbidden},
		{name: "create with write scope", method: http.MethodPost, path: "/v1/me", body: `{"name":"Alice"}`, token: "write", wantStatus: http.StatusCreated, wantCall: "create", wantName: "Alice"},
		{name: "create rejects read-only scope", method: http.MethodPost, path: "/v1/me", body: `{"name":"Alice"}`, token: "read", wantStatus: http.StatusForbidden},
		{name: "update with write scope", method: http.MethodPatch, path: "/v1/me", body: `{"name":"Bob"}`, token: "write", wantStatus: http.StatusOK, wantCall: "update", wantName: "Bob"},
		{name: "update rejects read-only scope", method: http.MethodPatch, path: "/v1/me", body: `{"name":"Bob"}`, token: "read", wantStatus: http.StatusForbidden},
		{name: "delete with write scope", method: http.MethodDelete, path: "/v1/me", token: "write", wantStatus: http.StatusNoContent, wantCall: "delete"},
		{name: "delete rejects read-only scope", method: http.MethodDelete, path: "/v1/me", token: "read", wantStatus: http.StatusForbidden},
	}
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		scope := readScope
		if token == "write" {
			scope = writeScope
		}
		return &auth.TokenInfo{UserID: "logto-sub", Scopes: []string{scope}, Expiration: time.Now().Add(time.Hour)}, nil
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &recordingUsers{}
			handler, err := New("https://api.example.com", "https://issuer.example.com", verifier, users)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.token != "" {
				request.Header.Set("Authorization", "Bearer "+tt.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if users.operation != tt.wantCall || users.name != tt.wantName {
				t.Errorf("service call = %q (%q), want %q (%q)", users.operation, users.name, tt.wantCall, tt.wantName)
			}
			if tt.wantCall != "" && users.authID != "logto-sub" {
				t.Errorf("service authID = %q", users.authID)
			}
			if tt.isMetadata {
				var metadata oauthex.ProtectedResourceMetadata
				if err := json.Unmarshal(response.Body.Bytes(), &metadata); err != nil {
					t.Fatal(err)
				}
				if metadata.Resource != "https://api.example.com" || !reflect.DeepEqual(metadata.AuthorizationServers, []string{"https://issuer.example.com"}) || !reflect.DeepEqual(metadata.ScopesSupported, []string{readScope, writeScope}) {
					t.Errorf("metadata = %+v", metadata)
				}
			}
		})
	}
}

func TestNewRejectsInvalidResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		wantErr  bool
	}{
		{name: "root URL", resource: "https://api.example.com"},
		{name: "relative URL", resource: "/api", wantErr: true},
		{name: "non-root URL", resource: "https://api.example.com/v1", wantErr: true},
		{name: "query", resource: "https://api.example.com?x=1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.resource, "https://issuer.example.com", nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
