package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"my-mcp/internal/apiserver"
	"my-mcp/internal/authn"
	"my-mcp/internal/config"
	"my-mcp/internal/repository"
	"my-mcp/internal/usecase"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	listenAddress     = ":8080"
	readHeaderTimeout = 15 * time.Second
	readTimeout       = 2 * time.Minute
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 3 * time.Minute
)

func main() {
	if err := run(); err != nil {
		slog.Error("API server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configuration, err := config.LoadAPI()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	issuer, err := url.JoinPath(configuration.LogtoURL, "oidc")
	if err != nil {
		return fmt.Errorf("build issuer URL: %w", err)
	}
	verifier, err := authn.NewVerifier(ctx, issuer, configuration.APIURL)
	if err != nil {
		return fmt.Errorf("initialize token verifier: %w", err)
	}
	defer func() {
		if err := verifier.Close(context.Background()); err != nil {
			slog.Error("failed to stop JWKS cache", "error", err)
		}
	}()

	pool, err := pgxpool.New(ctx, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize database pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	users := usecase.NewUsers(repository.NewUserRepository(pool))
	handler, err := apiserver.New(configuration.APIURL, issuer, verifier.Verify, users)
	if err != nil {
		return fmt.Errorf("initialize API handler: %w", err)
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
		slog.Info("API server starting", "address", listenAddress, "resource", configuration.APIURL)
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down API server: %w", err)
		}
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}
