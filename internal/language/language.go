package language

type Lang string

const (
	LangJA Lang = "ja"
	LangEN Lang = "en"
)

func (l Lang) Valid() bool {
	return l == LangJA || l == LangEN
}
