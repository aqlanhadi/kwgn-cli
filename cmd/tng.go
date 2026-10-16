package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/integrations/tng"
	"github.com/spf13/cobra"
)

var (
	tngSessionPath string
	tngOutDir      string
	tngFrom        string
	tngTo          string
	tngSources     string
	tngOpenImage   bool
)

var tngCmd = &cobra.Command{
	Use:          "tng",
	SilenceUsage: true,
	Short:        "Pull transaction history from the Touch 'n Go consumer portal",
	Long: `Signs in to the Touch 'n Go consumer portal with a QR code scanned in the
TNG eWallet app, then downloads eWallet, Touch 'n Go card and RFID history.

  kwgn tng login     show the QR, wait for the scan, save the login
  kwgn tng sync      download history and merge it into --out
  kwgn tng status    show when the saved login expires
  kwgn tng logout    end the session and delete the saved login`,
}

var tngLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in by scanning a QR code with the TNG eWallet app",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		c := tng.NewClient(nil)
		qrDir, err := os.MkdirTemp("", "kwgn-tng-login-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(qrDir)

		for attempt := 1; ; attempt++ {
			q, err := c.InitQRLogin(ctx)
			if err != nil {
				return err
			}
			png, err := q.PNG()
			if err != nil {
				return err
			}
			// A new file per QR: image viewers do not reload a file that changes
			// on disk, so reusing one name leaves the expired code on screen.
			qrPath := filepath.Join(qrDir, fmt.Sprintf("tng-login-%d.png", attempt))
			if err := os.WriteFile(qrPath, png, 0o600); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr)
			shown := q.WriteTerminalQR(os.Stderr)
			fmt.Fprintf(os.Stderr, "Scan the QR in the TNG eWallet app (Scan > Login). It is valid until %s.\n  image: %s\n",
				q.Expires().Local().Format("15:04:05"), qrPath)
			if tngOpenImage || !shown {
				openFile(qrPath)
			}

			status, err := waitForScan(ctx, c, q)
			if err != nil {
				return err
			}
			if status == tng.StatusSuccess {
				break
			}
			fmt.Fprintln(os.Stderr, "QR expired; showing a new one.")
		}

		if err := tng.SaveSession(tngSessionPath, c.Session); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Signed in. Saved to %s\n", tngSessionPath)
		printSessionExpiry(c.Session)
		return nil
	},
}

// waitForScan polls verify-login until the QR is scanned or expires.
func waitForScan(ctx context.Context, c *tng.Client, q *tng.QRLogin) (string, error) {
	deadline := q.Expires().Add(5 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		status, _, err := c.VerifyQRLogin(ctx, q.RequestID)
		if err != nil {
			return "", err
		}
		if status != last && status != tng.StatusPending {
			fmt.Fprintf(os.Stderr, "  login status: %s\n", status)
		}
		last = status
		switch status {
		case tng.StatusSuccess:
			return status, nil
		case tng.StatusFailed:
			return "", errors.New("tng: login was rejected in the app")
		case tng.StatusExpired:
			return status, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return tng.StatusExpired, nil
}

var tngSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Download TNG history and merge it into the output directory",
	Long: `Downloads eWallet, card and RFID history and merges it into one JSON file per
source in --out, keyed by each source's transaction ID.

With no --from, a source with nothing on disk is backfilled as far as the
portal allows; otherwise the sync re-reads the last 14 days before the newest
row on disk, to pick up transactions the portal posts late.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := tng.LoadSession(tngSessionPath)
		if err != nil {
			return err
		}
		if s.RefreshExpired(time.Now()) {
			return tng.ErrSessionExpired
		}
		c := tng.NewClient(s)
		c.OnSessionChanged = func(s *tng.Session) error { return tng.SaveSession(tngSessionPath, s) }

		to := time.Now()
		if tngTo != "" {
			if to, err = time.Parse("2006-01-02", tngTo); err != nil {
				return fmt.Errorf("--to: %w", err)
			}
		}

		cov, err := tng.LoadCoverage(tngOutDir)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		failed := false
		var synced []tng.Source
		for _, src := range strings.Split(tngSources, ",") {
			src := tng.Source(strings.TrimSpace(src))
			if err := syncSource(ctx, c, cov, src, to); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", src, err)
				failed = true
			}
			synced = append(synced, src)
		}
		for _, src := range synced {
			printGaps(src, cov.Gaps(src, time.Now()))
		}
		if failed {
			return errors.New("tng: sync finished with errors")
		}
		return nil
	},
}

func syncSource(ctx context.Context, c *tng.Client, cov tng.Coverage, src tng.Source, to time.Time) error {
	existing, err := tng.LoadRows(tngOutDir, src)
	if err != nil {
		return err
	}
	from, err := syncFrom(src, existing)
	if err != nil {
		return err
	}

	var rows tng.Rows
	var wins []tng.Window
	switch src {
	case tng.SourceEwallet:
		rows, wins, err = c.FetchEwallet(ctx, from, to)
	case tng.SourceCard:
		rows, wins, err = c.FetchCard(ctx, from, to)
	case tng.SourceRFID:
		rows, wins, err = c.FetchRFID(ctx, from, to)
	default:
		return fmt.Errorf("unknown source (want ewallet, card or rfid)")
	}
	if err != nil {
		// Keep what was fetched before the failure; the next sync fills the rest.
		if len(rows) == 0 {
			return err
		}
		fmt.Fprintf(os.Stderr, "%s: stopped early: %v\n", src, err)
	}

	merged, res, mergeErr := tng.Merge(src, existing, rows)
	if mergeErr != nil {
		return mergeErr
	}
	if saveErr := tng.SaveRows(tngOutDir, src, merged); saveErr != nil {
		return saveErr
	}
	// Coverage is saved after the rows it vouches for.
	cov.Record(src, wins, time.Now())
	if saveErr := tng.SaveCoverage(tngOutDir, cov); saveErr != nil {
		return saveErr
	}

	for _, w := range wins {
		note := ""
		switch {
		case w.Refused:
			note = "  (older than the portal keeps)"
		case !w.Complete(src):
			note = "  INCOMPLETE, not counted as covered"
		}
		if verbose || note != "" {
			fmt.Fprintf(os.Stderr, "  %s %s..%s  reported %d, got %d%s\n", src,
				w.From.Format("2006-01-02"), w.To.Format("2006-01-02"), w.Reported, w.Got, note)
		}
	}
	fmt.Printf("%-7s %s..%s  fetched %d, new %d, changed %d, on disk %d\n", src,
		from.Format("2006-01-02"), to.Format("2006-01-02"), len(rows), res.Added, res.Updated, res.Total)
	return err
}

// syncFrom picks where a source's sync starts.
func syncFrom(src tng.Source, existing tng.Rows) (time.Time, error) {
	if tngFrom != "" {
		return time.Parse("2006-01-02", tngFrom)
	}
	if latest, ok := latestDate(src, existing); ok {
		return latest.AddDate(0, 0, -14), nil
	}
	// Nothing on disk: go back as far as the portal might keep.
	if src == tng.SourceEwallet {
		return time.Now().AddDate(-3, 0, 0), nil // the walk stops at the first refused window
	}
	return time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC), nil
}

func latestDate(src tng.Source, rows tng.Rows) (time.Time, bool) {
	if len(rows) == 0 {
		return time.Time{}, false
	}
	var r map[string]any
	if err := json.Unmarshal(rows[len(rows)-1], &r); err != nil {
		return time.Time{}, false
	}
	field := map[tng.Source]string{tng.SourceEwallet: "txnDate", tng.SourceCard: "txDatetime", tng.SourceRFID: "transactionDateTime"}[src]
	s, _ := r[field].(string)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

var tngStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the saved login is still valid, and which days are covered",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := tng.LoadSession(tngSessionPath)
		switch {
		case errors.Is(err, tng.ErrNotLoggedIn):
			fmt.Println("login: none saved")
		case err != nil:
			return err
		default:
			fmt.Println("login:")
			printSessionExpiry(s)
		}

		cov, err := tng.LoadCoverage(tngOutDir)
		if err != nil {
			return err
		}
		for _, src := range []tng.Source{tng.SourceEwallet, tng.SourceCard, tng.SourceRFID} {
			sc := cov[src]
			if sc == nil || len(sc.Covered) == 0 {
				fmt.Printf("%-7s no coverage recorded; run `kwgn tng sync`\n", src)
				continue
			}
			fmt.Printf("%-7s last sync %s\n", src, sc.LastSync.Local().Format("2006-01-02 15:04"))
			for _, sp := range sc.Covered {
				fmt.Printf("          covered %s..%s\n", sp.From, sp.To)
			}
			printGaps(src, cov.Gaps(src, time.Now()))
		}
		return nil
	},
}

// printGaps names each uncovered run of days, and how to fill it if it can be.
func printGaps(src tng.Source, gaps []tng.Gap) {
	for _, g := range gaps {
		if g.Lost {
			fmt.Printf("          LOST    %s..%s  older than the portal keeps; only an earlier export has these days\n", g.From, g.To)
		} else {
			fmt.Printf("          gap     %s..%s  run: kwgn tng sync --sources %s --from %s --to %s\n", g.From, g.To, src, g.From, g.To)
		}
	}
}

var tngLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "End the session and delete the saved login",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := tng.LoadSession(tngSessionPath)
		if errors.Is(err, tng.ErrNotLoggedIn) {
			return nil
		}
		if err != nil {
			return err
		}
		_ = tng.NewClient(s).Logout(cmd.Context())
		return os.Remove(tngSessionPath)
	},
}

func printSessionExpiry(s *tng.Session) {
	show := func(label string, unix int64) {
		if unix == 0 {
			fmt.Fprintf(os.Stderr, "  %s: expiry not given\n", label)
			return
		}
		t := time.Unix(unix, 0)
		state := "valid for " + time.Until(t).Round(time.Minute).String()
		if time.Now().After(t) {
			state = "expired"
		}
		fmt.Fprintf(os.Stderr, "  %s: %s (%s)\n", label, t.Local().Format("2006-01-02 15:04"), state)
	}
	show("access token ", s.AccessTokenExpiry)
	show("login (refresh token)", s.RefreshTokenExpiry)
}

func openFile(path string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", path)
	case "linux":
		c = exec.Command("xdg-open", path)
	default:
		return
	}
	_ = c.Start()
}

func init() {
	rootCmd.AddCommand(tngCmd)
	tngCmd.AddCommand(tngLoginCmd, tngSyncCmd, tngStatusCmd, tngLogoutCmd)

	defaultSession, _ := tng.DefaultSessionPath()
	home, _ := os.UserHomeDir()
	tngCmd.PersistentFlags().StringVar(&tngSessionPath, "session", defaultSession, "where the login is saved")
	tngLoginCmd.Flags().BoolVar(&tngOpenImage, "open", false, "also open the QR as an image (it always opens if the terminal cannot show it)")
	tngCmd.PersistentFlags().StringVar(&tngOutDir, "out", filepath.Join(home, "kwgn-data", "tng"), "directory holding ewallet.json, card.json, rfid.json and coverage.json")
	tngSyncCmd.Flags().StringVar(&tngFrom, "from", "", "start date YYYY-MM-DD (default: backfill, or 14 days before the newest row on disk)")
	tngSyncCmd.Flags().StringVar(&tngTo, "to", "", "end date YYYY-MM-DD (default: today)")
	tngSyncCmd.Flags().StringVar(&tngSources, "sources", "ewallet,card,rfid", "comma-separated sources to sync")
}
