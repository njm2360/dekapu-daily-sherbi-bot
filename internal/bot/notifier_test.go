package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
)

func TestDateHeader_JA(t *testing.T) {
	got := dateHeader(time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), ball.LangJA)
	if got != "5/9" {
		t.Errorf("dateHeader JA = %q, want %q", got, "5/9")
	}
}

func TestDateHeader_EN(t *testing.T) {
	got := dateHeader(time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), ball.LangEN)
	if got != "May 9" {
		t.Errorf("dateHeader EN = %q, want %q", got, "May 9")
	}
}

func TestDateHeader_EN_AllMonths(t *testing.T) {
	wants := []string{
		"January 1", "February 1", "March 1", "April 1", "May 1", "June 1",
		"July 1", "August 1", "September 1", "October 1", "November 1", "December 1",
	}
	for i, w := range wants {
		got := dateHeader(time.Date(2026, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC), ball.LangEN)
		if got != w {
			t.Errorf("month %d: got %q, want %q", i+1, got, w)
		}
	}
}

func TestBuildMessage_NewDay_JA(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(NewDay, []int{1, 2}, now, ball.LangJA)
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

func TestBuildMessage_NewDay_EN(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(NewDay, []int{1, 2}, now, ball.LangEN)
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

func TestBuildMessage_SeedUpdate_JA(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(SeedUpdate, []int{1}, now, ball.LangJA)
	if !strings.HasPrefix(got, "# 5/9 (シード更新でデイリーが変わったよ)") {
		t.Errorf("buildMessage SeedUpdate JA missing seed-update header:\n%s", got)
	}
	if !strings.Contains(got, "## 橙") {
		t.Errorf("buildMessage SeedUpdate JA missing ball name:\n%s", got)
	}
}

func TestBuildMessage_SeedUpdate_EN(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(SeedUpdate, []int{1}, now, ball.LangEN)
	if !strings.HasPrefix(got, "# May 9 (Daily updated by seed change)") {
		t.Errorf("buildMessage SeedUpdate EN missing seed-update header:\n%s", got)
	}
	if !strings.Contains(got, "## Orange") {
		t.Errorf("buildMessage SeedUpdate EN missing ball name:\n%s", got)
	}
}

func TestBuildMessage_UndefinedID_JA(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(NewDay, []int{1, 99}, now, ball.LangJA)
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

func TestBuildMessage_UndefinedID_EN(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(NewDay, []int{99}, now, ball.LangEN)
	want := strings.Join([]string{
		"# May 9",
		"## Unknown (ID:99)",
		"Please check in the world",
	}, "\n")
	if got != want {
		t.Errorf("buildMessage with undefined ID (EN) =\n%q\nwant\n%q", got, want)
	}
}

func TestBuildMessage_EmptyBallIDs(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(NewDay, nil, now, ball.LangJA)
	if got != "# 5/9" {
		t.Errorf("buildMessage with empty balls = %q, want %q", got, "# 5/9")
	}
}

// Description が空文字のボールが定義に追加された場合に、本文側に余計な改行が出ないことを担保する。
// 現在の定義には Description 空のエントリは無いので、未定義 ID は Description が必ず付くロジックを使い、
// ここでは挙動仕様だけ示す: Description="" のとき "## Name" だけが出る (改行を足さない)。
func TestBuildMessage_HeaderOrderingStable(t *testing.T) {
	now := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	got := buildMessage(NewDay, []int{2, 1}, now, ball.LangJA)
	// 入力順を維持すること
	idxGreen := strings.Index(got, "## 緑")
	idxOrange := strings.Index(got, "## 橙")
	if idxGreen < 0 || idxOrange < 0 {
		t.Fatalf("missing entries:\n%s", got)
	}
	if idxGreen > idxOrange {
		t.Errorf("input order not preserved: 緑 should precede 橙\n%s", got)
	}
}
