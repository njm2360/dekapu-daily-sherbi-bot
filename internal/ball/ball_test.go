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

func TestName_Defined(t *testing.T) {
	if got := Name(1, LangJA); got != "橙" {
		t.Errorf("Name(1, ja) = %q, want 橙", got)
	}
	if got := Name(1, LangEN); got != "Orange" {
		t.Errorf("Name(1, en) = %q, want Orange", got)
	}
}

func TestName_Undefined_JA(t *testing.T) {
	got := Name(99, LangJA)
	want := "未定義 (ID:99)"
	if got != want {
		t.Errorf("Name(99, ja) = %q, want %q", got, want)
	}
}

func TestName_Undefined_EN(t *testing.T) {
	got := Name(99, LangEN)
	want := "Unknown (ID:99)"
	if got != want {
		t.Errorf("Name(99, en) = %q, want %q", got, want)
	}
}

func TestName_InvalidLangFallsBackToJA(t *testing.T) {
	if got := Name(1, Lang("xx")); got != "橙" {
		t.Errorf("Name(1, xx) = %q, want 橙 (JA fallback)", got)
	}
	if got := Name(99, Lang("xx")); got != "未定義 (ID:99)" {
		t.Errorf("Name(99, xx) = %q, want 未定義 (ID:99)", got)
	}
}

func TestFormat_AllDefined(t *testing.T) {
	got := Format([]int{1, 2}, LangJA)
	want := []Info{
		{Name: "橙", Description: "連チャンした後に別のボールに変わるよ"},
		{Name: "緑", Description: "ルーレットフィーバーを回すよ"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestFormat_MixedUndefined_JA(t *testing.T) {
	got := Format([]int{1, 99, 2}, LangJA)
	want := []Info{
		{Name: "橙", Description: "連チャンした後に別のボールに変わるよ"},
		{Name: "未定義 (ID:99)", Description: "ワールド内で確認してね"},
		{Name: "緑", Description: "ルーレットフィーバーを回すよ"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestFormat_MixedUndefined_EN(t *testing.T) {
	got := Format([]int{1, 99}, LangEN)
	want := []Info{
		{Name: "Orange", Description: "Chains itself then changes to another color"},
		{Name: "Unknown (ID:99)", Description: "Please check in the world"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestFormat_InvalidLangFallsBackToJA(t *testing.T) {
	got := Format([]int{99}, Lang("xx"))
	want := []Info{{Name: "未定義 (ID:99)", Description: "ワールド内で確認してね"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestAllIDs_SortedAndContainsOnlyDefined(t *testing.T) {
	ids := AllIDs()
	want := []int{1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("AllIDs = %v, want %v", ids, want)
	}
}

func TestLang_Valid(t *testing.T) {
	if !LangJA.Valid() || !LangEN.Valid() {
		t.Error("LangJA/LangEN must be valid")
	}
	if Lang("xx").Valid() {
		t.Error("invalid lang should not be Valid()")
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
