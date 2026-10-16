package tng

import (
	"reflect"
	"testing"
	"time"
)

func win(from, to string, reported, got int) Window {
	return Window{From: day(from), To: day(to), Reported: reported, Got: got}
}

func TestCoverageCountsOnlyCompleteSettledWindows(t *testing.T) {
	c := Coverage{}
	fetched := day("2026-10-16").Add(11 * time.Hour)
	c.Record(SourceCard, []Window{
		win("2026-01-01", "2026-10-16", 30, 30), // newest 14 days not yet settled
		win("2025-01-01", "2025-12-31", 40, 39), // a row short: not covered
		win("2024-01-01", "2024-12-31", 10, 10),
		{From: day("2023-01-01"), To: day("2023-12-31"), Refused: true},
	}, fetched)
	want := []Span{{"2024-01-01", "2024-12-31"}, {"2026-01-01", "2026-10-02"}}
	if got := c[SourceCard].Covered; !reflect.DeepEqual(got, want) {
		t.Fatalf("covered %v, want %v", got, want)
	}
}

func TestCoverageEwalletEmptyWindowAndRetention(t *testing.T) {
	c := Coverage{}
	c.Record(SourceEwallet, []Window{
		win("2026-07-01", "2026-09-28", 1, 0), // the API's count of 1 for an empty range
		win("2025-07-01", "2025-12-31", 50, 50),
	}, day("2026-10-16"))
	// 365 days before 2026-10-16 is 2025-10-16: days before it are not claimed.
	want := []Span{{"2025-10-16", "2025-12-31"}, {"2026-07-01", "2026-09-28"}}
	if got := c[SourceEwallet].Covered; !reflect.DeepEqual(got, want) {
		t.Fatalf("covered %v, want %v", got, want)
	}
}

func TestCoverageMergesOverlappingAndTouchingSpans(t *testing.T) {
	c := Coverage{}
	c.Record(SourceRFID, []Window{win("2026-01-01", "2026-03-31", 5, 5)}, day("2026-10-16"))
	c.Record(SourceRFID, []Window{win("2026-04-01", "2026-05-31", 5, 5), win("2026-03-01", "2026-04-15", 5, 5)}, day("2026-10-16"))
	want := []Span{{"2026-01-01", "2026-05-31"}}
	if got := c[SourceRFID].Covered; !reflect.DeepEqual(got, want) {
		t.Fatalf("covered %v, want %v", got, want)
	}
}

func TestGapsSplitAtRetentionFloor(t *testing.T) {
	c := Coverage{SourceEwallet: {Covered: []Span{
		{"2025-01-01", "2025-06-30"},
		{"2025-12-01", "2026-03-31"},
	}}}
	got := c.Gaps(SourceEwallet, day("2026-10-16"))
	want := []Gap{
		{Span: Span{"2025-07-01", "2025-10-15"}, Lost: true},
		{Span: Span{"2025-10-16", "2025-11-30"}},
		{Span: Span{"2026-04-01", "2026-10-02"}}, // since the last sync, up to the settled horizon
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gaps %v, want %v", got, want)
	}
}

func TestGapsWithoutRetentionAreNeverLost(t *testing.T) {
	c := Coverage{SourceCard: {Covered: []Span{{"2021-01-01", "2021-12-31"}, {"2026-01-01", "2026-10-02"}}}}
	got := c.Gaps(SourceCard, day("2026-10-16"))
	want := []Gap{{Span: Span{"2022-01-01", "2025-12-31"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gaps %v, want %v", got, want)
	}
}

func TestCoverageSaveLoad(t *testing.T) {
	dir := t.TempDir()
	c, err := LoadCoverage(dir)
	if err != nil || len(c) != 0 {
		t.Fatalf("empty dir: %v, %v", c, err)
	}
	c.Record(SourceCard, []Window{win("2026-01-01", "2026-06-30", 3, 3)}, day("2026-10-16"))
	if err := SaveCoverage(dir, c); err != nil {
		t.Fatal(err)
	}
	back, err := LoadCoverage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back[SourceCard].Covered, c[SourceCard].Covered) || !back[SourceCard].LastSync.Equal(c[SourceCard].LastSync) {
		t.Fatalf("round trip: got %+v, want %+v", back[SourceCard], c[SourceCard])
	}
}
