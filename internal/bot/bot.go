package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/balltext"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/language"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/settings"
)

type Bot struct {
	session  *discordgo.Session
	repo     *settings.Repository
	history  *dailyhistory.Repository
	notifier *Notifier

	readyCh       chan struct{}
	readyOnce     sync.Once
	initialGuilds sync.Map // guildID -> struct{}: known via Ready or already-logged join
}

func New(repo *settings.Repository, history *dailyhistory.Repository, token string) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	s.Identify.Intents = discordgo.IntentsGuilds
	b := &Bot{
		session:  s,
		repo:     repo,
		history:  history,
		notifier: NewNotifier(s, repo),
		readyCh:  make(chan struct{}),
	}
	b.registerHandlers()
	return b, nil
}

func (b *Bot) Notifier() *Notifier { return b.notifier }

func (b *Bot) Ready() <-chan struct{} { return b.readyCh }

func (b *Bot) Run(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}
	defer b.session.Close()

	if err := b.registerCommands(); err != nil {
		return fmt.Errorf("register commands: %w", err)
	}

	<-ctx.Done()
	return nil
}

// ---- Discord handlers ----

func (b *Bot) registerHandlers() {
	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		// Register guilds known at the moment of Ready so subsequent GuildCreate
		// events for them are recognized as initial sync (not new joins).
		for _, g := range r.Guilds {
			b.initialGuilds.Store(g.ID, struct{}{})
		}
		b.readyOnce.Do(func() { close(b.readyCh) })
		log.Printf("Discord bot ready: %s (id=%s)", r.User.Username, r.User.ID)
	})
	b.session.AddHandler(func(s *discordgo.Session, g *discordgo.GuildCreate) {
		<-b.readyCh // ensure initialGuilds is populated by the Ready handler
		if err := b.repo.EnsureGuild(g.ID); err != nil {
			log.Printf("EnsureGuild %s: %v", g.ID, err)
			return
		}
		if _, known := b.initialGuilds.LoadOrStore(g.ID, struct{}{}); !known {
			log.Printf("Joined guild: %s (id=%s)", g.Name, g.ID)
		}
	})
	b.session.AddHandler(func(s *discordgo.Session, g *discordgo.GuildDelete) {
		if g.Unavailable {
			return
		}
		if err := b.repo.RemoveGuild(g.ID); err != nil {
			log.Printf("RemoveGuild %s: %v", g.ID, err)
			return
		}
		b.initialGuilds.Delete(g.ID)
		log.Printf("Left guild: id=%s", g.ID)
	})
	b.session.AddHandler(b.handleInteraction)
}

// ---- Slash commands ----

var slashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "setchannel",
		Description: "Set this channel to receive notifications",
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.Japanese: "このチャンネルに通知を送るよう設定します",
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "lang",
				Description: "Notification language (default: ja)",
				DescriptionLocalizations: map[discordgo.Locale]string{
					discordgo.Japanese: "通知言語（デフォルトは ja）",
				},
				Required: false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "ja", Value: "ja"},
					{Name: "en", Value: "en"},
				},
			},
		},
	},
	{
		Name:        "unsetchannel",
		Description: "Remove notification settings for this channel",
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.Japanese: "このチャンネルの通知設定を解除します",
		},
	},
	{
		Name:        "setmentionrole",
		Description: "Set a role to mention on notification",
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.Japanese: "通知時にメンションするロールを設定します",
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "Role to mention",
				DescriptionLocalizations: map[discordgo.Locale]string{
					discordgo.Japanese: "メンションするロール",
				},
				Required: true,
			},
		},
	},
	{
		Name:        "unsetmentionrole",
		Description: "Remove mention role settings",
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.Japanese: "通知時のメンションロール設定を解除します",
		},
	},
	{
		Name:        "finddaily",
		Description: "Search for dailies containing the specified colors (max 20)",
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.Japanese: "指定した色を含むデイリーを検索します(最大20件)",
		},
		Options: []*discordgo.ApplicationCommandOption{
			ballColorOption("color1", "Color 1", "色1", true),
			ballColorOption("color2", "Color 2 (optional)", "色2 (オプション)", false),
			ballColorOption("color3", "Color 3 (optional)", "色3 (オプション)", false),
			ballColorOption("color4", "Color 4 (optional)", "色4 (オプション)", false),
		},
	},
}

func ballColorOption(name, descEN, descJA string, required bool) *discordgo.ApplicationCommandOption {
	ids := balltext.AllIDs()
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(ids))
	for _, id := range ids {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name: balltext.Name(id, language.LangEN),
			NameLocalizations: map[discordgo.Locale]string{
				discordgo.Japanese: balltext.Name(id, language.LangJA),
			},
			Value: id,
		})
	}
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionInteger,
		Name:        name,
		Description: descEN,
		DescriptionLocalizations: map[discordgo.Locale]string{
			discordgo.Japanese: descJA,
		},
		Required: required,
		Choices:  choices,
	}
}

func (b *Bot) registerCommands() error {
	_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, "", slashCommands)
	return err
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	switch data.Name {
	case "setchannel":
		b.cmdSetChannel(s, i, data)
	case "unsetchannel":
		b.cmdUnsetChannel(s, i)
	case "setmentionrole":
		b.cmdSetMentionRole(s, i, data)
	case "unsetmentionrole":
		b.cmdUnsetMentionRole(s, i)
	case "finddaily":
		b.cmdFindDaily(s, i, data)
	}
}

func replyEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func channelMention(channelID string) string {
	return "<#" + channelID + ">"
}

func (b *Bot) cmdSetChannel(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	l := language.LangJA
	for _, opt := range data.Options {
		if opt.Name == "lang" {
			v := opt.StringValue()
			if language.Lang(v).Valid() {
				l = language.Lang(v)
			}
		}
	}

	guildID := i.GuildID
	channelID := i.ChannelID
	chID, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		replyEphemeral(s, i, "チャンネルIDの解析に失敗したよ。")
		return
	}

	if err := b.repo.EnsureGuild(guildID); err != nil {
		log.Printf("EnsureGuild: %v", err)
	}
	if err := b.repo.SetChannel(guildID, chID, l); err != nil {
		replyEphemeral(s, i, "設定の保存に失敗したよ。")
		log.Printf("SetChannel: %v", err)
		return
	}

	label := "日本語"
	if l == language.LangEN {
		label = "English"
	}
	replyEphemeral(s, i, fmt.Sprintf("%s に通知を送るよ！（言語: %s）", channelMention(channelID), label))
	log.Printf("Set channel %s (lang=%s) for guild %s", channelID, l, guildID)
}

func (b *Bot) cmdUnsetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	chID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		replyEphemeral(s, i, "チャンネルIDの解析に失敗したよ。")
		return
	}
	ok, err := b.repo.UnsetChannel(i.GuildID, chID)
	if err != nil {
		replyEphemeral(s, i, "設定の解除に失敗したよ。")
		log.Printf("UnsetChannel: %v", err)
		return
	}
	mention := channelMention(i.ChannelID)
	if ok {
		replyEphemeral(s, i, fmt.Sprintf("%s の通知チャンネル設定を解除したよ！", mention))
	} else {
		replyEphemeral(s, i, fmt.Sprintf("%s は通知チャンネルに設定されていないよ。", mention))
	}
}

func (b *Bot) cmdSetMentionRole(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	var roleID string
	for _, opt := range data.Options {
		if opt.Name == "role" {
			roleID = opt.RoleValue(s, i.GuildID).ID
		}
	}
	if roleID == "" {
		replyEphemeral(s, i, "ロールが指定されていないよ。")
		return
	}
	rid, err := strconv.ParseInt(roleID, 10, 64)
	if err != nil {
		replyEphemeral(s, i, "ロールIDの解析に失敗したよ。")
		return
	}
	if err := b.repo.EnsureGuild(i.GuildID); err != nil {
		log.Printf("EnsureGuild: %v", err)
	}
	if err := b.repo.SetMentionRole(i.GuildID, rid); err != nil {
		replyEphemeral(s, i, "設定の保存に失敗したよ。")
		log.Printf("SetMentionRole: %v", err)
		return
	}
	replyEphemeral(s, i, fmt.Sprintf("通知時に <@&%s> をメンションするよ！", roleID))
	log.Printf("Set mention role %s for guild %s", roleID, i.GuildID)
}

func (b *Bot) cmdUnsetMentionRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ok, err := b.repo.UnsetMentionRole(i.GuildID)
	if err != nil {
		replyEphemeral(s, i, "設定の解除に失敗したよ。")
		log.Printf("UnsetMentionRole: %v", err)
		return
	}
	if ok {
		replyEphemeral(s, i, "メンションロールの設定を解除したよ！")
	} else {
		replyEphemeral(s, i, "メンションロールは設定されていないよ。")
	}
}

const findDailyLimit = 20

func (b *Bot) cmdFindDaily(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	var ids []int
	for _, opt := range data.Options {
		switch opt.Name {
		case "color1", "color2", "color3", "color4":
			ids = append(ids, int(opt.IntValue()))
		}
	}
	matches, err := b.history.FindByBalls(ids, findDailyLimit)
	l := discordLocaleToLang(i.Locale)
	if err != nil {
		if l == language.LangEN {
			replyEphemeral(s, i, "Search failed.")
		} else {
			replyEphemeral(s, i, "検索に失敗したよ。")
		}
		log.Printf("FindByBalls: %v", err)
		return
	}

	replyEphemeral(s, i, formatFindDailyResult(ids, matches, l))
}

func formatFindDailyResult(query []int, matches []dailyhistory.Match, lang language.Lang) string {
	queryNames := make([]string, len(query))
	for j, id := range query {
		queryNames[j] = balltext.Name(id, lang)
	}

	var sb strings.Builder
	if lang == language.LangEN {
		fmt.Fprintf(&sb, "Query: %s\n", strings.Join(queryNames, " / "))
	} else {
		fmt.Fprintf(&sb, "検索条件: %s\n", strings.Join(queryNames, " / "))
	}

	if len(matches) == 0 {
		if lang == language.LangEN {
			sb.WriteString("No matching days found.")
		} else {
			sb.WriteString("該当する日が見つからなかったよ。")
		}
		return sb.String()
	}

	prevDay := -1
	for _, m := range matches {
		balls := ballNames(m.BallIDs, lang)
		if m.DayNumber != prevDay {
			date := ball.Daily{DayNumber: m.DayNumber}.Date()
			fmt.Fprintf(&sb, "- %s: %s", date.Format("2006/01/02"), balls)
			prevDay = m.DayNumber
		} else {
			fmt.Fprintf(&sb, " → %s", balls)
		}
		if m.Revision == 0 && hasNextRevisionForDay(matches, m.DayNumber, m.Revision) {
			// next revision exists — leave the line open
			continue
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func hasNextRevisionForDay(matches []dailyhistory.Match, day, rev int) bool {
	for _, m := range matches {
		if m.DayNumber == day && m.Revision > rev {
			return true
		}
	}
	return false
}

func discordLocaleToLang(locale discordgo.Locale) language.Lang {
	if locale == discordgo.Japanese {
		return language.LangJA
	}
	return language.LangEN
}

func ballNames(ids []int, lang language.Lang) string {
	parts := make([]string, len(ids))
	for j, id := range ids {
		parts[j] = balltext.Name(id, lang)
	}
	sep := "・"
	if lang == language.LangEN {
		sep = " / "
	}
	return strings.Join(parts, sep)
}
