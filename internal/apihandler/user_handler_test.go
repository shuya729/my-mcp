package apihandler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"my-mcp/internal/domain"
	"my-mcp/internal/usecase"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

type recordingUsers struct {
	operation string
	authID    string
	name      string
	err       error
}

func (u *recordingUsers) Get(_ context.Context, authID string) (domain.User, error) {
	u.operation, u.authID = "get", authID
	return domain.User{ID: "user-id", Name: "Alice"}, u.err
}

func (u *recordingUsers) Create(_ context.Context, authID, name string) (domain.User, error) {
	u.operation, u.authID, u.name = "create", authID, name
	return domain.User{ID: "user-id", Name: name}, u.err
}

func (u *recordingUsers) Update(_ context.Context, authID, name string) (domain.User, error) {
	u.operation, u.authID, u.name = "update", authID, name
	return domain.User{ID: "user-id", Name: name}, u.err
}

func (u *recordingUsers) Delete(_ context.Context, authID string) error {
	u.operation, u.authID = "delete", authID
	return u.err
}

func TestUserHandler(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		body          string
		serviceErr    error
		wantStatus    int
		wantOperation string
		wantName      string
		wantBody      string
	}{
		{name: "get", method: http.MethodGet, wantStatus: http.StatusOK, wantOperation: "get", wantBody: `{"id":"user-id","name":"Alice"}`},
		{name: "create", method: http.MethodPost, body: `{"name":"Bob"}`, wantStatus: http.StatusCreated, wantOperation: "create", wantName: "Bob", wantBody: `{"id":"user-id","name":"Bob"}`},
		{name: "update", method: http.MethodPatch, body: `{"name":"Carol"}`, wantStatus: http.StatusOK, wantOperation: "update", wantName: "Carol", wantBody: `{"id":"user-id","name":"Carol"}`},
		{name: "delete", method: http.MethodDelete, wantStatus: http.StatusNoContent, wantOperation: "delete"},
		{name: "missing name", method: http.MethodPost, body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "malformed JSON", method: http.MethodPost, body: `{"name":`, wantStatus: http.StatusBadRequest},
		{name: "unknown field", method: http.MethodPost, body: `{"name":"Bob","admin":true}`, wantStatus: http.StatusBadRequest},
		{name: "trailing JSON", method: http.MethodPost, body: `{"name":"Bob"}{}`, wantStatus: http.StatusBadRequest},
		{name: "oversized body", method: http.MethodPost, body: strings.Repeat("x", 1<<20+1), wantStatus: http.StatusBadRequest},
		{name: "missing update name", method: http.MethodPatch, body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "not found", method: http.MethodGet, serviceErr: usecase.ErrUserNotFound, wantStatus: http.StatusNotFound, wantOperation: "get"},
		{name: "already exists", method: http.MethodPost, body: `{"name":"Bob"}`, serviceErr: usecase.ErrUserExists, wantStatus: http.StatusConflict, wantOperation: "create", wantName: "Bob"},
		{name: "invalid name", method: http.MethodPatch, body: `{"name":" "}`, serviceErr: usecase.ErrInvalidName, wantStatus: http.StatusBadRequest, wantOperation: "update", wantName: " "},
		{name: "unexpected service error", method: http.MethodDelete, serviceErr: errors.New("database failure"), wantStatus: http.StatusInternalServerError, wantOperation: "delete"},
	}
	verifier := func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		return &auth.TokenInfo{UserID: "logto-sub", Expiration: time.Now().Add(time.Hour)}, nil
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &recordingUsers{err: tt.serviceErr}
			h := NewUserHandler(users)
			var endpoint http.HandlerFunc
			switch tt.method {
			case http.MethodGet:
				endpoint = h.GetMe
			case http.MethodPost:
				endpoint = h.CreateMe
			case http.MethodPatch:
				endpoint = h.UpdateMe
			case http.MethodDelete:
				endpoint = h.DeleteMe
			}
			request := httptest.NewRequest(tt.method, "/v1/me", strings.NewReader(tt.body))
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()
			auth.RequireBearerToken(verifier, nil)(endpoint).ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if users.operation != tt.wantOperation || users.name != tt.wantName {
				t.Errorf("service call = %q (%q), want %q (%q)", users.operation, users.name, tt.wantOperation, tt.wantName)
			}
			if tt.wantOperation != "" && users.authID != "logto-sub" {
				t.Errorf("service authID = %q", users.authID)
			}
			if tt.wantBody != "" && strings.TrimSpace(response.Body.String()) != tt.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), tt.wantBody)
			}
			if tt.wantStatus == http.StatusNoContent && response.Body.Len() != 0 {
				t.Errorf("204 response has a body")
			}
		})
	}
}
