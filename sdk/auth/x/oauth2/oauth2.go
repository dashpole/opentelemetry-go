// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/otel/sdk/auth/x"
)

const (
	// defaultExpirySkew is subtracted from the token's expires_in duration so
	// tokens are refreshed proactively before they expire on the wire.
	defaultExpirySkew = 30 * time.Second

	// defaultFallbackLifetime is used when the token server omits expires_in.
	defaultFallbackLifetime = 60 * time.Second

	// maxTokenResponseBytes bounds the response body read from the token endpoint.
	maxTokenResponseBytes = 1 << 20 // 1 MiB
)

// ErrShutdown is returned by an [Authenticator] after [Authenticator.Shutdown]
// has been called.
var ErrShutdown = errors.New("oauth2 authenticator is shut down")

// Config configures an OAuth 2.0 Client Credentials [Authenticator].
type Config struct {
	// TokenURL is the OAuth 2.0 token endpoint URL (required).
	TokenURL string

	// ClientID is the OAuth 2.0 client identifier (required).
	ClientID string

	// ClientSecret is the OAuth 2.0 client secret (required).
	ClientSecret string //nolint:gosec // Intentional OAuth2 client credential field.

	// Scopes is an optional list of OAuth 2.0 scopes to request.
	Scopes []string

	// Audience is an optional target audience parameter to include in the
	// token request.
	Audience string
}

type options struct {
	httpClient *http.Client
	expirySkew time.Duration
	now        func() time.Time
}

// Option configures optional settings on an OAuth 2.0 [Authenticator].
type Option interface {
	apply(options) options
}

type optionFunc func(options) options

func (f optionFunc) apply(o options) options { return f(o) }

// WithHTTPClient configures a custom [http.Client] used to communicate with
// the OAuth 2.0 token endpoint.
func WithHTTPClient(client *http.Client) Option {
	return optionFunc(func(o options) options {
		o.httpClient = client
		return o
	})
}

// WithExpirySkew configures the safety buffer subtracted from a token's
// expires_in lifetime before considering the cached token expired.
func WithExpirySkew(skew time.Duration) Option {
	return optionFunc(func(o options) options {
		o.expirySkew = skew
		return o
	})
}

// Authenticator implements [x.Authenticator] using the OAuth 2.0 Client
// Credentials grant (RFC 6749 §4.4).
//
// It caches the acquired access token in memory and refreshes it automatically
// when no token is cached or the cached token is within the expiry skew window.
type Authenticator struct {
	tokenURL     string
	clientID     string
	clientSecret string
	scopes       []string
	audience     string

	httpClient    *http.Client
	ownHTTPClient bool
	expirySkew    time.Duration
	now           func() time.Time

	mu          sync.Mutex
	stopped     bool
	accessToken string
	expiresAt   time.Time
}

var _ x.Authenticator = (*Authenticator)(nil)

// New creates a new OAuth 2.0 Client Credentials [Authenticator].
//
// It returns an error if TokenURL, ClientID, or ClientSecret are empty or if
// TokenURL is not a valid HTTP/HTTPS URL.
func New(cfg Config, opts ...Option) (*Authenticator, error) {
	o := options{
		expirySkew: defaultExpirySkew,
		now:        time.Now,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		o = opt.apply(o)
	}

	if strings.TrimSpace(cfg.TokenURL) == "" {
		return nil, errors.New("oauth2: token_url is required")
	}
	parsedURL, err := url.Parse(cfg.TokenURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("oauth2: token_url must be a valid http or https URL")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, errors.New("oauth2: client_id is required")
	}
	if cfg.ClientSecret == "" {
		return nil, errors.New("oauth2: client_secret is required")
	}

	hc := o.httpClient
	ownHC := false
	if hc == nil {
		hc = &http.Client{
			Timeout:   30 * time.Second,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		}
		ownHC = true
	}

	return &Authenticator{
		tokenURL:      cfg.TokenURL,
		clientID:      cfg.ClientID,
		clientSecret:  cfg.ClientSecret,
		scopes:        append([]string(nil), cfg.Scopes...),
		audience:      cfg.Audience,
		httpClient:    hc,
		ownHTTPClient: ownHC,
		expirySkew:    o.expirySkew,
		now:           o.now,
	}, nil
}

func (a *Authenticator) headers(ctx context.Context) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return nil, ErrShutdown
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := a.now()
	if a.accessToken != "" && (a.expiresAt.IsZero() || now.Before(a.expiresAt)) {
		return map[string]string{
			"Authorization": "Bearer " + a.accessToken,
		}, nil
	}

	tok, exp, err := a.fetchToken(ctx, now)
	if err != nil {
		return nil, err
	}

	a.accessToken = tok
	a.expiresAt = exp

	return map[string]string{
		"Authorization": "Bearer " + a.accessToken,
	}, nil
}

// RoundTripper wraps base with an [http.RoundTripper] that attaches the OAuth
// 2.0 Bearer token to each outgoing HTTP export request attempt.
func (a *Authenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return x.NewHeaders(a.headers).RoundTripper(base)
}

// PerRPCCredentials returns a [credentials.PerRPCCredentials] that attaches
// the OAuth 2.0 Bearer token to each outgoing gRPC export RPC attempt.
func (a *Authenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return x.NewHeaders(a.headers).PerRPCCredentials()
}

type tokenResponse struct {
	AccessToken string `json:"access_token"` //nolint:gosec // OAuth2 token response schema field.
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (a *Authenticator) fetchToken(ctx context.Context, now time.Time) (string, time.Time, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if len(a.scopes) > 0 {
		form.Set("scope", strings.Join(a.scopes, " "))
	}
	if a.audience != "" {
		form.Set("audience", a.audience)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("oauth2: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(a.clientID, a.clientSecret)

	//nolint:gosec // tokenURL is validated in New.
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("oauth2: token request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Do not include response body or client secret in error message to
		// guarantee credential redaction.
		return "", time.Time{}, fmt.Errorf("oauth2: token endpoint returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("oauth2: read token response: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(bodyBytes, &tr); err != nil {
		return "", time.Time{}, fmt.Errorf("oauth2: decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, errors.New("oauth2: token response missing access_token")
	}

	var lifetime time.Duration
	if tr.ExpiresIn > 0 {
		lifetime = time.Duration(tr.ExpiresIn) * time.Second
		if lifetime > a.expirySkew {
			lifetime -= a.expirySkew
		} else {
			lifetime /= 2
		}
	} else {
		lifetime = defaultFallbackLifetime
	}

	return tr.AccessToken, now.Add(lifetime), nil
}

// Shutdown clears any cached access token and closes idle connections on the
// internal HTTP client if it was created by [New]. Shutdown is idempotent.
func (a *Authenticator) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stopped = true
	a.accessToken = ""
	a.expiresAt = time.Time{}

	if a.ownHTTPClient && a.httpClient != nil {
		a.httpClient.CloseIdleConnections()
	}
	return ctx.Err()
}
