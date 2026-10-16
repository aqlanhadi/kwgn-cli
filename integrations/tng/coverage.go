package tng

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Coverage records, per source, the days that were fetched completely: every
// window covering the day returned as many rows as the portal said it had. It
// tells a day with no transactions apart from a day nobody asked about, and
// shows which gaps the portal has since dropped for good.
//
// Coverage is "we asked and got everything listed", not proof that the portal
// lists everything. The card's running balance can prove that; the eWallet
// needs a balance taken from the app.

const dayLayout = "2006-01-02"

// settleDays: the newest days are not counted as covered, because the portal
// posts some rows late (card readers upload offline). The sync re-reads the
// last 14 days before the newest row, so a later sync confirms them.
const settleDays = 14

// retentionDays is how far back a source still serves rows. A fetch of older
// days is not counted (rows may already be gone), and a gap older than this is
// lost. The eWallet was measured on 2026-10-12: rows from 2025-09-14 (393 days)
// were gone, 2025-09-15 still served; 365 leaves a margin. Card and RFID keep
// history back to 2021, so their limit is unknown and not applied.
var retentionDays = map[Source]int{SourceEwallet: 365}

// Span is a run of days, both ends inclusive, as YYYY-MM-DD in Malaysia time.
type Span struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type SourceCoverage struct {
	Covered  []Span    `json:"covered"`
	LastSync time.Time `json:"lastSync"`
}

type Coverage map[Source]*SourceCoverage

// Complete reports whether a window returned every row the portal counted.
func (w Window) Complete(src Source) bool {
	if w.Refused {
		return false
	}
	// The eWallet API reports a count of 1 for a range with no rows.
	if src == SourceEwallet && w.Got == 0 && w.Reported == 1 {
		return true
	}
	return w.Got == w.Reported
}

// Record adds a sync's complete windows, fetched at the given time.
func (c Coverage) Record(src Source, wins []Window, fetched time.Time) {
	sc := c[src]
	if sc == nil {
		sc = &SourceCoverage{}
		c[src] = sc
	}
	horizon := dayStart(fetched).AddDate(0, 0, -settleDays)
	floor, hasFloor := retentionFloor(src, fetched)
	for _, w := range wins {
		if !w.Complete(src) {
			continue
		}
		from, to := dayStart(w.From), dayStart(w.To)
		if to.After(horizon) {
			to = horizon
		}
		if hasFloor && from.Before(floor) {
			from = floor
		}
		if from.After(to) {
			continue
		}
		sc.Covered = append(sc.Covered, Span{from.Format(dayLayout), to.Format(dayLayout)})
	}
	sc.Covered = mergeSpans(sc.Covered)
	sc.LastSync = fetched
}

func retentionFloor(src Source, now time.Time) (time.Time, bool) {
	n, ok := retentionDays[src]
	if !ok {
		return time.Time{}, false
	}
	return dayStart(now).AddDate(0, 0, -n), true
}

func parseDay(s string) time.Time {
	t, err := time.ParseInLocation(dayLayout, s, kl)
	if err != nil {
		return time.Time{}
	}
	return t
}

// mergeSpans sorts spans and joins any that overlap or touch.
func mergeSpans(spans []Span) []Span {
	sort.Slice(spans, func(i, j int) bool { return spans[i].From < spans[j].From })
	var out []Span
	for _, s := range spans {
		if n := len(out); n > 0 && !parseDay(s.From).After(parseDay(out[n-1].To).AddDate(0, 0, 1)) {
			if s.To > out[n-1].To {
				out[n-1].To = s.To
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// Gap is a run of days not covered. Lost means the portal no longer keeps
// those days, so no sync can fill it.
type Gap struct {
	Span
	Lost bool
}

// Gaps lists the uncovered days from the first covered day up to the newest
// day a sync now could cover. A gap that straddles the retention floor is
// split into its lost and still-fetchable parts.
func (c Coverage) Gaps(src Source, now time.Time) []Gap {
	sc := c[src]
	if sc == nil || len(sc.Covered) == 0 {
		return nil
	}
	var raw []Span
	for i := 1; i < len(sc.Covered); i++ {
		raw = append(raw, Span{
			parseDay(sc.Covered[i-1].To).AddDate(0, 0, 1).Format(dayLayout),
			parseDay(sc.Covered[i].From).AddDate(0, 0, -1).Format(dayLayout),
		})
	}
	horizon := dayStart(now).AddDate(0, 0, -settleDays)
	if last := parseDay(sc.Covered[len(sc.Covered)-1].To); last.Before(horizon) {
		raw = append(raw, Span{last.AddDate(0, 0, 1).Format(dayLayout), horizon.Format(dayLayout)})
	}

	floor, hasFloor := retentionFloor(src, now)
	var out []Gap
	for _, g := range raw {
		from, to := parseDay(g.From), parseDay(g.To)
		switch {
		case !hasFloor || !from.Before(floor):
			out = append(out, Gap{Span: g})
		case to.Before(floor):
			out = append(out, Gap{Span: g, Lost: true})
		default:
			out = append(out,
				Gap{Span: Span{g.From, floor.AddDate(0, 0, -1).Format(dayLayout)}, Lost: true},
				Gap{Span: Span{floor.Format(dayLayout), g.To}})
		}
	}
	return out
}

func coveragePath(dir string) string { return filepath.Join(dir, "coverage.json") }

// LoadCoverage reads coverage.json; a missing file is no coverage.
func LoadCoverage(dir string) (Coverage, error) {
	c := Coverage{}
	b, err := os.ReadFile(coveragePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("tng: reading %s: %w", coveragePath(dir), err)
	}
	return c, nil
}

// SaveCoverage writes coverage.json atomically.
func SaveCoverage(dir string, c Coverage) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	path := coveragePath(dir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
