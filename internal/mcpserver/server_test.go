package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"my-mcp/internal/domain"
	"my-mcp/internal/mcptools"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		resource    string
		wantHandler bool
		wantErr     bool
	}{
		{name: "root URL", resource: "https://mcp.example.com", wantHandler: true},
		{name: "root URL with trailing slash", resource: "https://mcp.example.com/", wantHandler: true},
		{name: "malformed URL", resource: "https://mcp.example.com/%ZZ", wantErr: true},
		{name: "relative URL", resource: "/mcp", wantErr: true},
		{name: "URL without host", resource: "https:///mcp", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(test.resource, "https://issuer.example.com", nil, nil)
			if (err != nil) != test.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, test.wantErr)
			}
			if (handler != nil) != test.wantHandler {
				t.Errorf("New() handler = %v, wantHandler %v", handler, test.wantHandler)
			}
		})
	}
}

func TestUserToolsAreRegistered(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		return &auth.TokenInfo{UserID: "sub", Expiration: time.Now().Add(time.Hour)}, nil
	}
	handler, err := New("https://mcp.example.com", "https://issuer.example.com", verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d, body = %s", response.Code, response.Body.String())
	}
	var result struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Required []string `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range result.Result.Tools {
		names = append(names, tool.Name)
		if (tool.Name == "create_user" || tool.Name == "update_user") && !slices.Contains(tool.InputSchema.Required, "name") {
			t.Errorf("%s input schema does not require name: %s", tool.Name, response.Body.String())
		}
	}
	slices.Sort(names)
	want := []string{"create_user", "delete_user", "get_user", "update_user"}
	if !slices.Equal(names, want) {
		t.Errorf("registered tools = %v, want %v; body = %s", names, want, response.Body.String())
	}
}

type recordingUserService struct {
	operation string
	authID    string
	name      string
}

func (u *recordingUserService) Get(_ context.Context, authID string) (domain.User, error) {
	u.operation, u.authID = "get", authID
	return domain.User{ID: "user-id", Name: "Alice"}, nil
}

func (u *recordingUserService) Create(_ context.Context, authID, name string) (domain.User, error) {
	u.operation, u.authID, u.name = "create", authID, name
	return domain.User{ID: "user-id", Name: name}, nil
}

func (u *recordingUserService) Update(_ context.Context, authID, name string) (domain.User, error) {
	u.operation, u.authID, u.name = "update", authID, name
	return domain.User{ID: "user-id", Name: name}, nil
}

func (u *recordingUserService) Delete(_ context.Context, authID string) error {
	u.operation, u.authID = "delete", authID
	return nil
}

func TestToolCallsUseAuthenticatedIdentity(t *testing.T) {
	tests := []struct {
		name          string
		tool          string
		arguments     string
		scope         string
		wantOperation string
		wantName      string
		wantOutput    map[string]any
		wantToolError bool
	}{
		{name: "get with read scope", tool: "get_user", arguments: `{}`, scope: mcptools.ReadScope, wantOperation: "get", wantOutput: map[string]any{"id": "user-id", "name": "Alice"}},
		{name: "create with write scope", tool: "create_user", arguments: `{"name":"Bob"}`, scope: mcptools.WriteScope, wantOperation: "create", wantName: "Bob", wantOutput: map[string]any{"id": "user-id", "name": "Bob"}},
		{name: "update with write scope", tool: "update_user", arguments: `{"name":"Carol"}`, scope: mcptools.WriteScope, wantOperation: "update", wantName: "Carol", wantOutput: map[string]any{"id": "user-id", "name": "Carol"}},
		{name: "delete with write scope", tool: "delete_user", arguments: `{}`, scope: mcptools.WriteScope, wantOperation: "delete", wantOutput: map[string]any{}},
		{name: "get rejects write-only scope", tool: "get_user", arguments: `{}`, scope: mcptools.WriteScope, wantToolError: true},
		{name: "create rejects read-only scope", tool: "create_user", arguments: `{"name":"Bob"}`, scope: mcptools.ReadScope, wantToolError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &recordingUserService{}
			verifier := func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
				return &auth.TokenInfo{UserID: "logto-sub", Scopes: []string{tt.scope}, Expiration: time.Now().Add(time.Hour)}, nil
			}
			handler, err := New("https://mcp.example.com", "https://issuer.example.com", verifier, users)
			if err != nil {
				t.Fatal(err)
			}
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tt.tool + `","arguments":` + tt.arguments + `}}`
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer test-token")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			var result struct {
				Result struct {
					IsError           bool           `json:"isError"`
					StructuredContent map[string]any `json:"structuredContent"`
				} `json:"result"`
				Error json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Error) != 0 || result.Result.IsError != tt.wantToolError {
				t.Errorf("tool error = %v, protocol error present = %v, wantToolError %v", result.Result.IsError, len(result.Error) != 0, tt.wantToolError)
			}
			if users.operation != tt.wantOperation || users.name != tt.wantName {
				t.Errorf("service call = %q (%q), want %q (%q)", users.operation, users.name, tt.wantOperation, tt.wantName)
			}
			if tt.wantOperation != "" && users.authID != "logto-sub" {
				t.Errorf("service authID = %q", users.authID)
			}
			if !tt.wantToolError && !reflect.DeepEqual(result.Result.StructuredContent, tt.wantOutput) {
				t.Errorf("structured content = %+v, want %+v", result.Result.StructuredContent, tt.wantOutput)
			}
		})
	}
}

func TestNewHTTP(t *testing.T) {
	const (
		resource = "https://mcp.example.com"
		issuer   = "https://issuer.example.com"
	)
	tests := []struct {
		name               string
		method             string
		path               string
		authorization      string
		verifierInfo       *auth.TokenInfo
		verifierErr        error
		wantStatus         int
		wantChallenge      bool
		wantVerifierCalled bool
		wantMetadata       *oauthex.ProtectedResourceMetadata
	}{
		{
			name:       "metadata without authentication",
			method:     http.MethodGet,
			path:       "/.well-known/oauth-protected-resource",
			wantStatus: http.StatusOK,
			wantMetadata: &oauthex.ProtectedResourceMetadata{
				Resource:               resource,
				AuthorizationServers:   []string{issuer},
				ScopesSupported:        []string{mcptools.ReadScope, mcptools.WriteScope},
				BearerMethodsSupported: []string{"header"},
				ResourceName:           "my-mcp",
			},
		},
		{
			name:          "POST without bearer token",
			method:        http.MethodPost,
			path:          "/",
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: true,
		},
		{
			name:               "invalid bearer token",
			method:             http.MethodPost,
			path:               "/",
			authorization:      "Bearer test-token",
			verifierErr:        auth.ErrInvalidToken,
			wantStatus:         http.StatusUnauthorized,
			wantChallenge:      true,
			wantVerifierCalled: true,
		},
		{
			name:       "root rejects GET",
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "other path is not an MCP endpoint",
			method:     http.MethodPost,
			path:       "/other",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifierCalled := false
			verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
				verifierCalled = true
				if token != "test-token" {
					t.Errorf("verifier token = %q, want %q", token, "test-token")
				}
				return test.verifierInfo, test.verifierErr
			}
			handler, err := New(resource, issuer, verifier, nil)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			request := httptest.NewRequest(test.method, test.path, nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("ServeHTTP() status = %d, want %d", response.Code, test.wantStatus)
			}
			if got := response.Header().Get("WWW-Authenticate"); (got != "") != test.wantChallenge {
				t.Errorf("WWW-Authenticate present = %v, want %v", got != "", test.wantChallenge)
			}
			if verifierCalled != test.wantVerifierCalled {
				t.Errorf("verifier called = %v, want %v", verifierCalled, test.wantVerifierCalled)
			}
			if test.wantMetadata != nil {
				var got oauthex.ProtectedResourceMetadata
				if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode metadata: %v", err)
				}
				if !reflect.DeepEqual(&got, test.wantMetadata) {
					t.Errorf("metadata = %+v, want %+v", got, test.wantMetadata)
				}
			}
		})
	}
}
