package bot

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/settings"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/watcher"
)

var jst = time.FixedZone("JST", 9*3600)

type Bot struct {
	session  *discordgo.Session
	repo     *settings.Repository
	history  *dailyhistory.Repository
	notifier *Notifier
	logDir   string
	stateDir string

	detector *Detector

	readyCh       chan struct{}
	readyOnce     sync.Once
	initialGuilds sync.Map // guildID -> struct{}: known via Ready or already-logged join
}

func New(repo *settings.Repository, history *dailyhistory.Repository, token, logDir, stateDir string) (*Bot, error) {
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
		logDir:   logDir,
		stateDir: stateDir,
		readyCh:  make(chan struct{}),
	}
	b.registerHandlers()
	return b, nil
}

func (b *Bot) Run(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}
	defer b.session.Close()

	if err := b.registerCommands(); err != nil {
		return fmt.Errorf("register commands: %w", err)
	}

	detector, err := NewDetector(b.history, b.notifier)
	if err != nil {
		return fmt.Errorf("init detector: %w", err)
	}
	b.detector = detector

	stateFile := filepath.Join(b.stateDir, "state.json")
	repo := watcher.NewFileRepo(stateFile)

	newHandler := func(_ string) watcher.LineHandler {
		return func(_, line string) {
			b.detector.OnLine(line)
		}
	}

	w := watcher.NewLogWatcher(b.logDir, newHandler, repo, true)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Run(ctx)
	}()

	<-ctx.Done()
	wg.Wait()
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
		Description: "このチャンネルに通知を送るよう設定します",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "lang",
				Description: "通知言語（デフォルトは ja）",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "ja", Value: "ja"},
					{Name: "en", Value: "en"},
				},
			},
		},
	},
	{
		Name:        "unsetchannel",
		Description: "このチャンネルの通知設定を解除します",
	},
	{
		Name:        "setmentionrole",
		Description: "通知時にメンションするロールを設定します",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "メンションするロール",
				Required:    true,
			},
		},
	},
	{
		Name:        "unsetmentionrole",
		Description: "通知時のメンションロール設定を解除します",
	},
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
	}
}

func reply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

func channelMention(channelID string) string {
	return "<#" + channelID + ">"
}

func (b *Bot) cmdSetChannel(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	lang := ball.LangJA
	for _, opt := range data.Options {
		if opt.Name == "lang" {
			v := opt.StringValue()
			if ball.Lang(v).Valid() {
				lang = ball.Lang(v)
			}
		}
	}

	guildID := i.GuildID
	channelID := i.ChannelID
	chID, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		reply(s, i, "チャンネルIDの解析に失敗したよ。")
		return
	}

	if err := b.repo.EnsureGuild(guildID); err != nil {
		log.Printf("EnsureGuild: %v", err)
	}
	if err := b.repo.SetChannel(guildID, chID, lang); err != nil {
		reply(s, i, "設定の保存に失敗したよ。")
		log.Printf("SetChannel: %v", err)
		return
	}

	label := "日本語"
	if lang == ball.LangEN {
		label = "English"
	}
	reply(s, i, fmt.Sprintf("%s に通知を送るよ！（言語: %s）", channelMention(channelID), label))
	log.Printf("Set channel %s (lang=%s) for guild %s", channelID, lang, guildID)
}

func (b *Bot) cmdUnsetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	chID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		reply(s, i, "チャンネルIDの解析に失敗したよ。")
		return
	}
	ok, err := b.repo.UnsetChannel(i.GuildID, chID)
	if err != nil {
		reply(s, i, "設定の解除に失敗したよ。")
		log.Printf("UnsetChannel: %v", err)
		return
	}
	mention := channelMention(i.ChannelID)
	if ok {
		reply(s, i, fmt.Sprintf("%s の通知チャンネル設定を解除したよ！", mention))
	} else {
		reply(s, i, fmt.Sprintf("%s は通知チャンネルに設定されていないよ。", mention))
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
		reply(s, i, "ロールが指定されていないよ。")
		return
	}
	rid, err := strconv.ParseInt(roleID, 10, 64)
	if err != nil {
		reply(s, i, "ロールIDの解析に失敗したよ。")
		return
	}
	if err := b.repo.EnsureGuild(i.GuildID); err != nil {
		log.Printf("EnsureGuild: %v", err)
	}
	if err := b.repo.SetMentionRole(i.GuildID, rid); err != nil {
		reply(s, i, "設定の保存に失敗したよ。")
		log.Printf("SetMentionRole: %v", err)
		return
	}
	reply(s, i, fmt.Sprintf("通知時に <@&%s> をメンションするよ！", roleID))
	log.Printf("Set mention role %s for guild %s", roleID, i.GuildID)
}

func (b *Bot) cmdUnsetMentionRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ok, err := b.repo.UnsetMentionRole(i.GuildID)
	if err != nil {
		reply(s, i, "設定の解除に失敗したよ。")
		log.Printf("UnsetMentionRole: %v", err)
		return
	}
	if ok {
		reply(s, i, "メンションロールの設定を解除したよ！")
	} else {
		reply(s, i, "メンションロールは設定されていないよ。")
	}
}
