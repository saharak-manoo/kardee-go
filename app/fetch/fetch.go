// Package fetch is a polite HTTP client: one request at a time per host, with a pause
// between requests, an honest User-Agent, and a body size cap.
package fetch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const maxBody = 8 << 20 // 8 MB; Krungsri's listing page is ~3.5 MB

// ErrBlocked means the site refused us (403/429 or a bot wall). Don't retry around it.
var ErrBlocked = errors.New("blocked by the site")

// legacyTLSHosts only offer TLS 1.2 with RSA key exchange (krungsricard.com:
// AES256-GCM-SHA384), which Go leaves out by default. Certificates are still verified.
var legacyTLSHosts = map[string]bool{"www.krungsricard.com": true}

type Client struct {
	http      *http.Client
	legacy    *http.Client
	userAgent string
	delay     time.Duration

	mu   sync.Mutex
	last map[string]time.Time // host -> last request
	host map[string]*sync.Mutex
}

func New(userAgent string, delay time.Duration) *Client {
	suites := make([]uint16, 0, len(tls.CipherSuites())+2)
	for _, cs := range tls.CipherSuites() {
		suites = append(suites, cs.ID)
	}
	suites = append(suites, tls.TLS_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_RSA_WITH_AES_128_GCM_SHA256)
	legacy := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, CipherSuites: suites}, //nolint:gosec // RSA key exchange only for legacyTLSHosts; certs verified
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		http:      &http.Client{Timeout: 30 * time.Second},
		legacy:    &http.Client{Timeout: 30 * time.Second, Transport: legacy},
		userAgent: userAgent,
		delay:     delay,
		last:      map[string]time.Time{},
		host:      map[string]*sync.Mutex{},
	}
}

// Get returns the body of a 200 response.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	unlock := c.wait(ctx, u.Host)
	defer unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml,application/json;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "th,en;q=0.8")
	hc := c.http
	if legacyTLSHosts[u.Host] {
		hc = c.legacy
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%s: %w (status %d)", rawURL, ErrBlocked, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: status %d", rawURL, resp.StatusCode)
	}
	return body, nil
}

// wait holds the host's lock and sleeps until delay has passed since its last request.
func (c *Client) wait(ctx context.Context, host string) func() {
	c.mu.Lock()
	m := c.host[host]
	if m == nil {
		m = &sync.Mutex{}
		c.host[host] = m
	}
	c.mu.Unlock()

	m.Lock()
	c.mu.Lock()
	next := c.last[host].Add(c.delay)
	c.mu.Unlock()
	if d := time.Until(next); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
		}
	}
	return func() {
		c.mu.Lock()
		c.last[host] = time.Now()
		c.mu.Unlock()
		m.Unlock()
	}
}
