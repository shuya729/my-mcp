package mcpserver

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"my-mcp/internal/mcptools"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const clockSkew = 30 * time.Second

// MCP HTTP ハンドラーを構築
func New(resource, issuer string, verifier auth.TokenVerifier, users mcptools.UserService) (http.Handler, error) {
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
	userTools := mcptools.UserTools{Users: users}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_user",
		Description: "Get the authenticated user's ID and name. Requires user:read.",
	}, userTools.GetUser)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_user",
		Description: "Create the authenticated user with a name. Requires user:write.",
	}, userTools.CreateUser)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_user",
		Description: "Update the authenticated user's name. Requires user:write.",
	}, userTools.UpdateUser)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_user",
		Description: "Delete the authenticated user. Requires user:write.",
	}, userTools.DeleteUser)

	streamableHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)
	authenticatedHandler := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL.String(),
		ClockSkew:           clockSkew,
	})(streamableHandler)

	r := chi.NewRouter()
	r.Handle(metadataPath, auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   []string{issuer},
		ScopesSupported:        []string{mcptools.ReadScope, mcptools.WriteScope},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "my-mcp",
	}))
	r.Method(http.MethodPost, "/", authenticatedHandler)

	return r, nil
}
