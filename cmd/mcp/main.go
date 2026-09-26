package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"my-mcp/internal/authn"
	"my-mcp/internal/config"
	"my-mcp/internal/mcpserver"
	"my-mcp/internal/service"
)

const (
	listenAddress     = ":8081"
	readHeaderTimeout = 15 * time.Second
	readTimeout       = 2 * time.Minute
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 3 * time.Minute
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	issuer, err := url.JoinPath(configuration.LogtoURL, "oidc")
	if err != nil {
		slog.Error("failed to build issuer URL", "error", err)
		os.Exit(1)
	}

	verifier, err := authn.NewVerifier(ctx, issuer, configuration.MCPURL)
	if err != nil {
		slog.Error("failed to initialize token verifier", "error", err)
		os.Exit(1)
	}

	handler, err := mcpserver.New(configuration.MCPURL, issuer, verifier.Verify, service.IdentityService{})
	if err != nil {
		_ = verifier.Close(context.Background())
		slog.Error("failed to initialize MCP server", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErrors := make(chan error, 1)
	go func() {
		slog.Info("MCP server starting", "address", listenAddress, "resource", configuration.MCPURL)
		serveErrors <- server.ListenAndServe()
	}()

	exitCode := 0
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shut down HTTP server", "error", err)
		}
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("MCP server stopped", "error", err)
			exitCode = 1
		}
	}

	if err := verifier.Close(context.Background()); err != nil {
		slog.Error("failed to stop JWKS cache", "error", err)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
