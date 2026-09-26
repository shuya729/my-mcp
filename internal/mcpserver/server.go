package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"my-mcp/internal/service"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	RequiredScope = "user:read"
	clockSkew     = 30 * time.Second
)

type toolHandler struct {
	identities service.IdentityService
}

// MCP HTTP ハンドラーを構築
func New(resource, issuer string, verifier auth.TokenVerifier, identities service.IdentityService) (http.Handler, error) {
	resourceURL, err := url.Parse(resource)
	if err != nil {
		return nil, fmt.Errorf("parse MCP_URL: %w", err)
	}
	if resourceURL.Scheme == "" || resourceURL.Host == "" || (resourceURL.Path != "" && resourceURL.Path != "/") || resourceURL.RawQuery != "" || resourceURL.Fragment != "" {
		return nil, errors.New("MCP_URL must be an absolute root URL without query or fragment")
	}

	const metadataPath = "/.well-known/oauth-protected-resource"
	metadataURL := resourceURL.ResolveReference(&url.URL{Path: metadataPath})

	server := mcp.NewServer(&mcp.Implementation{Name: "my-mcp", Version: "1.0.0"}, nil)
	tools := &toolHandler{identities: identities}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "whoami",
		Description: "Return the authenticated user's identity.",
	}, tools.whoAmI)

	streamableHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)
	authenticatedHandler := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL.String(),
		Scopes:              []string{RequiredScope},
		ClockSkew:           clockSkew,
	})(streamableHandler)

	mux := http.NewServeMux()
	mux.Handle(metadataPath, auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   []string{issuer},
		ScopesSupported:        []string{RequiredScope},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "my-mcp",
	}))
	mux.Handle("POST /{$}", authenticatedHandler)

	return mux, nil
}

// ユーザーの UserID を返す
func (h *toolHandler) whoAmI(_ context.Context, request *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, service.Identity, error) {
	if request.Extra == nil || request.Extra.TokenInfo == nil {
		return nil, service.Identity{}, errors.New("authenticated identity is unavailable")
	}
	tokenInfo := request.Extra.TokenInfo
	return nil, h.identities.Current(tokenInfo.UserID), nil
}
