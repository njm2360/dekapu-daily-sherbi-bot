package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/detector"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/language"
)

// JA形式は「月/日」になる
func TestDateHeader_JA(t *testing.T) {
	got := dateHeader(time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), language.LangJA)
	if got != "5/9" {
		t.Errorf("dateHeader JA = %q, want %q", got, "5/9")
	}
}

// EN形式は「Month Day」になる
func TestDateHeader_EN(t *testing.T) {
	got := dateHeader(time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), language.LangEN)
	if got != "May 9" {
		t.Errorf("dateHeader EN = %q, want %q", got, "May 9")
	}
}

// EN形式で12ヶ月すべて正しい英語月名になる
func TestDateHeader_EN_AllMonths(t *testing.T) {
	wants := []string{
		"January 1", "February 1", "March 1", "April 1", "May 1", "June 1",
		"July 1", "August 1", "September 1", "October 1", "November 1", "December 1",
	}
	for i, w := range wants {
		got := dateHeader(time.Date(2026, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC), language.LangEN)
		if got != w {
			t.Errorf("month %d: got %q, want %q", i+1, got, w)
		}
	}
}

// NewDayメッセージのJA全文が日付ヘッダ+ボール名+説明の順で組み立てられる
func TestBuildMessage_NewDay_JA(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(detector.NewDay, []int{1, 2}, now, language.LangJA)
	want := strings.Join([]string{
		"# 5/9",
		"## 橙",
		"連チャンした後に別のボールに変わるよ",
		"## 緑",
		"ルーレットフィーバーを回すよ",
	}, "\n")
	if got != want {
		t.Errorf("buildMessage NewDay JA =\n%q\nwant\n%q", got, want)
	}
}

// NewDayメッセージのEN全文が日付ヘッダ+ボール名+説明の順で組み立てられる
func TestBuildMessage_NewDay_EN(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(detector.NewDay, []int{1, 2}, now, language.LangEN)
	want := strings.Join([]string{
		"# May 9",
		"## Orange",
		"Chains itself then changes to another color",
		"## Green",
		"Spins fever roulettes",
	}, "\n")
	if got != want {
		t.Errorf("buildMessage NewDay EN =\n%q\nwant\n%q", got, want)
	}
}

// SeedUpdateはJAヘッダにシード更新の注釈が付く
func TestBuildMessage_SeedUpdate_JA(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(detector.SeedUpdate, []int{1}, now, language.LangJA)
	if !strings.HasPrefix(got, "# 5/9 (シード更新でデイリーが変わったよ)") {
		t.Errorf("buildMessage SeedUpdate JA missing seed-update header:\n%s", got)
	}
	if !strings.Contains(got, "## 橙") {
		t.Errorf("buildMessage SeedUpdate JA missing ball name:\n%s", got)
	}
}

// SeedUpdateはENヘッダにシード更新の注釈が付く
func TestBuildMessage_SeedUpdate_EN(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(detector.SeedUpdate, []int{1}, now, language.LangEN)
	if !strings.HasPrefix(got, "# May 9 (Daily updated by seed change)") {
		t.Errorf("buildMessage SeedUpdate EN missing seed-update header:\n%s", got)
	}
	if !strings.Contains(got, "## Orange") {
		t.Errorf("buildMessage SeedUpdate EN missing ball name:\n%s", got)
	}
}

// 未定義IDはJAフォールバック文言で出力される
func TestBuildMessage_UndefinedID_JA(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(detector.NewDay, []int{1, 99}, now, language.LangJA)
	want := strings.Join([]string{
		"# 5/9",
		"## 橙",
		"連チャンした後に別のボールに変わるよ",
		"## 未定義 (ID:99)",
		"ワールド内で確認してね",
	}, "\n")
	if got != want {
		t.Errorf("buildMessage with undefined ID (JA) =\n%q\nwant\n%q", got, want)
	}
}

// 未定義IDはENフォールバック文言で出力される
func TestBuildMessage_UndefinedID_EN(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(detector.NewDay, []int{99}, now, language.LangEN)
	want := strings.Join([]string{
		"# May 9",
		"## Unknown (ID:99)",
		"Please check in the world",
	}, "\n")
	if got != want {
		t.Errorf("buildMessage with undefined ID (EN) =\n%q\nwant\n%q", got, want)
	}
}
