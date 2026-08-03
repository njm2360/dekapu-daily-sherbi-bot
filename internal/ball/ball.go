package ball

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var specialBallPattern = regexp.MustCompile(`\[DailySpecialBallManager\].*Day (\d+).*Special balls are ([\d,\s]+)`)

type Daily struct {
	DayNumber int
	BallIDs   []int
}

func (d Daily) Date() time.Time {
	return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d.DayNumber)
}

func Parse(line string) *Daily {
	m := specialBallPattern.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	day, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	parts := strings.Split(m[2], ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		ids = append(ids, n)
	}
	if len(ids) == 0 {
		return nil
	}
	return &Daily{DayNumber: day, BallIDs: ids}
}
