package discord

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/play"
	"github.com/OGA45/gomatalk/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

var (
	Dg *discordgo.Session
)

// DiscordConnect make a new connection to Discord
func DiscordConnect() (err error) {
	Dg, err = discordgo.New("Bot " + config.O().Discord.Token)
	if err != nil {
		log.Println("FATA: error creating Discord session,", err)
		return
	}
	log.Println("INFO: Bot is Opening")
	Dg.AddHandler(MessageCreateHandler)
	Dg.AddHandler(GuildCreateHandler)
	Dg.AddHandler(VoiceStatusUpdateHandler)
	Dg.AddHandler(VoiceServerUpdateHandler)
	Dg.AddHandler(ConnectHandler)
	Dg.AddHandler(SlashCommandHandler)
	if config.O().Discord.NumShard > 1 {
		Dg.ShardCount = config.O().Discord.NumShard
		Dg.ShardID = config.O().Discord.ShardID
	}

	if config.O().Discord.Debug {
		Dg.LogLevel = discordgo.LogDebug
	}
	// Open Websocket
	err = Dg.Open()
	if err != nil {
		log.Println("FATA: Error Open():", err)
		return
	}
	_, err = Dg.User("@me")
	if err != nil {
		// Login unsuccessful
		log.Println("FATA:", err)
		return
	} // Login successful
	initRoutine()
	gatewayWatchdog()
	go Adding_slash_commands()
	log.Println("INFO: Bot is now running. Press CTRL-C to exit.")
	Dg.UpdateGameStatus(0, config.O().Discord.Status)
	return nil
}

// Shutdown disconnects all voice instances and closes the Discord session.
func Shutdown() {
	global.CloseAllInstances()
	if Dg != nil {
		if err := Dg.Close(); err != nil {
			log.Println("ERROR: Dg.Close:", err)
		}
	}
}

// SearchVoiceChannel search the voice channel id into from guild.
func SearchVoiceChannel(user string) (voiceChannelID string) {
	for _, g := range Dg.State.Guilds {
		for _, v := range g.VoiceStates {
			if v.UserID == user {
				return v.ChannelID
			}
		}
	}
	return ""
}

func UserCountVoiceChannel(voiceChannel string) int {
	count := 0
	for _, g := range Dg.State.Guilds {
		for _, v := range g.VoiceStates {
			if v.ChannelID != voiceChannel {
				continue
			}
			user, err := Dg.User(v.UserID)
			if err != nil || user == nil {
				continue
			}
			if !user.Bot {
				count++
			}
		}
	}
	return count
}

// SearchGuild returns the guild ID for a text channel, or "" if it cannot be
// resolved (Dg.Channel can return a nil channel + error).
func SearchGuild(textChannelID string) (guildID string) {
	channel, err := Dg.Channel(textChannelID)
	if err != nil || channel == nil {
		return ""
	}
	return channel.GuildID
}

// ChMessageSend sends a message, retrying a few times on transient errors and
// bailing immediately on permanent (4xx) ones instead of blocking ~10s.
func ChMessageSend(textChannelID, message string) {
	for attempt := 0; attempt < 3; attempt++ {
		_, err := Dg.ChannelMessageSend(textChannelID, message)
		if err == nil {
			return
		}
		if isPermanentRESTError(err) {
			log.Println("ERROR: ChannelMessageSend:", err)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func ChFileSend(textChannelID, name, message string) {
	if _, err := Dg.ChannelFileSend(textChannelID, name, strings.NewReader(message)); err != nil {
		log.Println("ERROR: ChannelFileSend:", err)
	}
}

// ChMessageSendEmbed send an embeded messages.
func ChMessageSendEmbed(textChannelID, title, description string, user discordgo.User) {
	embed := discordgo.MessageEmbed{}
	embed.Title = title
	embed.Description = description
	embed.Color = colorError
	author := discordgo.MessageEmbedAuthor{}
	author.Name = user.Username
	author.IconURL = user.AvatarURL("")
	embed.Author = &author
	for attempt := 0; attempt < 3; attempt++ {
		_, err := Dg.ChannelMessageSendEmbed(textChannelID, &embed)
		if err == nil {
			return
		}
		if isPermanentRESTError(err) {
			log.Println("ERROR: ChannelMessageSendEmbed:", err)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// isPermanentRESTError reports whether err is a 4xx Discord REST error that
// will not be fixed by retrying.
func isPermanentRESTError(err error) bool {
	var rerr *discordgo.RESTError
	if errors.As(err, &rerr) && rerr.Response != nil {
		return rerr.Response.StatusCode >= 400 && rerr.Response.StatusCode < 500
	}
	return false
}

func initRoutine() {
	global.SpeechSignal = make(chan voice.SpeechSignal)
	go play.GlobalPlay(global.SpeechSignal)
}

func recoverHandler(name string) {
	if r := recover(); r != nil {
		log.Printf("ERROR: recovered from panic in %s: %v", name, r)
	}
}

// VoiceServerUpdateHandler は音声サーバー移行を検知してログ出力する
// （自動再接続は yeongaori 版 DAVE 実装の制約により無効化）
func VoiceServerUpdateHandler(s *discordgo.Session, vs *discordgo.VoiceServerUpdate) {
	v := global.GetInstance(vs.GuildID)
	if v == nil || v.GetVoice() == nil {
		return
	}
	log.Printf("INFO: Voice server update detected for guild %s (internal reconnect)", vs.GuildID)
}

// ConnectHandler
func ConnectHandler(s *discordgo.Session, connect *discordgo.Connect) {
	s.UpdateGameStatus(0, config.O().Discord.Status)
}

// SlashCommandHandler
func SlashCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	defer recoverHandler("SlashCommandHandler")
	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"help":             Help,
		"voices_list":      Voices_list,
		"summon":           Summon,
		"bye":              Bye,
		"stop":             Stop,
		"words_list":       Words_list,
		"add_word":         Add_word,
		"delete_word":      Delete_word,
		"status":           Status,
		"update_voice":     Update_voice,
		"add_bot":          Add_bot,
		"delete_bot":       Delete_bot,
		"bots_list":        Bots_list,
		"random":           Random,
		"update_bot_voice": Update_bot_voice,
		"random_bot":       Random_bot,
		"reboot":           Reboot,
	}
	if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
		h(s, i)
	}
}

// GuildCreateHandler
func GuildCreateHandler(s *discordgo.Session, guild *discordgo.GuildCreate) {
	defer recoverHandler("GuildCreateHandler")
	log.Println("INFO: Guild Create:", guild.ID)
	err := global.DB.CreateGuild(guild.ID)
	if err != nil {
		log.Println("FATA: DB", err)
		return
	}
}

func VoiceStatusUpdateHandler(s *discordgo.Session, vsu *discordgo.VoiceStateUpdate) {
	defer recoverHandler("VoiceStatusUpdateHandler")

	// Bot 自身の入退室/移動を常時記録する（可視の入退出の決定的な証拠になる）。
	if s.State != nil && s.State.User != nil && vsu.UserID == s.State.User.ID {
		before := ""
		if vsu.BeforeUpdate != nil {
			before = vsu.BeforeUpdate.ChannelID
		}
		if before != vsu.ChannelID {
			log.Printf("INFO: bot voice state changed in guild %s: %q -> %q", vsu.GuildID, before, vsu.ChannelID)
		}
	}

	v := global.GetInstance(vsu.GuildID)
	if v == nil || v.GetVoice() == nil {
		return
	}

	botID := ""
	if s.State != nil && s.State.User != nil {
		botID = s.State.User.ID
	}

	// The bot itself was moved between channels by a user -> follow it.
	// BeforeUpdate.ChannelID must be non-empty: a transition from "" is a
	// (re)join, not a move, and re-joining here would race the library's own
	// reconnect logic.
	if vsu.UserID == botID {
		if vsu.BeforeUpdate != nil && vsu.BeforeUpdate.ChannelID != "" && vsu.ChannelID != "" && vsu.BeforeUpdate.ChannelID != vsu.ChannelID {
			if vc, err := Dg.ChannelVoiceJoin(v.GuildID, vsu.ChannelID, false, false); err == nil {
				v.SetVoice(vc)
			}
		}
		return
	}

	// Ignore other bots' voice state changes.
	if user, err := Dg.User(vsu.UserID); err == nil && user != nil && user.Bot {
		return
	}

	// If no humans remain in the bot's channel, leave.
	if UserCountVoiceChannel(v.ChannelID) == 0 {
		closeConnection(v)
		ChMessageSend(v.ChannelID, config.O().Greeting["nobody"])
	}
}

// MessageCreateHandler
func MessageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	defer recoverHandler("MessageCreateHandler")
	guildID := SearchGuild(m.ChannelID)
	if guildID == "" {
		return
	}
	botList, _ := global.DB.ListBots(guildID)
	isSpecial := false
	if m.Author.Bot {
		if _, ok := botList[m.Author.ID]; !ok {
			return
		}
		isSpecial = true
	}
	v := global.GetInstance(guildID)
	if strings.HasPrefix(m.Content, config.O().Discord.Prefix) {
		content := strings.Replace(m.Content, config.O().Discord.Prefix, "", 1)
		command := strings.Fields(content)

		if len(command) == 0 {
			return
		}

		switch command[0] {
		case "help", "h":
			HelpReporter(m)
		case "summon", "s":
			JoinReporter(v, m, s)
		case "bye", "b":
			LeaveReporter(v, m)
		case "stop":
			StopReporter(v, m)
		case "words_list", "wl":
			ListWordsReporter(m)
		case "add_word", "aw":
			AddWordReporter(m)
		case "delete_word", "dw":
			DeleteWordReporter(m)
		case "status":
			StatusReporter(m)
		case "update_voice", "uv":
			SetStatusHandler(m)
		case "add_bot", "ab":
			AddBotReporter(m)
		case "delete_bot", "db":
			DeleteBotReporter(m)
		case "bots_list", "bl":
			ListBotReporter(m)
		case "random", "r":
			MakeRandomHandler(m)
		case "update_bot_voice", "ubv":
			SetStatusForOtherHandler(m)
		case "random_bot", "rb":
			MakeRandomForOther(m)
		case "reboot":
			RebootReporter(m)
		default:
			return
		}
		return
	}
	if v != nil && v.GetVoice() != nil {
		if !isSpecial && v.ChannelID != m.ChannelID {
			return
		}
		SpeechText(v, m, botList)
	}
}
