package balltext

import (
	"reflect"
	"testing"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/language"
)

func TestName_Defined(t *testing.T) {
	if got := Name(1, language.LangJA); got != "橙" {
		t.Errorf("Name(1, ja) = %q, want 橙", got)
	}
	if got := Name(1, language.LangEN); got != "Orange" {
		t.Errorf("Name(1, en) = %q, want Orange", got)
	}
}

func TestName_Undefined_JA(t *testing.T) {
	got := Name(99, language.LangJA)
	want := "未定義 (ID:99)"
	if got != want {
		t.Errorf("Name(99, ja) = %q, want %q", got, want)
	}
}

func TestName_Undefined_EN(t *testing.T) {
	got := Name(99, language.LangEN)
	want := "Unknown (ID:99)"
	if got != want {
		t.Errorf("Name(99, en) = %q, want %q", got, want)
	}
}

func TestName_InvalidLangFallsBackToJA(t *testing.T) {
	if got := Name(1, language.Lang("xx")); got != "橙" {
		t.Errorf("Name(1, xx) = %q, want 橙 (JA fallback)", got)
	}
	if got := Name(99, language.Lang("xx")); got != "未定義 (ID:99)" {
		t.Errorf("Name(99, xx) = %q, want 未定義 (ID:99)", got)
	}
}

func TestFormat_AllDefined(t *testing.T) {
	got := Format([]int{1, 2}, language.LangJA)
	want := []Info{
		{Name: "橙", Description: "連チャンした後に別のボールに変わるよ"},
		{Name: "緑", Description: "ルーレットフィーバーを回すよ"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestFormat_MixedUndefined_JA(t *testing.T) {
	got := Format([]int{1, 99, 2}, language.LangJA)
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
	got := Format([]int{1, 99}, language.LangEN)
	want := []Info{
		{Name: "Orange", Description: "Chains itself then changes to another color"},
		{Name: "Unknown (ID:99)", Description: "Please check in the world"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestFormat_InvalidLangFallsBackToJA(t *testing.T) {
	got := Format([]int{99}, language.Lang("xx"))
	want := []Info{{Name: "未定義 (ID:99)", Description: "ワールド内で確認してね"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Format = %v, want %v", got, want)
	}
}

func TestAllIDs_SortedAndContainsOnlyDefined(t *testing.T) {
	ids := AllIDs()
	want := []int{1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("AllIDs = %v, want %v", ids, want)
	}
}
