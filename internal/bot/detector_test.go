package bot

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
)

// ---- fakes ----

type insertCall struct {
	dayNumber int64
	revision  int64
	ballIDs   []int
}

type fakeStore struct {
	latest    *dailyhistory.Record
	inserts   []insertCall
	insertErr error
}

func (f *fakeStore) Latest() (*dailyhistory.Record, error) {
	if f.latest == nil {
		return nil, nil
	}
	c := *f.latest
	c.BallIDs = append([]int(nil), f.latest.BallIDs...)
	return &c, nil
}

func (f *fakeStore) Insert(day, rev int64, ids []int) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserts = append(f.inserts, insertCall{day, rev, append([]int(nil), ids...)})
	return nil
}

type notifyCall struct {
	kind      Kind
	dayNumber int64
	ballIDs   []int
}

type fakeNotifier struct {
	calls []notifyCall
}

func (f *fakeNotifier) Notify(kind Kind, daily ball.Daily) {
	f.calls = append(f.calls, notifyCall{kind, daily.DayNumber, append([]int(nil), daily.BallIDs...)})
}

// ---- helpers ----

func mustDetector(t *testing.T, store DailyStore, notifier DailyNotifier) *Detector {
	t.Helper()
	d, err := NewDetector(store, notifier)
	if err != nil {
		t.Fatalf("NewDetector: %v", err)
	}
	return d
}

func line(day int64, balls ...int) string {
	parts := ""
	for i, b := range balls {
		if i > 0 {
			parts += ", "
		}
		parts += fmt.Sprintf("%d", b)
	}
	return fmt.Sprintf("2026.05.09 12:34:56 Log        -  [DailySpecialBallManager] Day %d: Special balls are %s", day, parts)
}

func assertInserts(t *testing.T, got []insertCall, want []insertCall) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inserts mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func assertNotifies(t *testing.T, got []notifyCall, want []notifyCall) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("notifies mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// ---- tests ----

// 初回起動 (DB空) で最初のログを受けたら NewDay 通知 + insert(rev=0)。
func TestDetector_FirstLine_NotifiesNewDay(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))

	assertInserts(t, store.inserts, []insertCall{{100, 0, []int{5, 15, 6}}})
	assertNotifies(t, notifier.calls, []notifyCall{{NewDay, 100, []int{5, 15, 6}}})
}

// 同じ Day, 同じ balls の Rejoin は通知しない。
func TestDetector_RejoinSameDayBalls_Silent(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))
	notifier.calls = nil
	store.inserts = nil

	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(100, 5, 15, 6))

	if len(store.inserts) != 0 {
		t.Fatalf("expected no inserts, got %v", store.inserts)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifies, got %v", notifier.calls)
	}
}

// 同じ Day で balls が変わったら SeedUpdate 通知 + insert(rev+1)。
func TestDetector_SeedUpdateMidDay(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(100, 1, 2, 3))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6}},
		{100, 1, []int{1, 2, 3}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6}},
		{SeedUpdate, 100, []int{1, 2, 3}},
	})
}

// 同じ Day で 2 度目のシード更新は revision が連番で増える。
func TestDetector_MultipleSeedUpdatesIncrementRevision(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(100, 1, 2, 3))
	d.OnLine(line(100, 7, 8, 9))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6}},
		{100, 1, []int{1, 2, 3}},
		{100, 2, []int{7, 8, 9}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6}},
		{SeedUpdate, 100, []int{1, 2, 3}},
		{SeedUpdate, 100, []int{7, 8, 9}},
	})
}

// 新 Day は balls が違えば NewDay (rev=0) 通知。
func TestDetector_NewDayDifferentBalls(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(101, 1, 2, 3))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6}},
		{101, 0, []int{1, 2, 3}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6}},
		{NewDay, 101, []int{1, 2, 3}},
	})
}

// 9 時跨ぎで偶然 balls が同じでも、Day が違えば NewDay 通知 (rev=0)。
// これが時刻ベース判定を撤廃した理由の核となるケース。
func TestDetector_NewDayCoincidentallySameBalls(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(101, 5, 15, 6))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6}},
		{101, 0, []int{5, 15, 6}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6}},
		{NewDay, 101, []int{5, 15, 6}},
	})
}

// パターンに合致しない行は無視される (副作用なし)。
func TestDetector_NonMatchingLines_Ignored(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine("nothing relevant here")
	d.OnLine("[OtherManager] Day 100: Special balls are 5, 15, 6")
	d.OnLine("")
	d.OnLine("[DailySpecialBallManager] No day, no balls")

	if len(store.inserts) != 0 || len(notifier.calls) != 0 {
		t.Fatalf("expected no side effects, got inserts=%v notifies=%v",
			store.inserts, notifier.calls)
	}
}

// 再起動時は LatestDaily で前回状態が復元され、同 Day 同 balls の Rejoin は無通知。
func TestDetector_RestartRestoresState_SilentOnRejoin(t *testing.T) {
	store := &fakeStore{
		latest: &dailyhistory.Record{
			DayNumber: 101,
			Revision:  2,
			BallIDs:   []int{1, 2, 3},
		},
	}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(101, 1, 2, 3))

	if len(store.inserts) != 0 {
		t.Fatalf("expected no inserts on rejoin, got %v", store.inserts)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifies on rejoin, got %v", notifier.calls)
	}
}

// 再起動後に同 Day で balls が変わっていれば revision を引き継いで SeedUpdate。
func TestDetector_RestartThenSeedUpdate_IncrementsRevisionFromStore(t *testing.T) {
	store := &fakeStore{
		latest: &dailyhistory.Record{
			DayNumber: 101,
			Revision:  2,
			BallIDs:   []int{1, 2, 3},
		},
	}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(101, 7, 8, 9))

	assertInserts(t, store.inserts, []insertCall{{101, 3, []int{7, 8, 9}}})
	assertNotifies(t, notifier.calls, []notifyCall{{SeedUpdate, 101, []int{7, 8, 9}}})
}

// 再起動後に新しい Day が来たら NewDay (rev=0)。
func TestDetector_RestartThenNewDay_ResetsRevision(t *testing.T) {
	store := &fakeStore{
		latest: &dailyhistory.Record{
			DayNumber: 101,
			Revision:  5,
			BallIDs:   []int{1, 2, 3},
		},
	}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(102, 1, 2, 3))

	assertInserts(t, store.inserts, []insertCall{{102, 0, []int{1, 2, 3}}})
	assertNotifies(t, notifier.calls, []notifyCall{{NewDay, 102, []int{1, 2, 3}}})
}

// 4 個玉、5 個玉などボール数が変わっても検知できる (将来増加想定)。
func TestDetector_VariableBallCount(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6, 7))
	d.OnLine(line(101, 1, 2, 3, 4, 8))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6, 7}},
		{101, 0, []int{1, 2, 3, 4, 8}},
	})
}

// InsertDaily が失敗したら通知も止める (二重通知防止) し、内部状態 lastSeen も更新しない。
func TestDetector_InsertError_NoNotify_NoStateAdvance(t *testing.T) {
	store := &fakeStore{insertErr: errors.New("db down")}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))

	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notify on insert error, got %v", notifier.calls)
	}

	// 直後に DB が回復したと仮定して同じ行を再投入 → lastSeen が前進していないので NewDay として正しく検知される。
	store.insertErr = nil
	d.OnLine(line(100, 5, 15, 6))

	assertInserts(t, store.inserts, []insertCall{{100, 0, []int{5, 15, 6}}})
	assertNotifies(t, notifier.calls, []notifyCall{{NewDay, 100, []int{5, 15, 6}}})
}

// LatestDaily がエラーを返したら NewDetector も失敗する。
func TestNewDetector_StoreErrorPropagates(t *testing.T) {
	store := &errStore{err: errors.New("read failure")}
	if _, err := NewDetector(store, &fakeNotifier{}); err == nil {
		t.Fatal("expected error from NewDetector when LatestDaily fails")
	}
}

type errStore struct{ err error }

func (s *errStore) Latest() (*dailyhistory.Record, error) { return nil, s.err }
func (s *errStore) Insert(int64, int64, []int) error      { return nil }

// ボール順序が異なれば別 balls とみなして SeedUpdate (位置情報は意味を持つ)。
func TestDetector_OrderMatters(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(100, 6, 15, 5))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6}},
		{100, 1, []int{6, 15, 5}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6}},
		{SeedUpdate, 100, []int{6, 15, 5}},
	})
}

// 実シナリオ: 起動 → Rejoin数回 → シード更新 → Rejoin → 翌日。
func TestDetector_FullScenario(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	// 起動直後の最初のログ
	d.OnLine(line(100, 5, 15, 6))
	// Rejoin x 2
	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(100, 5, 15, 6))
	// ワールドアプデでシード更新
	d.OnLine(line(100, 1, 2, 3))
	// 更新後に Rejoin
	d.OnLine(line(100, 1, 2, 3))
	// 翌日 (偶然 balls 同じ)
	d.OnLine(line(101, 1, 2, 3))
	// 翌日 Rejoin
	d.OnLine(line(101, 1, 2, 3))

	assertInserts(t, store.inserts, []insertCall{
		{100, 0, []int{5, 15, 6}},
		{100, 1, []int{1, 2, 3}},
		{101, 0, []int{1, 2, 3}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6}},
		{SeedUpdate, 100, []int{1, 2, 3}},
		{NewDay, 101, []int{1, 2, 3}},
	})
}
