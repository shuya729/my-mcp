package apiserver

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"my-mcp/internal/apihandler"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	readScope  = "user:read"
	writeScope = "user:write"
	clockSkew  = 30 * time.Second
)

func New(resource, issuer string, verifier auth.TokenVerifier, users apihandler.UserService) (http.Handler, error) {
	resourceURL, err := url.Parse(resource)
	if err != nil {
		return nil, err
	}
	if resourceURL.Scheme == "" || resourceURL.Host == "" || (resourceURL.Path != "" && resourceURL.Path != "/") || resourceURL.RawQuery != "" || resourceURL.Fragment != "" {
		return nil, errors.New("API_URL must be an absolute root URL without query or fragment")
	}
	metadataURL := resourceURL.ResolveReference(&url.URL{Path: "/.well-known/oauth-protected-resource"})
	readAuth := requireBearerToken(verifier, metadataURL.String(), readScope)
	writeAuth := requireBearerToken(verifier, metadataURL.String(), writeScope)

	usersHandler := apihandler.NewUserHandler(users)
	r := chi.NewRouter()
	r.Get("/.well-known/oauth-protected-resource", auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   []string{issuer},
		ScopesSupported:        []string{readScope, writeScope},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "my-mcp-api",
	}).ServeHTTP)
	r.With(readAuth).Get("/v1/me", usersHandler.GetMe)
	r.With(writeAuth).Post("/v1/me", usersHandler.CreateMe)
	r.With(writeAuth).Patch("/v1/me", usersHandler.UpdateMe)
	r.With(writeAuth).Delete("/v1/me", usersHandler.DeleteMe)
	return r, nil
}

func requireBearerToken(verifier auth.TokenVerifier, metadataURL string, scopes ...string) func(http.Handler) http.Handler {
	return auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
		Scopes:              scopes,
		ClockSkew:           clockSkew,
	})
}
