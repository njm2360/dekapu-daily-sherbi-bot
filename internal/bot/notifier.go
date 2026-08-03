package bot

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/balltext"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/detector"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/language"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/settings"
)

var monthsEN = [...]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

func dateHeader(now time.Time, lang language.Lang) string {
	if lang == language.LangEN {
		return fmt.Sprintf("%s %d", monthsEN[int(now.Month())-1], now.Day())
	}
	return fmt.Sprintf("%d/%d", int(now.Month()), now.Day())
}

func buildMessage(kind detector.Kind, ballIDs []int, now time.Time, lang language.Lang) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(dateHeader(now, lang))
	if kind == detector.SeedUpdate {
		if lang == language.LangEN {
			sb.WriteString(" (Daily updated by seed change)")
		} else {
			sb.WriteString(" (シード更新でデイリーが変わったよ)")
		}
	}

	for _, info := range balltext.Format(ballIDs, lang) {
		sb.WriteString("\n## ")
		sb.WriteString(info.Name)
		if info.Description != "" {
			sb.WriteString("\n")
			sb.WriteString(info.Description)
		}
	}
	return sb.String()
}

type Notifier struct {
	session *discordgo.Session
	repo    *settings.Repository
}

func NewNotifier(s *discordgo.Session, r *settings.Repository) *Notifier {
	return &Notifier{session: s, repo: r}
}

func (n *Notifier) Notify(kind detector.Kind, daily ball.Daily) {
	date := daily.Date()

	channels, err := n.repo.AllChannels()
	if err != nil {
		log.Printf("Notify: AllChannels: %v", err)
		return
	}

	var wg sync.WaitGroup
	for _, cfg := range channels {
		wg.Add(1)
		go func(cfg settings.ChannelConfig) {
			defer wg.Done()

			message := buildMessage(kind, daily.BallIDs, date, cfg.Lang)
			channelID := cfg.ChannelID

			roleID, hasRole, err := n.repo.GetMentionRole(cfg.GuildID)
			if err != nil {
				log.Printf("Notify: GetMentionRole guild=%s: %v", cfg.GuildID, err)
			}
			content := message
			if hasRole {
				content = fmt.Sprintf("<@&%s>\n%s", roleID, message)
			}

			if _, err := n.session.ChannelMessageSend(channelID, content); err != nil {
				var rest *discordgo.RESTError
				if errors.As(err, &rest) && rest.Response != nil {
					switch rest.Response.StatusCode {
					case 404:
						log.Printf("Channel %s not found (guild=%s)", channelID, cfg.GuildID)
						return
					case 403:
						log.Printf("No permission to access channel %s (guild=%s)", channelID, cfg.GuildID)
						return
					}
				}
				log.Printf("ChannelMessageSend failed (channel=%s, guild=%s): %v",
					channelID, cfg.GuildID, err)
				return
			}
			log.Printf("Sent special balls %v (day=%d) to channel %s (guild=%s, lang=%s)",
				daily.BallIDs, daily.DayNumber, channelID, cfg.GuildID, cfg.Lang)
		}(cfg)
	}
	wg.Wait()
}
