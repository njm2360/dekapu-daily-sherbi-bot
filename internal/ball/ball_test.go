package ball

import (
	"reflect"
	"testing"
)

func TestParse_Valid(t *testing.T) {
	line := "2026.05.09 12:34:56 Log        -  [DailySpecialBallManager] Day 100: Special balls are 5, 15, 6"
	got := Parse(line)
	if got == nil {
		t.Fatal("Parse returned nil")
	}
	if got.DayNumber != 100 {
		t.Errorf("DayNumber = %d, want 100", got.DayNumber)
	}
	if !reflect.DeepEqual(got.BallIDs, []int{5, 15, 6}) {
		t.Errorf("BallIDs = %v, want [5 15 6]", got.BallIDs)
	}
}

func TestParse_NonMatching(t *testing.T) {
	cases := []string{
		"",
		"nothing relevant",
		"[OtherManager] Day 100: Special balls are 5, 15, 6",
		"[DailySpecialBallManager] No day, no balls",
	}
	for _, c := range cases {
		if got := Parse(c); got != nil {
			t.Errorf("Parse(%q) = %+v, want nil", c, got)
		}
	}
}

func TestParse_IncludesUndefinedIDs(t *testing.T) {
	line := "[DailySpecialBallManager] Day 200: Special balls are 5, 99, 6"
	got := Parse(line)
	if got == nil {
		t.Fatal("Parse returned nil")
	}
	if !reflect.DeepEqual(got.BallIDs, []int{5, 99, 6}) {
		t.Errorf("BallIDs = %v, want [5 99 6]", got.BallIDs)
	}
}

func TestDaily_Date(t *testing.T) {
	d := Daily{DayNumber: 0}
	got := d.Date().Format("2006-01-02")
	if got != "1970-01-01" {
		t.Errorf("Date for day 0 = %s, want 1970-01-01", got)
	}
	d = Daily{DayNumber: 365}
	got = d.Date().Format("2006-01-02")
	if got != "1971-01-01" {
		t.Errorf("Date for day 365 = %s, want 1971-01-01", got)
	}
}
