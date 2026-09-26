package authn

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jwt"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

func testSigningKeyAndJWKS(t *testing.T) (jwk.Key, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signingKey, err := jwk.Import[jwk.Key](privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := signingKey.Set(jwk.KeyIDKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	publicKey, err := jwk.PublicKeyOf(signingKey)
	if err != nil {
		t.Fatal(err)
	}
	keySet := jwk.NewSet()
	if err := keySet.AddKey(publicKey); err != nil {
		t.Fatal(err)
	}
	jwks, err := json.Marshal(keySet)
	if err != nil {
		t.Fatal(err)
	}
	return signingKey, jwks
}

func TestNewVerifier(t *testing.T) {
	_, validJWKS := testSigningKeyAndJWKS(t)

	tests := []struct {
		name              string
		metadataMissing   bool
		metadataMalformed bool
		issuerSuffix      string
		missingJWKSURI    bool
		jwksStatus        int
		jwksBody          string
		want              *Verifier
		wantErr           bool
	}{
		{name: "valid metadata and JWKS", want: &Verifier{audience: "test-audience"}},
		{name: "metadata not found", metadataMissing: true, wantErr: true},
		{name: "malformed metadata JSON", metadataMalformed: true, wantErr: true},
		{name: "issuer mismatch", issuerSuffix: "/", wantErr: true},
		{name: "missing jwks_uri", missingJWKSURI: true, wantErr: true},
		{name: "JWKS fetch failure", jwksStatus: http.StatusInternalServerError, wantErr: true},
		{name: "invalid JWKS", jwksBody: `not JSON`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server":
					if test.metadataMissing {
						http.NotFound(w, r)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if test.metadataMalformed {
						_, _ = w.Write([]byte(`{"issuer":`))
						return
					}
					jwksURI := server.URL + "/jwks"
					if test.missingJWKSURI {
						jwksURI = ""
					}
					_, _ = fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q,"code_challenge_methods_supported":["S256"]}`,
						server.URL+test.issuerSuffix, jwksURI)
				case "/jwks":
					if test.jwksStatus != 0 {
						w.WriteHeader(test.jwksStatus)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if test.jwksBody != "" {
						_, _ = w.Write([]byte(test.jwksBody))
					} else {
						_, _ = w.Write(validJWKS)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			got, err := NewVerifier(ctx, server.URL, "test-audience")
			if (err != nil) != test.wantErr {
				t.Errorf("NewVerifier() error = %v, wantErr %v", err, test.wantErr)
			}

			want := test.want
			if want != nil {
				want = &Verifier{issuer: server.URL, audience: want.audience, jwksURI: server.URL + "/jwks"}
			}
			if got != nil {
				verifier := got
				t.Cleanup(func() {
					if err := verifier.Close(context.Background()); err != nil {
						t.Errorf("Close() error = %v", err)
					}
				})
				if got.cache == nil {
					t.Error("NewVerifier() cache = nil, want initialized cache")
				}
				copy := *got
				copy.cache = nil
				got = &copy
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("NewVerifier() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestVerifierVerify(t *testing.T) {
	signingKey, jwks := testSigningKeyAndJWKS(t)
	otherKey, _ := testSigningKeyAndJWKS(t)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			_, _ = fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q,"code_challenge_methods_supported":["S256"]}`,
				server.URL, server.URL+"/jwks")
		case "/jwks":
			_, _ = w.Write(jwks)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	const audience = "test-audience"
	verifier, err := NewVerifier(context.Background(), server.URL, audience)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	t.Cleanup(func() {
		if err := verifier.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	expiration := time.Now().Add(5 * time.Minute).Truncate(time.Second).UTC()
	tests := []struct {
		name              string
		scope             any
		hasScope          bool
		issuerMismatch    bool
		audienceMismatch  bool
		missingSubject    bool
		emptySubject      bool
		missingExpiration bool
		malformed         bool
		invalidSignature  bool
		otherSigningKey   bool
		want              *auth.TokenInfo
		wantErr           bool
	}{
		{name: "valid token", hasScope: true, scope: "read write", want: &auth.TokenInfo{UserID: "user-123", Expiration: expiration, Scopes: []string{"read", "write"}}},
		{name: "scope absent", want: &auth.TokenInfo{UserID: "user-123", Expiration: expiration}},
		{name: "scope with whitespace", hasScope: true, scope: " read\twrite \n admin ", want: &auth.TokenInfo{UserID: "user-123", Expiration: expiration, Scopes: []string{"read", "write", "admin"}}},
		{name: "scope with invalid type", hasScope: true, scope: []string{"read"}, wantErr: true},
		{name: "malformed token", malformed: true, wantErr: true},
		{name: "invalid signature", invalidSignature: true, wantErr: true},
		{name: "different signing key", otherSigningKey: true, wantErr: true},
		{name: "issuer mismatch", issuerMismatch: true, wantErr: true},
		{name: "audience mismatch", audienceMismatch: true, wantErr: true},
		{name: "missing subject", missingSubject: true, wantErr: true},
		{name: "empty subject", emptySubject: true, wantErr: true},
		{name: "missing expiration", missingExpiration: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rawToken := "not-a-jwt"
			if !test.malformed {
				issuer := server.URL
				if test.issuerMismatch {
					issuer += "/other"
				}
				tokenAudience := audience
				if test.audienceMismatch {
					tokenAudience = "other-audience"
				}
				builder := jwt.NewBuilder().Issuer(issuer).Audience([]string{tokenAudience})
				if !test.missingSubject {
					subject := "user-123"
					if test.emptySubject {
						subject = ""
					}
					builder.Subject(subject)
				}
				if !test.missingExpiration {
					builder.Expiration(expiration)
				}
				if test.hasScope {
					builder.Claim("scope", test.scope)
				}
				token, err := builder.Build()
				if err != nil {
					t.Fatalf("Build() error = %v", err)
				}
				key := signingKey
				if test.otherSigningKey {
					key = otherKey
				}
				signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), key))
				if err != nil {
					t.Fatalf("Sign() error = %v", err)
				}
				rawToken = string(signed)
				if test.invalidSignature {
					parts := strings.Split(rawToken, ".")
					parts[2] = base64.RawURLEncoding.EncodeToString([]byte("invalid-signature"))
					rawToken = strings.Join(parts, ".")
				}
			}

			got, err := verifier.Verify(context.Background(), rawToken, nil)
			if (err != nil) != test.wantErr {
				t.Errorf("Verify() error = %v, wantErr %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("Verify() = %+v, want %+v", got, test.want)
			}
		})
	}
}
