package bot

import (
	"log"
	"sync"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
)

type DailyStore interface {
	Latest() (*dailyhistory.Record, error)
	Insert(dayNumber, revision int64, ballIDs []int) error
}

type DailyNotifier interface {
	Notify(kind Kind, ballIDs []int)
}

type Detector struct {
	store    DailyStore
	notifier DailyNotifier

	mu       sync.Mutex
	lastSeen *dailyhistory.Record
}

func NewDetector(store DailyStore, notifier DailyNotifier) (*Detector, error) {
	latest, err := store.Latest()
	if err != nil {
		return nil, err
	}
	return &Detector{store: store, notifier: notifier, lastSeen: latest}, nil
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (d *Detector) OnLine(line string) {
	parsed := ball.Parse(line)
	if parsed == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var (
		revision int64
		kind     Kind
	)
	switch {
	case d.lastSeen == nil:
		revision, kind = 0, NewDay
	case d.lastSeen.DayNumber != parsed.DayNumber:
		revision, kind = 0, NewDay
	case equalInts(d.lastSeen.BallIDs, parsed.BallIDs):
		return
	default:
		revision, kind = d.lastSeen.Revision+1, SeedUpdate
	}

	if err := d.store.Insert(parsed.DayNumber, revision, parsed.BallIDs); err != nil {
		log.Printf("Insert daily day=%d rev=%d: %v", parsed.DayNumber, revision, err)
		return
	}
	d.lastSeen = &dailyhistory.Record{
		DayNumber: parsed.DayNumber,
		Revision:  revision,
		BallIDs:   append([]int(nil), parsed.BallIDs...),
	}
	log.Printf("Detected daily (kind=%d, day=%d, rev=%d, balls=%v)",
		kind, parsed.DayNumber, revision, parsed.BallIDs)
	d.notifier.Notify(kind, parsed.BallIDs)
}
