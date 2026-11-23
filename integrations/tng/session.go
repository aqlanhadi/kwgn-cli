package tng

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotLoggedIn    = errors.New("tng: not logged in; run `kwgn tng login`")
	ErrSessionExpired = errors.New("tng: login has expired; run `kwgn tng login` again")
)

// Session is the eWallet login. Expiry fields are unix seconds, as the API
// returns them; zero means unknown.
type Session struct {
	AccessToken        string `json:"accessToken"`
	RefreshToken       string `json:"refreshToken"`
	AccessType         string `json:"accessType"`
	AccessTokenExpiry  int64  `json:"accessTokenExpiry"`
	RefreshTokenExpiry int64  `json:"refreshTokenExpiry"`
	// The card portal token, valid for this eWallet login only.
	MXAccessToken  string `json:"mxAccessToken,omitempty"`
	MXRefreshToken string `json:"mxRefreshToken,omitempty"`
}

// expirySlack renews a token a little before the server would reject it.
const expirySlack = 30 * time.Second

func (s *Session) AccessExpired(now time.Time) bool {
	return s.AccessTokenExpiry > 0 && now.Add(expirySlack).Unix() >= s.AccessTokenExpiry
}

func (s *Session) RefreshExpired(now time.Time) bool {
	return s.RefreshTokenExpiry > 0 && now.Unix() >= s.RefreshTokenExpiry
}

// unixField accepts an expiry as a number, a numeric string or an RFC 3339
// string; the portal's JavaScript treats it as unix seconds.
type unixField int64

func (u *unixField) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*u = 0
		return nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		if n > 1e12 { // milliseconds
			n /= 1000
		}
		*u = unixField(n)
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("tng: unrecognised expiry %q", s)
	}
	*u = unixField(t.Unix())
	return nil
}

type tokenResponse struct {
	Status             string    `json:"status"`
	AccessToken        string    `json:"accessToken"`
	RefreshToken       string    `json:"refreshToken"`
	AccessType         string    `json:"accessType"`
	AccessTokenExpiry  unixField `json:"accessTokenExpiry"`
	RefreshTokenExpiry unixField `json:"refreshTokenExpiry"`
}

// QRLogin is one login attempt. The QR expires after about a minute.
type QRLogin struct {
	RequestID string `json:"requestId"`
	QRCode    string `json:"qrcode"` // data:image/png;base64,...
	Deeplink  string `json:"deeplink"`
	ExpiredAt string `json:"expiredAt"`
}

// PNG decodes the QR image.
func (q *QRLogin) PNG() ([]byte, error) {
	_, b64, ok := strings.Cut(q.QRCode, "base64,")
	if !ok {
		return nil, fmt.Errorf("tng: qrcode is not a base64 data URL")
	}
	return base64.StdEncoding.DecodeString(b64)
}

// Expires returns when the QR stops being accepted.
func (q *QRLogin) Expires() time.Time {
	t, err := time.Parse(time.RFC3339, q.ExpiredAt)
	if err != nil {
		return time.Now().Add(time.Minute)
	}
	return t
}

// InitQRLogin starts a login and returns the QR to scan.
func (c *Client) InitQRLogin(ctx context.Context) (*QRLogin, error) {
	var q QRLogin
	if err := c.do(ctx, "POST", c.EportalBase+"auth/tngd/init", eportalHeader(""), struct{}{}, &q); err != nil {
		return nil, err
	}
	if q.RequestID == "" || q.QRCode == "" {
		return nil, fmt.Errorf("tng: init returned no QR")
	}
	return &q, nil
}

// LoginStatus values returned by verify-login.
const (
	StatusPending = "PENDING"
	StatusSuccess = "SUCCESS"
	StatusFailed  = "FAILED"
	StatusExpired = "EXPIRED"
)

// VerifyQRLogin checks whether the QR has been scanned. It returns the status
// and, once it is SUCCESS, the new session (also stored on the client).
func (c *Client) VerifyQRLogin(ctx context.Context, requestID string) (string, *Session, error) {
	var r tokenResponse
	err := c.do(ctx, "POST", c.EportalBase+"auth/tngd/verify-login", eportalHeader(""),
		map[string]string{"requestId": requestID}, &r)
	if err != nil {
		return "", nil, err
	}
	if r.AccessToken == "" || r.RefreshToken == "" {
		if r.Status == "" {
			r.Status = StatusPending
		}
		return r.Status, nil, nil
	}
	c.Session = &Session{
		AccessToken:        r.AccessToken,
		RefreshToken:       r.RefreshToken,
		AccessType:         r.AccessType,
		AccessTokenExpiry:  int64(r.AccessTokenExpiry),
		RefreshTokenExpiry: int64(r.RefreshTokenExpiry),
	}
	return StatusSuccess, c.Session, nil
}

// Refresh renews the access token. The portal sends the refresh token as the
// Authorization header and the old access token in the body.
func (c *Client) Refresh(ctx context.Context) error {
	if c.Session == nil {
		return ErrNotLoggedIn
	}
	var r tokenResponse
	err := c.do(ctx, "POST", c.EportalBase+"auth/tngd/refresh", eportalHeader(c.Session.RefreshToken),
		map[string]string{"accessToken": c.Session.AccessToken}, &r)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == 401 {
			return ErrSessionExpired
		}
		return err
	}
	if r.AccessToken == "" {
		return fmt.Errorf("tng: refresh returned no access token")
	}
	c.Session.AccessToken = r.AccessToken
	c.Session.AccessTokenExpiry = int64(r.AccessTokenExpiry)
	if r.AccessType != "" {
		c.Session.AccessType = r.AccessType
	}
	return c.sessionChanged()
}

// Logout ends the session on the server. Errors are ignored by the portal too.
func (c *Client) Logout(ctx context.Context) error {
	if c.Session == nil {
		return nil
	}
	return c.do(ctx, "POST", c.EportalBase+"auth/logout", eportalHeader(c.Session.AccessToken), struct{}{}, nil)
}

// DefaultSessionPath is ~/.config/kwgn/tng-session.json (or the OS equivalent).
func DefaultSessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kwgn", "tng-session.json"), nil
}

func LoadSession(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("tng: reading %s: %w", path, err)
	}
	return &s, nil
}

// SaveSession writes the session readable by the owner only.
func SaveSession(path string, s *Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
