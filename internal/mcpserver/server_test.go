package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"my-mcp/internal/service"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
			handler, err := New(test.resource, "https://issuer.example.com", nil, service.IdentityService{})
			if (err != nil) != test.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, test.wantErr)
			}
			if (handler != nil) != test.wantHandler {
				t.Errorf("New() handler = %v, wantHandler %v", handler, test.wantHandler)
			}
		})
	}
}

func TestNewHTTP(t *testing.T) {
	const (
		resource = "https://mcp.example.com"
		issuer   = "https://issuer.example.com"
	)
	const challenge = `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource", scope="user:read"`

	tests := []struct {
		name               string
		method             string
		path               string
		authorization      string
		verifierInfo       *auth.TokenInfo
		verifierErr        error
		wantStatus         int
		wantChallenge      string
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
				ScopesSupported:        []string{RequiredScope},
				BearerMethodsSupported: []string{"header"},
				ResourceName:           "my-mcp",
			},
		},
		{
			name:          "POST without bearer token",
			method:        http.MethodPost,
			path:          "/",
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: challenge,
		},
		{
			name:               "invalid bearer token",
			method:             http.MethodPost,
			path:               "/",
			authorization:      "Bearer test-token",
			verifierErr:        auth.ErrInvalidToken,
			wantStatus:         http.StatusUnauthorized,
			wantChallenge:      challenge,
			wantVerifierCalled: true,
		},
		{
			name:          "missing required scope",
			method:        http.MethodPost,
			path:          "/",
			authorization: "Bearer test-token",
			verifierInfo: &auth.TokenInfo{
				Scopes:     []string{"other:read"},
				Expiration: time.Now().Add(time.Hour),
			},
			wantStatus:         http.StatusForbidden,
			wantChallenge:      challenge,
			wantVerifierCalled: true,
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
			handler, err := New(resource, issuer, verifier, service.IdentityService{})
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
			if got := response.Header().Get("WWW-Authenticate"); got != test.wantChallenge {
				t.Errorf("ServeHTTP() WWW-Authenticate = %q, want %q", got, test.wantChallenge)
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

func TestToolHandlerWhoAmI(t *testing.T) {
	tests := []struct {
		name         string
		request      *mcp.CallToolRequest
		wantResult   *mcp.CallToolResult
		wantIdentity service.Identity
		wantErr      bool
	}{
		{
			name: "user ID in token info",
			request: &mcp.CallToolRequest{Extra: &mcp.RequestExtra{
				TokenInfo: &auth.TokenInfo{UserID: "user-123"},
			}},
			wantIdentity: service.Identity{UserID: "user-123"},
		},
		{name: "missing Extra", request: &mcp.CallToolRequest{}, wantErr: true},
		{name: "missing TokenInfo", request: &mcp.CallToolRequest{Extra: &mcp.RequestExtra{}}, wantErr: true},
	}

	handler := &toolHandler{identities: service.IdentityService{}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, identity, err := handler.whoAmI(context.Background(), test.request, struct{}{})
			if (err != nil) != test.wantErr {
				t.Errorf("whoAmI() error = %v, wantErr %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(result, test.wantResult) {
				t.Errorf("whoAmI() result = %v, want %v", result, test.wantResult)
			}
			if !reflect.DeepEqual(identity, test.wantIdentity) {
				t.Errorf("whoAmI() identity = %v, want %v", identity, test.wantIdentity)
			}
		})
	}
}
