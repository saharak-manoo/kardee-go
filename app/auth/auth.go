// Package auth checks who a user is. A client proves it with a token from a login
// provider (a LIFF ID token today; Google or Apple later); the server checks that token
// with the provider and then issues its own session token. Clients never tell the server
// a user ID directly.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Login providers, stored in user_identities.provider.
const (
	ProviderLINE = "line"
)

// ErrInvalidToken means the provider did not accept the token.
var ErrInvalidToken = errors.New("invalid login token")

// Identity is a user as a provider knows them.
type Identity struct {
	Provider string
	Subject  string // the provider's user ID
	Name     string
	Picture  string
}

// Verifier checks a provider token.
type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

const lineVerifyURL = "https://api.line.me/oauth2/v2.1/verify"

// LINEVerifier checks LIFF / LINE Login ID tokens with LINE's verify endpoint.
// ChannelID is the LINE Login channel the LIFF app belongs to; it must sit under the
// same LINE provider as the Messaging API channel, or the bot sees different user IDs.
type LINEVerifier struct {
	ChannelID string
	Endpoint  string // tests only; default LINE's
	HTTP      *http.Client
}

func (v *LINEVerifier) Verify(ctx context.Context, token string) (Identity, error) {
	endpoint := v.Endpoint
	if endpoint == "" {
		endpoint = lineVerifyURL
	}
	client := v.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	form := url.Values{"id_token": {token}, "client_id": {v.ChannelID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("line verify: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return Identity{}, ErrInvalidToken
	}
	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("line verify: status %d", resp.StatusCode)
	}
	var claims struct {
		Sub     string `json:"sub"`
		Aud     string `json:"aud"`
		Exp     int64  `json:"exp"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return Identity{}, fmt.Errorf("line verify: %w", err)
	}
	// LINE already checks client_id; check again so a token for another channel never passes.
	if claims.Sub == "" || claims.Aud != v.ChannelID || time.Unix(claims.Exp, 0).Before(time.Now()) {
		return Identity{}, ErrInvalidToken
	}
	return Identity{Provider: ProviderLINE, Subject: claims.Sub, Name: claims.Name, Picture: claims.Picture}, nil
}

// NewToken makes a session token for the client and the hash the server stores.
func NewToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is how a session token is stored and looked up.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
