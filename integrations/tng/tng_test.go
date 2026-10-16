package tng

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func init() { pause = 0 }

func day(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, kl)
	if err != nil {
		panic(err)
	}
	return t
}

func TestWindowsCoverRangeNewestFirst(t *testing.T) {
	ws := windows(day("2026-01-01"), day("2026-07-01"), 90)
	if got := ws[0][1]; !got.Equal(day("2026-07-01")) {
		t.Fatalf("first window ends %v, want 2026-07-01", got)
	}
	if got := ws[len(ws)-1][0]; !got.Equal(day("2026-01-01")) {
		t.Fatalf("last window starts %v, want 2026-01-01", got)
	}
	for i, w := range ws {
		if n := int(w[1].Sub(w[0]).Hours()/24) + 1; n > 90 {
			t.Errorf("window %d spans %d days", i, n)
		}
		if i > 0 && !ws[i-1][0].Equal(w[1].AddDate(0, 0, 1)) {
			t.Errorf("gap or overlap between window %d and %d", i-1, i)
		}
	}
}

// fakeEportal serves 25 eWallet rows in the newest window, pages of 10, and
// refuses anything starting before 2025-01-01.
func fakeEportal(t *testing.T) (*httptest.Server, *int) {
	refreshes := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/tngd/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "refresh-1" {
			w.WriteHeader(401)
			return
		}
		refreshes++
		fmt.Fprintf(w, `{"accessToken":"access-2","accessType":"tngd","accessTokenExpiry":%d}`, time.Now().Add(time.Hour).Unix())
	})
	mux.HandleFunc("/v1/users/transactions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "access-2" {
			w.WriteHeader(401)
			return
		}
		q := r.URL.Query()
		start, _ := time.Parse("02012006", q.Get("startDate"))
		if start.Before(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
			w.WriteHeader(422)
			return
		}
		total := 0
		if q.Get("endDate") == "01072026" {
			total = 25
		}
		page, _ := strconv.Atoi(q.Get("pageNo"))
		if page > 1 && q.Get("searchId") != "s1" {
			t.Errorf("page %d sent searchId %q", page, q.Get("searchId"))
		}
		var data []map[string]string
		for i := (page - 1) * 10; i < total && i < page*10; i++ {
			data = append(data, map[string]string{"walletRef": fmt.Sprintf("ref%02d", i), "txnDate": fmt.Sprintf("2026-06-%02dT10:00:00+08:00", i+1)})
		}
		totalPage := (total + 9) / 10
		json.NewEncoder(w).Encode(map[string]any{
			"searchId":   "s1",
			"pagination": map[string]any{"pageNo": page, "totalCount": total, "totalPage": totalPage, "hasNextPage": page < totalPage},
			"data":       data,
		})
	})
	return httptest.NewServer(mux), &refreshes
}

func TestFetchEwalletPagesRefreshesAndStopsWhenRefused(t *testing.T) {
	srv, refreshes := fakeEportal(t)
	defer srv.Close()
	c := NewClient(&Session{
		AccessToken: "access-1", RefreshToken: "refresh-1",
		AccessTokenExpiry:  time.Now().Add(-time.Minute).Unix(),
		RefreshTokenExpiry: time.Now().Add(time.Hour).Unix(),
	})
	c.EportalBase = srv.URL + "/v1/"
	saved := false
	c.OnSessionChanged = func(*Session) error { saved = true; return nil }

	rows, wins, err := c.FetchEwallet(context.Background(), day("2023-01-01"), day("2026-07-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 25 {
		t.Errorf("got %d rows, want 25", len(rows))
	}
	if *refreshes != 1 || !saved {
		t.Errorf("refreshes = %d, saved = %v; want one refresh, saved", *refreshes, saved)
	}
	if wins[0].Reported != 25 || wins[0].Got != 25 {
		t.Errorf("first window reported %d got %d", wins[0].Reported, wins[0].Got)
	}
	last := wins[len(wins)-1]
	if !last.Refused {
		t.Errorf("walk did not stop at the refused window: %+v", last)
	}
	if last.From.Before(day("2024-09-01")) {
		t.Errorf("walk went past the refused window to %v", last.From)
	}
}

func TestFetchEwalletExpiredLogin(t *testing.T) {
	c := NewClient(&Session{AccessToken: "a", RefreshToken: "r", RefreshTokenExpiry: time.Now().Add(-time.Second).Unix()})
	_, _, err := c.FetchEwallet(context.Background(), day("2026-01-01"), day("2026-01-02"))
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}

func TestMXLoginFollowsSSOLinkAndRetriesOn401(t *testing.T) {
	var mxLogins, cardCalls int
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/v1/auth/tngd/bind/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scene") != "CARD" {
			t.Errorf("scene = %q", r.URL.Query().Get("scene"))
		}
		fmt.Fprintf(w, `{"isBound":true,"bindingStatus":"ACTIVE","redirectUrl":"%s/card/dashboard?sessionId=S&bindingId=B&sign=G"}`, srv.URL)
	})
	mux.HandleFunc("/mx/api/v1/sso-login-authenticate", func(w http.ResponseWriter, r *http.Request) {
		var b map[string]string
		json.NewDecoder(r.Body).Decode(&b)
		if b["sessionId"] != "S" || b["bindingId"] != "B" || b["sign"] != "G" {
			t.Errorf("SSO body = %v", b)
		}
		mxLogins++
		fmt.Fprintf(w, `{"access_token":"mx-%d"}`, mxLogins)
	})
	mux.HandleFunc("/mx/api/v1/card/transactions", func(w http.ResponseWriter, r *http.Request) {
		cardCalls++
		if r.Header.Get("X-App-Version") == "" {
			t.Error("missing X-App-Version")
		}
		// The first token has idled out.
		if r.Header.Get("Authorization") == "Bearer mx-1" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"statusCode":"000","totalElement":2,"totalPage":1,"transactionInfo":[
			{"cardMfgNo":"1","txSeqNo":"10","txId":"a","txDatetime":"2026-01-02 10:00:00"},
			{"cardMfgNo":"1","txSeqNo":"10","txId":"a","txDatetime":"2026-01-02 10:00:00"}]}`)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(&Session{AccessToken: "x", RefreshToken: "y"})
	c.EportalBase = srv.URL + "/v1/"
	c.MXBase = srv.URL + "/mx/api/v1/"
	rows, wins, err := c.FetchCard(context.Background(), day("2026-01-01"), day("2026-01-31"))
	if err != nil {
		t.Fatal(err)
	}
	if mxLogins != 2 || cardCalls != 2 {
		t.Errorf("mx logins %d, card calls %d; want 2 and 2", mxLogins, cardCalls)
	}
	if len(rows) != 2 || wins[0].Reported != 2 {
		t.Errorf("rows %d, reported %d", len(rows), wins[0].Reported)
	}
	merged, res, err := Merge(SourceCard, nil, rows)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Errorf("server-side duplicate not folded: %d rows", res.Total)
	}
	_ = merged
}

func TestMergeKeepsOldHistoryAndCountsChanges(t *testing.T) {
	old := Rows{
		json.RawMessage(`{"walletRef":"a","txnDate":"2025-01-01T00:00:00+08:00","txnStatus":"PENDING"}`),
		json.RawMessage(`{"walletRef":"b","txnDate":"2025-02-01T00:00:00+08:00"}`),
	}
	in := Rows{
		json.RawMessage(`{"walletRef":"b", "txnDate":"2025-02-01T00:00:00+08:00"}`), // same, different spacing
		json.RawMessage(`{"walletRef":"a","txnDate":"2025-01-01T00:00:00+08:00","txnStatus":"SUCCESS"}`),
		json.RawMessage(`{"walletRef":"c","txnDate":"2024-12-01T00:00:00+08:00"}`),
	}
	out, res, err := Merge(SourceEwallet, old, in)
	if err != nil {
		t.Fatal(err)
	}
	if res != (MergeResult{Added: 1, Updated: 1, Total: 3}) {
		t.Errorf("result = %+v", res)
	}
	var first map[string]string
	json.Unmarshal(out[0], &first)
	if first["walletRef"] != "c" {
		t.Errorf("rows not sorted by date: first is %s", first["walletRef"])
	}

	dir := t.TempDir()
	if err := SaveRows(dir, SourceEwallet, out); err != nil {
		t.Fatal(err)
	}
	back, err := LoadRows(dir, SourceEwallet)
	if err != nil || len(back) != 3 {
		t.Fatalf("reload: %d rows, %v", len(back), err)
	}
}

func TestUnixFieldFormats(t *testing.T) {
	for in, want := range map[string]int64{
		`1792050000`:             1792050000,
		`"1792050000"`:           1792050000,
		`1792050000123`:          1792050000,
		`"2026-10-15T07:13:47Z"`: 1792048427,
		`null`:                   0,
	} {
		var u unixField
		if err := json.Unmarshal([]byte(in), &u); err != nil {
			t.Errorf("%s: %v", in, err)
		} else if int64(u) != want {
			t.Errorf("%s = %d, want %d", in, u, want)
		}
	}
}

func TestMXReusesSavedTokenAndReportsLostToken(t *testing.T) {
	var ssoCalls int
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/v1/auth/tngd/bind/verify", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"isBound":true,"bindingStatus":"ACTIVE","redirectUrl":"%s/x?sessionId=S&bindingId=B"}`, srv.URL)
	})
	mux.HandleFunc("/mx/api/v1/sso-login-authenticate", func(w http.ResponseWriter, r *http.Request) {
		ssoCalls++
		fmt.Fprint(w, `{"statusCode":"000","statusMessage":"Success"}`) // token already handed out
	})
	mux.HandleFunc("/mx/api/v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer saved" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"statusCode":"000","totalPage":1,"totalRecords":0,"transactions":[]}`)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(&Session{AccessToken: "x", RefreshToken: "y", MXAccessToken: "saved"})
	c.EportalBase = srv.URL + "/v1/"
	c.MXBase = srv.URL + "/mx/api/v1/"
	if _, _, err := c.FetchRFID(context.Background(), day("2026-01-01"), day("2026-01-31")); err != nil {
		t.Fatal(err)
	}
	if ssoCalls != 0 {
		t.Errorf("signed in again despite a saved token")
	}

	c.Session.MXAccessToken = ""
	_, _, err := c.FetchRFID(context.Background(), day("2026-01-01"), day("2026-01-31"))
	if err == nil || !strings.Contains(err.Error(), "kwgn tng login") {
		t.Errorf("lost token should tell the user to log in again, got %v", err)
	}
}
