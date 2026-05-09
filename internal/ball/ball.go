package ball

import (
	"regexp"
	"strconv"
	"strings"
)

type Lang string

const (
	LangJA Lang = "ja"
	LangEN Lang = "en"
)

func (l Lang) Valid() bool {
	return l == LangJA || l == LangEN
}

type Info struct {
	Name        string
	Description string
}

var specialBallPattern = regexp.MustCompile(`\[DailySpecialBallManager\].*Day (\d+).*Special balls are ([\d,\s]+)`)

type Daily struct {
	DayNumber int64
	BallIDs   []int
}

var names = map[Lang]map[int]Info{
	LangJA: {
		1:  {"橙", "連チャンした後に別のボールに変わるよ"},
		2:  {"緑", "ルーレットフィーバーを回すよ"},
		4:  {"ピンク", "ボールをさらに増やすよ"},
		5:  {"紫", "プッシャー台を平らにしてボールを増やすよ"},
		6:  {"白", "多めにメダルを出すよ"},
		7:  {"水色", "シャルべチャンスの抽選ボールを出すよ"},
		8:  {"灰桜", "シャルべチャンスで1マス進むよ"},
		9:  {"群青", "シャルベJPプログレッシブカウンターを増やすよ"},
		10: {"赤", "連チャンした後に別のボールに変わるよ"},
		11: {"クリーム", "シャルべチャンスの倍率を上げるよ"},
		12: {"ミント", "強い抽選ボールを出すことがあるよ"},
		13: {"ピーチ", "SPメダルを出すよ"},
		14: {"星空", "時々同じボールを2つ出すよ"},
		15: {"草原", "パレッタチャンスの倍率を上げるよ"},
		16: {"黒", "たまにレインボーシャルべを出すよ"},
		17: {"黄", "パレッタチャンスを早く回すよ"},
		18: {"ラベンダー", "パレッタJPプログレッシブカウンターを増やすよ"},
		19: {"ライム", "メダル投入量を一時的に増やすよ"},
	},
	LangEN: {
		1:  {"Orange", "Chains itself then changes to another color"},
		2:  {"Green", "Spins fever roulettes"},
		4:  {"Pink", "Spawns more balls"},
		5:  {"Purple", "Flattens pusher and spawns balls"},
		6:  {"White", "More payouts"},
		7:  {"Cyan", "Spawns lottery balls for Sherbi Chance"},
		8:  {"Pale Pink", "Advance 1 step at Sherbi Chance"},
		9:  {"Navy", "Raises Sherbi JACKPOT prog. counter"},
		10: {"Red", "Chains itself then changes to another color"},
		11: {"Cream", "Increases multiplier for Sherbi Chance"},
		12: {"Mint", "Has a chance to spawn strong lottery balls"},
		13: {"Peach", "Spawns a SP medal"},
		14: {"Starry", "May spawn two duplicated balls"},
		15: {"Meadow", "Increases multiplier for Paletta Chance"},
		16: {"Black", "Has small chance to spawn Rainbow Sherbi"},
		17: {"Yellow", "Speeds up Paletta Chance game"},
		18: {"Lavender", "Increases Paletta JACKPOT prog. counter"},
		19: {"Lime", "Temporarily increases medal insertion amount"},
	},
}

func Parse(line string) *Daily {
	m := specialBallPattern.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	day, err := strconv.ParseInt(m[1], 10, 64)
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

func Format(ids []int, lang Lang) []Info {
	dict, ok := names[lang]
	if !ok {
		dict = names[LangJA]
	}
	out := make([]Info, 0, len(ids))
	for _, id := range ids {
		if info, ok := dict[id]; ok {
			out = append(out, info)
		} else {
			out = append(out, Info{Name: strconv.Itoa(id), Description: ""})
		}
	}
	return out
}
