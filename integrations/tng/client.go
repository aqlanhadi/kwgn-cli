// Package tng pulls transaction history from the Touch 'n Go consumer portal.
//
// The portal is two sites with their own APIs:
//
//   - consumer.touchngo.com.my talks to api-eportal.tngdigital.com.my and holds
//     the eWallet history. Login is a QR code scanned with the TNG eWallet app;
//     it yields an access token and a refresh token.
//   - consumer2.touchngo.com.my ("MX") holds the Touch 'n Go card and RFID
//     history. It is entered through a single-sign-on link that the eWallet
//     API hands out, so it needs no login of its own.
//
// None of this is a published API. The shapes here were read from the portal's
// own JavaScript and confirmed against live responses in October 2026.
package tng

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

const (
	DefaultEportalBase = "https://api-eportal.tngdigital.com.my/v1/"
	DefaultMXBase      = "https://consumer2.touchngo.com.my/mx/api/v1/"

	eportalOrigin = "https://consumer.touchngo.com.my"
	mxOrigin      = "https://consumer2.touchngo.com.my"
	// mxAppVersion is the version the card portal's own bundle sends.
	mxAppVersion = "0.0.1-stg"
	userAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"
)

// Client calls both portal APIs. Session holds the eWallet login and the card
// portal token obtained through it.
type Client struct {
	HTTP        *http.Client
	EportalBase string
	MXBase      string
	Session     *Session
	// OnSessionChanged is called whenever a token is renewed or obtained, so
	// the caller can persist it.
	OnSessionChanged func(*Session) error
}

func (c *Client) sessionChanged() error {
	if c.OnSessionChanged != nil {
		return c.OnSessionChanged(c.Session)
	}
	return nil
}

func NewClient(s *Session) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		HTTP:        &http.Client{Timeout: 60 * time.Second, Jar: jar},
		EportalBase: DefaultEportalBase,
		MXBase:      DefaultMXBase,
		Session:     s,
	}
}

// APIError is a non-2xx response from either portal.
type APIError struct {
	Status int
	Path   string
	Body   string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "..."
	}
	return fmt.Sprintf("tng: %s returned %d: %s", e.Path, e.Status, body)
}

// do sends a request and decodes a JSON response into out (if non-nil).
func (c *Client) do(ctx context.Context, method, rawURL string, header http.Header, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		path := rawURL
		if u, err := url.Parse(rawURL); err == nil {
			path = u.Path
		}
		return &APIError{Status: resp.StatusCode, Path: path, Body: string(data)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("tng: decoding %s: %w", rawURL, err)
	}
	return nil
}

func eportalHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Origin", eportalOrigin)
	h.Set("Referer", eportalOrigin+"/")
	if token != "" {
		h.Set("Authorization", token)
	}
	return h
}

// eportal calls the eWallet API, renewing the access token first when it has
// expired, the way the portal's tokenExpiryHandler does.
func (c *Client) eportal(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if c.Session == nil {
		return ErrNotLoggedIn
	}
	if c.Session.RefreshExpired(time.Now()) {
		return ErrSessionExpired
	}
	if c.Session.AccessExpired(time.Now()) {
		if err := c.Refresh(ctx); err != nil {
			return err
		}
	}
	u := c.EportalBase + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.do(ctx, method, u, eportalHeader(c.Session.AccessToken), body, out)
}
