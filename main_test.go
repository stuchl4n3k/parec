package main

import (
	"testing"
	"time"
)

func TestMonthsBackCutoff(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		now    time.Time
		months int
		want   time.Time
	}{
		{time.Date(2026, 5, 11, 14, 0, 0, 0, loc), 1, time.Date(2026, 5, 1, 0, 0, 0, 0, loc)},
		{time.Date(2026, 5, 11, 14, 0, 0, 0, loc), 2, time.Date(2026, 4, 1, 0, 0, 0, 0, loc)},
		{time.Date(2026, 5, 11, 14, 0, 0, 0, loc), 6, time.Date(2025, 12, 1, 0, 0, 0, 0, loc)},
		// Year-boundary: Jan with 3 months back → Nov of previous year.
		{time.Date(2026, 1, 15, 0, 0, 0, 0, loc), 3, time.Date(2025, 11, 1, 0, 0, 0, 0, loc)},
		// Last day of month is irrelevant — we always anchor to first-of-month.
		{time.Date(2026, 3, 31, 23, 59, 59, 0, loc), 1, time.Date(2026, 3, 1, 0, 0, 0, 0, loc)},
	}

	for _, tc := range cases {
		got := monthsBackCutoff(tc.now, tc.months)
		if !got.Equal(tc.want) {
			t.Errorf("monthsBackCutoff(%s, %d) = %s, want %s",
				tc.now.Format(time.RFC3339), tc.months,
				got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
		}
	}
}
