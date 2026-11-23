package tng

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Source names the three histories the portal keeps.
type Source string

const (
	SourceEwallet Source = "ewallet"
	SourceCard    Source = "card"
	SourceRFID    Source = "rfid"
)

// Rows are kept exactly as the API returned them, so nothing is lost before
// anyone decides how to read them.
type Rows []json.RawMessage

// Window is one date range that was fetched, with the count the server
// reported, so a caller can prove nothing was skipped.
type Window struct {
	From, To time.Time
	Reported int
	Got      int
	// Refused is set when the server rejected the range as too old.
	Refused bool
}

var kl = mustKL()

func mustKL() *time.Location {
	loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		return time.FixedZone("MYT", 8*3600)
	}
	return loc
}

// ewalletTxnTypes is every filter the Transactions page offers ("Select All").
const ewalletTxnTypes = "RFID,VisaPrepaid,PayDirect,TOLL_PAYMENT,PAYMENT,EWALLET,DUITNOW_TRANSFER,RELOAD,REWARDS,REFUND,FUND_OUT,DIRECT_CREDIT,CASHBACK_PACKET,P2P_MONEY_PACKET,GOLD_INVESTMENT,PARTNER_CREDIT"

const (
	// The eWallet API accepts at most 90 days per query and ignores page
	// sizes above 10.
	ewalletWindowDays = 90
	ewalletPageSize   = 10
	// The card and RFID APIs accept a year per query and large pages.
	mxWindowDays = 365
	mxPageSize   = 1000
)

// windows splits [from, to] into consecutive day ranges of at most n days,
// newest first, so a walk backwards can stop at the first refused range.
func windows(from, to time.Time, n int) [][2]time.Time {
	from = dayStart(from)
	to = dayStart(to)
	var out [][2]time.Time
	for end := to; !end.Before(from); {
		start := end.AddDate(0, 0, -(n - 1))
		if start.Before(from) {
			start = from
		}
		out = append(out, [2]time.Time{start, end})
		end = start.AddDate(0, 0, -1)
	}
	return out
}

func dayStart(t time.Time) time.Time {
	t = t.In(kl)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, kl)
}

// pause spaces requests out; the portal has no published limits.
var pause = 200 * time.Millisecond

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type ewalletPage struct {
	SearchID   string `json:"searchId"`
	Pagination struct {
		PageNo      int  `json:"pageNo"`
		TotalCount  int  `json:"totalCount"`
		TotalPage   int  `json:"totalPage"`
		HasNextPage bool `json:"hasNextPage"`
	} `json:"pagination"`
	Data Rows `json:"data"`
}

// FetchEwallet walks [from, to] in 90-day windows, newest first. A 422 means
// the range is older than the portal keeps; the walk stops there.
func (c *Client) FetchEwallet(ctx context.Context, from, to time.Time) (Rows, []Window, error) {
	var all Rows
	var log []Window
	for _, w := range windows(from, to, ewalletWindowDays) {
		win := Window{From: w[0], To: w[1]}
		searchID := ""
		for page := 1; ; page++ {
			q := url.Values{}
			if searchID != "" {
				q.Set("searchId", searchID)
			}
			q.Set("startDate", w[0].Format("02012006"))
			q.Set("endDate", w[1].Format("02012006"))
			q.Set("pageNo", strconv.Itoa(page))
			q.Set("pageSize", strconv.Itoa(ewalletPageSize))
			q.Set("txnTypes", ewalletTxnTypes)
			var p ewalletPage
			err := c.eportal(ctx, "GET", "users/transactions", q, nil, &p)
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
				win.Refused = true
				return all, append(log, win), nil
			}
			if err != nil {
				return all, log, fmt.Errorf("eWallet %s..%s page %d: %w", w[0].Format("2006-01-02"), w[1].Format("2006-01-02"), page, err)
			}
			if p.SearchID != "" {
				searchID = p.SearchID
			}
			win.Reported = p.Pagination.TotalCount
			win.Got += len(p.Data)
			all = append(all, p.Data...)
			if err := sleep(ctx, pause); err != nil {
				return all, log, err
			}
			if !p.Pagination.HasNextPage || len(p.Data) == 0 {
				break
			}
		}
		log = append(log, win)
	}
	return all, log, nil
}

// BindRedirect asks the eWallet API for the single-sign-on link into the card
// portal. scene is "CARD" or "RFID".
func (c *Client) BindRedirect(ctx context.Context, scene string) (string, error) {
	var r struct {
		IsBound       bool   `json:"isBound"`
		BindingStatus string `json:"bindingStatus"`
		RedirectURL   string `json:"redirectUrl"`
	}
	if err := c.eportal(ctx, "GET", "auth/tngd/bind/verify", url.Values{"scene": {scene}}, nil, &r); err != nil {
		return "", err
	}
	if !r.IsBound || r.RedirectURL == "" {
		return "", fmt.Errorf("tng: %s portal is not linked to this eWallet (status %q)", scene, r.BindingStatus)
	}
	return r.RedirectURL, nil
}

// ErrMXNeedsOTP means the card portal asked for a one-time password, which
// only the browser flow can answer.
var ErrMXNeedsOTP = errors.New("tng: the card portal is asking for a one-time password; open it once in the browser, then sync again")

// LoginMX follows the SSO link: the card portal reads sessionId, bindingId and
// sign from it and trades them for its own token. It hands the token out only
// on the first sign-in of an eWallet login and afterwards answers "000" with
// no token, expecting the caller to have kept it, so the token is saved on the
// session.
func (c *Client) LoginMX(ctx context.Context) error {
	if c.Session == nil {
		return ErrNotLoggedIn
	}
	var lastErr error
	for _, scene := range []string{"CARD", "RFID"} {
		link, err := c.BindRedirect(ctx, scene)
		if err != nil {
			lastErr = err
			continue
		}
		u, err := url.Parse(link)
		if err != nil {
			return fmt.Errorf("tng: bad SSO link: %w", err)
		}
		q := u.Query()
		body := map[string]string{"sessionId": q.Get("sessionId")}
		for _, k := range []string{"bindingId", "loginRequestId", "sign"} {
			if v := q.Get(k); v != "" {
				body[k] = v
			}
		}
		var raw json.RawMessage
		if err := c.do(ctx, "POST", c.MXBase+"sso-login-authenticate", c.mxHeader(), body, &raw); err != nil {
			lastErr = err
			continue
		}
		var r struct {
			StatusCode   string `json:"statusCode"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.Unmarshal(raw, &r)
		if r.StatusCode == "001" {
			return ErrMXNeedsOTP
		}
		if r.AccessToken == "" {
			lastErr = fmt.Errorf("tng: the card portal session for this login was already opened and its token was not kept; run `kwgn tng login` again (%s replied %s)", scene, redactTokens(raw))
			continue
		}
		c.Session.MXAccessToken = r.AccessToken
		c.Session.MXRefreshToken = r.RefreshToken
		return c.sessionChanged()
	}
	return lastErr
}

// redactTokens shows a response's shape without any token values.
func redactTokens(raw json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		s := string(raw)
		if len(s) > 200 {
			s = s[:200] + "..."
		}
		return s
	}
	for k := range m {
		if strings.Contains(strings.ToLower(k), "token") {
			m[k] = "<redacted>"
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func (c *Client) mxHeader() http.Header {
	h := http.Header{}
	h.Set("Origin", mxOrigin)
	h.Set("Referer", mxOrigin+"/")
	h.Set("X-App-Version", mxAppVersion)
	h.Set("X-Request-Id", "UCP_WEB"+time.Now().Format("20060102150405.000"))
	if c.Session != nil && c.Session.MXAccessToken != "" {
		h.Set("Authorization", "Bearer "+c.Session.MXAccessToken)
	}
	return h
}

func (c *Client) mx(ctx context.Context, path string, body, out any) error {
	if c.Session == nil {
		return ErrNotLoggedIn
	}
	if c.Session.MXAccessToken == "" {
		if err := c.LoginMX(ctx); err != nil {
			return err
		}
	}
	err := c.do(ctx, "POST", c.MXBase+path, c.mxHeader(), body, out)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
		// The card portal session idles out after about ten minutes; sign in again once.
		c.Session.MXAccessToken = ""
		if err := c.LoginMX(ctx); err != nil {
			return err
		}
		err = c.do(ctx, "POST", c.MXBase+path, c.mxHeader(), body, out)
	}
	return err
}

type mxStatus struct {
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

func (s mxStatus) err() error {
	if s.StatusCode != "000" {
		return fmt.Errorf("tng: card portal status %s: %s", s.StatusCode, s.StatusMessage)
	}
	return nil
}

// FetchCard returns every Touch 'n Go card transaction in [from, to], for all
// cards on the account.
func (c *Client) FetchCard(ctx context.Context, from, to time.Time) (Rows, []Window, error) {
	var all Rows
	var log []Window
	for _, w := range windows(from, to, mxWindowDays) {
		win := Window{From: w[0], To: w[1]}
		for page, totalPage := 0, 1; page < totalPage; page++ {
			body := map[string]any{
				"dateType": "T", "startDate": w[0].Format("2006-01-02"), "endDate": w[1].Format("2006-01-02"),
				"pagination": true, "dir": "desc", "cardMfgNo": "", "cardNo": "",
				"page": page, "size": mxPageSize, "sort": "transactionDatetime", "appSector": "", "txTypeInd": "",
			}
			var r struct {
				mxStatus
				TotalElement    int  `json:"totalElement"`
				TotalPage       int  `json:"totalPage"`
				TransactionInfo Rows `json:"transactionInfo"`
			}
			if err := c.mx(ctx, "card/transactions", body, &r); err != nil {
				return all, log, fmt.Errorf("card %s..%s: %w", w[0].Format("2006-01-02"), w[1].Format("2006-01-02"), err)
			}
			if err := r.err(); err != nil {
				return all, log, err
			}
			totalPage = r.TotalPage
			win.Reported = r.TotalElement
			win.Got += len(r.TransactionInfo)
			all = append(all, r.TransactionInfo...)
			if err := sleep(ctx, pause); err != nil {
				return all, log, err
			}
		}
		log = append(log, win)
	}
	return all, log, nil
}

// FetchRFID returns every RFID toll in [from, to].
func (c *Client) FetchRFID(ctx context.Context, from, to time.Time) (Rows, []Window, error) {
	var all Rows
	var log []Window
	for _, w := range windows(from, to, mxWindowDays) {
		win := Window{From: w[0], To: w[1]}
		for page, totalPage := 1, 1; page <= totalPage; page++ {
			body := map[string]any{
				"startDate": w[0].Format("2006-01-02"), "endDate": w[1].Format("2006-01-02"),
				"businessSector": "", "vehiclePlateNumber": "", "page": page, "limit": mxPageSize,
				"sort": "postedDateTime", "dir": "desc", "vehicleNo": nil,
			}
			var r struct {
				mxStatus
				TotalPage    int  `json:"totalPage"`
				TotalRecords int  `json:"totalRecords"`
				Transactions Rows `json:"transactions"`
			}
			if err := c.mx(ctx, "transactions", body, &r); err != nil {
				return all, log, fmt.Errorf("RFID %s..%s: %w", w[0].Format("2006-01-02"), w[1].Format("2006-01-02"), err)
			}
			if err := r.err(); err != nil {
				return all, log, err
			}
			totalPage = r.TotalPage
			win.Reported = r.TotalRecords
			win.Got += len(r.Transactions)
			all = append(all, r.Transactions...)
			if err := sleep(ctx, pause); err != nil {
				return all, log, err
			}
		}
		log = append(log, win)
	}
	return all, log, nil
}
