package bot

import (
	"strings"
	"testing"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/language"
)

// 該当なしのときは検索条件と見つからなかった旨だけを返す
func TestFormatFindDailyResult_NoMatch(t *testing.T) {
	got := formatFindDailyResult([]int{1, 2}, nil, language.LangJA)
	want := "検索条件: 橙 / 緑\n該当する日が見つからなかったよ。"
	if got != want {
		t.Errorf("formatFindDailyResult (no match, JA) =\n%q\nwant\n%q", got, want)
	}

	got = formatFindDailyResult([]int{1, 2}, nil, language.LangEN)
	want = "Query: Orange / Green\nNo matching days found."
	if got != want {
		t.Errorf("formatFindDailyResult (no match, EN) =\n%q\nwant\n%q", got, want)
	}
}

// 1日1リビジョンは日付+ボール名の1行になる
func TestFormatFindDailyResult_SingleRevision(t *testing.T) {
	matches := []dailyhistory.Match{
		{DayNumber: 100, Revision: 0, BallIDs: []int{1, 2}},
	}
	got := formatFindDailyResult([]int{1}, matches, language.LangJA)
	want := strings.Join([]string{
		"検索条件: 橙",
		"- 1970/04/11: 橙・緑",
		"",
	}, "\n")
	if got != want {
		t.Errorf("formatFindDailyResult (single revision) =\n%q\nwant\n%q", got, want)
	}
}

// 同じ日に3リビジョン以上マッチしても矢印で1行に連結される
func TestFormatFindDailyResult_ThreeRevisionsSameDay(t *testing.T) {
	matches := []dailyhistory.Match{
		{DayNumber: 100, Revision: 0, BallIDs: []int{1, 2}},
		{DayNumber: 100, Revision: 1, BallIDs: []int{1, 4}},
		{DayNumber: 100, Revision: 2, BallIDs: []int{1, 6}},
	}
	got := formatFindDailyResult([]int{1}, matches, language.LangJA)
	want := strings.Join([]string{
		"検索条件: 橙",
		"- 1970/04/11: 橙・緑 → 橙・ピンク → 橙・白",
		"",
	}, "\n")
	if got != want {
		t.Errorf("formatFindDailyResult (3 revisions) =\n%q\nwant\n%q", got, want)
	}
}

// リビジョン0がマッチせず途中のリビジョンだけ返っても1行に連結される
func TestFormatFindDailyResult_MatchStartsAtLaterRevision(t *testing.T) {
	matches := []dailyhistory.Match{
		{DayNumber: 100, Revision: 1, BallIDs: []int{1, 4}},
		{DayNumber: 100, Revision: 2, BallIDs: []int{1, 6}},
	}
	got := formatFindDailyResult([]int{1}, matches, language.LangJA)
	want := strings.Join([]string{
		"検索条件: 橙",
		"- 1970/04/11: 橙・ピンク → 橙・白",
		"",
	}, "\n")
	if got != want {
		t.Errorf("formatFindDailyResult (starts at rev1) =\n%q\nwant\n%q", got, want)
	}
}

// 日が変わると改行され、日ごとにリビジョンがまとめられる
func TestFormatFindDailyResult_MultipleDays(t *testing.T) {
	matches := []dailyhistory.Match{
		{DayNumber: 101, Revision: 0, BallIDs: []int{1, 2}},
		{DayNumber: 101, Revision: 1, BallIDs: []int{1, 4}},
		{DayNumber: 100, Revision: 0, BallIDs: []int{1, 6}},
	}
	got := formatFindDailyResult([]int{1}, matches, language.LangJA)
	want := strings.Join([]string{
		"検索条件: 橙",
		"- 1970/04/12: 橙・緑 → 橙・ピンク",
		"- 1970/04/11: 橙・白",
		"",
	}, "\n")
	if got != want {
		t.Errorf("formatFindDailyResult (multiple days) =\n%q\nwant\n%q", got, want)
	}
}

// ENはボール名がスラッシュ区切りで出力される
func TestFormatFindDailyResult_EN(t *testing.T) {
	matches := []dailyhistory.Match{
		{DayNumber: 100, Revision: 0, BallIDs: []int{1, 2}},
		{DayNumber: 100, Revision: 1, BallIDs: []int{1, 4}},
	}
	got := formatFindDailyResult([]int{1}, matches, language.LangEN)
	want := strings.Join([]string{
		"Query: Orange",
		"- 1970/04/11: Orange / Green → Orange / Pink",
		"",
	}, "\n")
	if got != want {
		t.Errorf("formatFindDailyResult (EN) =\n%q\nwant\n%q", got, want)
	}
}
