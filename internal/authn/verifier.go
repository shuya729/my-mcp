package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jwx-go/jwkfetch/v4"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/lestrrat-go/jwx/v4/jwt"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	clockSkew             = 30 * time.Second
	initializationTimeout = 90 * time.Second
	shutdownTimeout       = time.Second
)

type Verifier struct {
	issuer   string
	audience string
	jwksURI  string
	cache    *jwkfetch.Cache
}

// 認可サーバーを検出し、JWKS を読み込む。使用後は Close でキャッシュを停止する。
func NewVerifier(ctx context.Context, issuer, audience string) (*Verifier, error) {
	initCtx, cancel := context.WithTimeout(ctx, initializationTimeout)
	defer cancel()

	metadata, err := auth.GetAuthServerMetadata(initCtx, issuer, nil)
	if err != nil {
		return nil, fmt.Errorf("discover authorization server: %w", err)
	}
	if metadata == nil {
		return nil, errors.New("authorization server metadata not found")
	}
	if metadata.Issuer != issuer {
		return nil, fmt.Errorf("discovered issuer %q does not exactly match %q", metadata.Issuer, issuer)
	}
	if metadata.JWKSURI == "" {
		return nil, errors.New("authorization server metadata has no jwks_uri")
	}

	jwksHTTPClient := jwkfetch.DefaultHTTPClient()
	jwksHTTPClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	cache, err := jwkfetch.NewCache(
		// 起動の期限切れや SIGTERM で、処理中リクエストの鍵取得を停止しない。
		context.WithoutCancel(ctx),
		httprc.NewClient(),
		jwkfetch.WithHTTPClient(jwksHTTPClient),
	)
	if err != nil {
		return nil, fmt.Errorf("create JWKS cache: %w", err)
	}
	if err := cache.Register(initCtx, metadata.JWKSURI); err != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		_ = cache.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("register JWKS: %w", err)
	}

	return &Verifier{
		issuer:   issuer,
		audience: audience,
		jwksURI:  metadata.JWKSURI,
		cache:    cache,
	}, nil
}

// アクセストークンを検証
func (v *Verifier) Verify(ctx context.Context, rawToken string, _ *http.Request) (*auth.TokenInfo, error) {
	keySet, err := v.cache.Fetch(ctx, v.jwksURI)
	if err != nil {
		return nil, fmt.Errorf("load cached JWKS: %w", err)
	}

	token, err := jwt.Parse(
		[]byte(rawToken),
		jwt.WithKeySet(keySet, jws.WithInferAlgorithmFromKey(true)),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithRequiredClaim(jwt.ExpirationKey),
		jwt.WithRequiredClaim(jwt.SubjectKey),
		jwt.WithAcceptableSkew(clockSkew),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: token verification failed", auth.ErrInvalidToken)
	}

	subject, ok := token.Subject()
	if !ok || subject == "" {
		return nil, fmt.Errorf("%w: token subject is missing", auth.ErrInvalidToken)
	}
	expiration, ok := token.Expiration()
	if !ok {
		return nil, fmt.Errorf("%w: token expiration is missing", auth.ErrInvalidToken)
	}

	var scopes []string
	if scopeClaim, ok := token.Field("scope"); ok {
		scope, ok := scopeClaim.(string)
		if !ok {
			return nil, fmt.Errorf("%w: token scope has an invalid type", auth.ErrInvalidToken)
		}
		scopes = strings.Fields(scope)
	}

	return &auth.TokenInfo{
		Scopes:     scopes,
		Expiration: expiration,
		UserID:     subject,
	}, nil
}

// JWKS キャッシュの処理を終了
func (v *Verifier) Close(ctx context.Context) error {
	return v.cache.Shutdown(ctx)
}
