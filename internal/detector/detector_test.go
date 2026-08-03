package detector

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
)

// ---- フェイク ----

type insertCall struct {
	dayNumber int
	revision  int
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

func (f *fakeStore) Insert(day, rev int, ids []int) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserts = append(f.inserts, insertCall{day, rev, append([]int(nil), ids...)})
	return nil
}

type notifyCall struct {
	kind      Kind
	dayNumber int
	ballIDs   []int
}

type fakeNotifier struct {
	calls []notifyCall
}

func (f *fakeNotifier) Notify(kind Kind, daily ball.Daily) {
	f.calls = append(f.calls, notifyCall{kind, daily.DayNumber, append([]int(nil), daily.BallIDs...)})
}

// ---- ヘルパー ----

func mustDetector(t *testing.T, store DailyStore, notifier DailyNotifier) *Detector {
	t.Helper()
	d, err := New(store, notifier)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

func line(day int, balls ...int) string {
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

// ---- テスト ----

// 初回起動（DB空）で最初のログを受けたらNewDay通知+Insert(rev=0)
func TestDetector_FirstLine_NotifiesNewDay(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))

	assertInserts(t, store.inserts, []insertCall{{100, 0, []int{5, 15, 6}}})
	assertNotifies(t, notifier.calls, []notifyCall{{NewDay, 100, []int{5, 15, 6}}})
}

// 同じDay・同じballsであればDaily行を検知しても通知しない
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

// 同じDayでballsが変わったらSeedUpdate通知+Insert(rev+1)
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

// 同じDayで2度目のシード更新はrevisionが連番で増える
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

// Dayが変わったらNewDay(rev=0)通知
func TestDetector_DayChange_NotifiesNewDay(t *testing.T) {
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

// 新Dayで偶然ballsが同じでもNewDay通知(rev=0)
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

// 過去のDay行(複数ログファイルの読み直しで遅れて届く)は無視する
func TestDetector_StaleDayLine_Ignored(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(101, 1, 2, 3))
	// 古いログファイル側のDay行が後から届く
	d.OnLine(line(100, 5, 15, 6))
	// その後の当日分は正しく処理される(シード更新)
	d.OnLine(line(101, 7, 8, 9))

	assertInserts(t, store.inserts, []insertCall{
		{101, 0, []int{1, 2, 3}},
		{101, 1, []int{7, 8, 9}},
	})
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 101, []int{1, 2, 3}},
		{SeedUpdate, 101, []int{7, 8, 9}},
	})
}

// 再起動後(Latest復元)でも過去のDay行は無視する
func TestDetector_RestartThenStaleDayLine_Ignored(t *testing.T) {
	store := &fakeStore{
		latest: &dailyhistory.Record{
			DayNumber: 101,
			Revision:  0,
			BallIDs:   []int{1, 2, 3},
		},
	}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))

	if len(store.inserts) != 0 {
		t.Fatalf("expected no inserts, got %v", store.inserts)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifies, got %v", notifier.calls)
	}
}

// 再起動時はLatestで前回状態が復元され、同Day・同ballsの場合は通知しない
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
		t.Fatalf("expected no inserts, got %v", store.inserts)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifies, got %v", notifier.calls)
	}
}

// 再起動後に同Dayでballsが違う場合はrevisionを引き継いでSeedUpdate
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

// 再起動後に新Dayが来たらNewDay(rev=0)
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

// デイリーの数が将来的に増えても動作すること
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
	assertNotifies(t, notifier.calls, []notifyCall{
		{NewDay, 100, []int{5, 15, 6, 7}},
		{NewDay, 101, []int{1, 2, 3, 4, 8}},
	})
}

// Insertが失敗したら通知も止め(二重通知防止)、内部状態lastSeenも更新しない
func TestDetector_InsertError_NoNotify_NoStateAdvance(t *testing.T) {
	store := &fakeStore{insertErr: errors.New("db down")}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	d.OnLine(line(100, 5, 15, 6))

	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notify on insert error, got %v", notifier.calls)
	}

	// 直後にDBが回復したと仮定して同じ行を再投入→lastSeenが前進していないのでNewDayとして正しく検知される。
	store.insertErr = nil
	d.OnLine(line(100, 5, 15, 6))

	assertInserts(t, store.inserts, []insertCall{{100, 0, []int{5, 15, 6}}})
	assertNotifies(t, notifier.calls, []notifyCall{{NewDay, 100, []int{5, 15, 6}}})
}

// LatestがエラーならNewも失敗する
func TestNew_StoreErrorPropagates(t *testing.T) {
	store := &errStore{err: errors.New("read failure")}
	if _, err := New(store, &fakeNotifier{}); err == nil {
		t.Fatal("expected error from New when Latest fails")
	}
}

type errStore struct{ err error }

func (s *errStore) Latest() (*dailyhistory.Record, error) { return nil, s.err }
func (s *errStore) Insert(int, int, []int) error          { return nil }

// ボール順序が異なれば別ballsとみなしてSeedUpdate(位置情報は意味を持つ)
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

// 実シナリオ: 起動→Rejoin数回→シード更新→Rejoin→翌日。
func TestDetector_FullScenario(t *testing.T) {
	store := &fakeStore{}
	notifier := &fakeNotifier{}
	d := mustDetector(t, store, notifier)

	// 起動直後の最初のログ
	d.OnLine(line(100, 5, 15, 6))
	// Rejoinを2回
	d.OnLine(line(100, 5, 15, 6))
	d.OnLine(line(100, 5, 15, 6))
	// ワールドアプデでシード更新
	d.OnLine(line(100, 1, 2, 3))
	// 更新後にRejoin
	d.OnLine(line(100, 1, 2, 3))
	// 翌日(偶然ballsが同じ)
	d.OnLine(line(101, 1, 2, 3))
	// 翌日Rejoin
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
