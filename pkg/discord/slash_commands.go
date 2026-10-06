package discord

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/OGA45/gomatalk/pkg/config"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/OGA45/gomatalk/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// voiceParamOptions returns the six numeric voice-parameter options shared by
// update_voice and update_bot_voice. Each call allocates fresh Min/Max value
// pointers (they must be addressable per registration).
func voiceParamOptions() []*discordgo.ApplicationCommandOption {
	speedMin := 0.5
	negTwenty := -20.0
	zero := 0.0
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionNumber, Name: "話す速度", Description: "speed", MaxValue: 2.0, MinValue: &speedMin, Required: true},
		{Type: discordgo.ApplicationCommandOptionNumber, Name: "声のトーン", Description: "tone", MaxValue: 20.0, MinValue: &negTwenty, Required: true},
		{Type: discordgo.ApplicationCommandOptionNumber, Name: "声のイントネーション", Description: "intone (AivisSpeech/VOICEROIDは0~2)", MaxValue: 4.0, MinValue: &zero, Required: true},
		{Type: discordgo.ApplicationCommandOptionNumber, Name: "閾値", Description: "threshold", MaxValue: 1.0, MinValue: &zero, Required: true},
		{Type: discordgo.ApplicationCommandOptionNumber, Name: "オールパス", Description: "allpass", MaxValue: 1.0, MinValue: &zero, Required: true},
		{Type: discordgo.ApplicationCommandOptionNumber, Name: "音量", Description: "volume", MaxValue: 20.0, MinValue: &negTwenty, Required: true},
	}
}

func Adding_slash_commands() {
	log.Println("INFO: Adding commands...")
	adminPerm := int64(discordgo.PermissionAdministrator)

	updateVoiceOptions := append([]*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "声の種類", Description: "voice", Required: true},
	}, voiceParamOptions()...)

	updateBotVoiceOptions := append([]*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "変更するbotのid", Description: "Bot-id", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "声の種類", Description: "voice", Required: true},
	}, voiceParamOptions()...)

	commands := []*discordgo.ApplicationCommand{
		{Name: "help", Description: "コマンド一覧と簡単な説明を表示"},
		{Name: "summon", Description: "読み上げを開始"},
		{Name: "bye", Description: "読み上げを終了"},
		{Name: "stop", Description: "読み上げを一時停止"},
		{Name: "words_list", Description: "辞書一覧を表示"},
		{
			Name:        "add_word",
			Description: "辞書登録",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "登録する単語", Description: "Set-word", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "登録する読み", Description: "Set-reading", Required: true},
			},
		},
		{
			Name:        "delete_word",
			Description: "辞書削除",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "削除する単語", Description: "Set-word", Required: true},
			},
		},
		{Name: "status", Description: "現在の声の設定を表示"},
		{
			Name:        "update_voice",
			Description: "声の設定を変更",
			Options:     updateVoiceOptions,
		},
		{
			Name:        "add_bot",
			Description: "BOTを読み上げ対象に登録",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "登録するbotのid", Description: "Bot-id", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "登録するwav-list", Description: "Wav-list"},
			},
		},
		{
			Name:        "delete_bot",
			Description: "BOTを読み上げ対象から削除",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "削除するbotのid", Description: "Bot-id", Required: true},
			},
		},
		{Name: "bots_list", Description: "読み上げ対象BOTの一覧を表示"},
		{Name: "random", Description: "自分の声をﾗﾝﾀﾞﾑで変更する"},
		{
			Name:        "update_bot_voice",
			Description: "BOTの音声を変更",
			Options:     updateBotVoiceOptions,
		},
		{
			Name:        "random_bot",
			Description: "BOTの声をﾗﾝﾀﾞﾑで変更する",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "変更するbotのid", Description: "Bot-id", Required: true},
			},
		},
		{
			Name:                     "reboot",
			Description:              "BOTを再起動する",
			DefaultMemberPermissions: &adminPerm,
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "パスワード", Description: "Secret-word", Required: true},
			},
		},
		{Name: "voices_list", Description: "登録されている声の一覧を表示"},
	}

	appID := Dg.State.User.ID

	// Do NOT use ApplicationCommandBulkOverwrite: it deletes the Activity's
	// PRIMARY_ENTRY_POINT command (type 4), which Discord manages and which
	// cannot be recreated via the bot API — breaking the "Launch" button.
	// Instead, diff against the live command list: leave the Entry Point
	// untouched, prune stale chat commands, and (re)create our definitions
	// (create upserts by name).
	existing, err := Dg.ApplicationCommands(appID, "")
	if err != nil {
		log.Println("ERROR: cannot list existing slash commands:", err)
		existing = nil // continue: still (re)create our own definitions
	}

	wanted := make(map[string]bool, len(commands))
	for _, c := range commands {
		wanted[c.Name] = true
	}

	hasEntryPoint := false
	for _, c := range existing {
		if c.Type == discordgo.PrimaryEntryPointApplicationCommand {
			// The Activity "Launch" command — never delete or recreate it.
			hasEntryPoint = true
			continue
		}
		if !wanted[c.Name] {
			if err := Dg.ApplicationCommandDelete(appID, "", c.ID); err != nil {
				log.Println("ERROR: cannot delete stale slash command", c.Name, ":", err)
			}
		}
	}

	for _, c := range commands {
		if _, err := Dg.ApplicationCommandCreate(appID, "", c); err != nil {
			log.Println("ERROR: cannot register slash command", c.Name, ":", err)
		}
	}

	// Discord creates the Entry Point command when Activities are first
	// enabled, but it does NOT recreate it if it was ever deleted — and the
	// old BulkOverwrite registration used here deleted it on every startup.
	// Recreate it so the Activity is launchable from the App Launcher.
	if config.O().Activity.Enabled && !hasEntryPoint {
		_, err := Dg.ApplicationCommandCreate(appID, "", &discordgo.ApplicationCommand{
			Name:        "launch",
			Description: "アクティビティを起動",
			Type:        discordgo.PrimaryEntryPointApplicationCommand,
			Handler:     discordgo.DiscordLaunchActivityEntryPointCommand,
		})
		if err != nil {
			log.Println("ERROR: cannot create activity entry point command:", err)
		} else {
			log.Println("INFO: created activity entry point command (launch)")
		}
	}
}

// interactionUser returns the invoking user, handling both guild (Member) and
// DM (User) interactions; nil if neither is present.
func interactionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User
	}
	return i.User
}

// optionMap indexes an interaction's options by name.
func optionMap(i *discordgo.InteractionCreate) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	options := i.ApplicationCommandData().Options
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		m[opt.Name] = opt
	}
	return m
}

// parseVoiceOptions builds a UserInfo from the shared voice-parameter options.
func parseVoiceOptions(opts map[string]*discordgo.ApplicationCommandInteractionDataOption) model.UserInfo {
	ui := model.UserInfo{}
	if o, ok := opts["声の種類"]; ok {
		ui.Voice = o.StringValue()
	}
	if o, ok := opts["話す速度"]; ok {
		ui.Speed = o.FloatValue()
	}
	if o, ok := opts["声のトーン"]; ok {
		ui.Tone = o.FloatValue()
	}
	if o, ok := opts["声のイントネーション"]; ok {
		ui.Intone = o.FloatValue()
	}
	if o, ok := opts["閾値"]; ok {
		ui.Threshold = o.FloatValue()
	}
	if o, ok := opts["オールパス"]; ok {
		ui.AllPass = o.FloatValue()
	}
	if o, ok := opts["音量"]; ok {
		ui.Volume = o.FloatValue()
	}
	return ui
}

// voiceCredits are the attributions the voice licences require (VOICEVOX
// terms of use; CC BY for the HTS voices).
const voiceCredits = "VOICEVOX:各キャラクター（/voices_list の VOICEVOX の声）\n" +
	"AivisSpeech: まお / コハク (Aivis Common Model License 1.0)\n" +
	"HTS Voice \"Mei\" / \"Takumi\" (c) Nagoya Institute of Technology, CC BY 3.0\n" +
	"HTS Voice \"NIT ATR503 M001\" (c) Nagoya Institute of Technology, CC BY 3.0\n" +
	"HTS Voice \"tohoku-f01\" (c) Ito-Nose Lab., Tohoku University, CC BY 4.0"

func Help(s *discordgo.Session, i *discordgo.InteractionCreate) {
	fields := []*discordgo.MessageEmbedField{
		{Name: "/help", Value: "```コマンド一覧と簡単な説明を表示する```"},
		{Name: "/summon", Value: "```読み上げを開始する```"},
		{Name: "/bye", Value: "```読み上げを終了する```"},
		{Name: "/add_word", Value: "```単語を辞書に登録する```"},
		{Name: "/delete_word", Value: "```単語を辞書から削除する```"},
		{Name: "/words_list", Value: "```辞書一覧を表示する```"},
		{Name: "/add_bot", Value: "```BOTを読み上げ対象に登録する```"},
		{Name: "/delete_bot", Value: "```BOTを読み上げ対象から削除する```"},
		{Name: "/bots_list", Value: "```読み上げ対象BOTの一覧を表示する```"},
		{Name: "/random", Value: "```自分の声をﾗﾝﾀﾞﾑで変更する```"},
		{Name: "/status", Value: "```自分の現在の声の設定を表示する```"},
		{Name: "/update_voice", Value: "```自分の声の設定を変更する```"},
		{Name: "/update_bot_voice", Value: "```BOTの声の設定を変更する```"},
		{Name: "/random_bot", Value: "```BOTの声をﾗﾝﾀﾞﾑで変更する```"},
		{Name: "/reboot", Value: "```BOTを再起動する\n当然他のサーバーにも影響が出るので注意```"},
		{Name: "/stop", Value: "```読み上げを一時停止```"},
		{Name: "/voices_list", Value: "```登録されている声の一覧を表示する```"},
		{Name: "音声クレジット", Value: "```" + voiceCredits + "```"},
	}
	respondEmbed(s, i, &discordgo.MessageEmbed{
		Title:       "コマンド一覧",
		Description: ":warning: **注意:旧コマンドは非推奨なので表記から消しています。**",
		Color:       colorInfo,
		Fields:      fields,
	}, false)
}

func Voices_list(s *discordgo.Session, i *discordgo.InteractionCreate) {
	respondInfo(s, i, "登録されている声の一覧", "```\n"+voice.CategorizedVoiceList()+"```")
}

func Summon(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	if user == nil {
		respondError(s, i, "**サーバー内で実行してください。**")
		return
	}
	log.Println("INFO:", user.Username, "send 'join'")
	voiceChannelID := SearchVoiceChannel(user.ID)
	if voiceChannelID == "" {
		respondError(s, i, "**貴方はVCに参加していません。**")
		return
	}
	guildID := i.GuildID

	v, created := global.CreateInstanceIfAbsent(guildID, func() *voice.VoiceInstance {
		return &voice.VoiceInstance{
			GuildID:   guildID,
			Session:   s,
			ChannelID: i.ChannelID,
			Stop:      make(chan bool, 1),
		}
	})
	if !created {
		respondError(s, i, "**すでに参加しています。**")
		return
	}

	vc, err := Dg.ChannelVoiceJoin(guildID, voiceChannelID, false, false)
	if err != nil {
		global.DeleteInstance(guildID) // drop the orphan so a retry can recreate it
		log.Println("ERROR: Error to join in a voice channel: ", err)
		respondError(s, i, "**VCに参加できませんでした。開発者にお問い合わせください。**")
		return
	}
	v.SetVoice(vc)
	if config.O().Discord.Debug {
		vc.LogLevel = discordgo.LogDebug
	}
	respondSuccess(s, i, "**読み上げを開始します。**")
}

func Bye(s *discordgo.Session, i *discordgo.InteractionCreate) {
	v := global.GetInstance(i.GuildID)
	if v == nil {
		respondError(s, i, "**VCに参加していません。**")
		return
	}
	closeConnection(v)
	respondSuccess(s, i, "**終了します。**")
}

func Stop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	if user == nil {
		respondError(s, i, "**サーバー内で実行してください。**")
		return
	}
	v := global.GetInstance(i.GuildID)
	if v == nil {
		respondError(s, i, "**VCに参加していません。**")
		return
	}
	if v.ChannelID != SearchVoiceChannel(user.ID) {
		respondError(s, i, "**自分が参加していないVCの読み上げを止めることは出来ません。**")
		return
	}
	v.StopTalking()
	respondSuccess(s, i, "**読み上げを一時停止します。**")
}

func Words_list(s *discordgo.Session, i *discordgo.InteractionCreate) {
	wordsList, err := global.DB.ListWords(i.GuildID)
	if err != nil {
		respondError(s, i, msgGetInfoFailed)
		return
	}
	var msg string
	for k, v := range wordsList {
		msg += fmt.Sprintf("・単語: %s、読み: %s\n", k, v)
	}
	if msg == "" {
		msg = "現在何も登録されていません。"
	}
	respondInfo(s, i, "登録されている単語一覧", "```"+msg+"```")
}

func Add_word(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var word, reading string
	if o, ok := opts["登録する単語"]; ok {
		word = o.StringValue()
	}
	if o, ok := opts["登録する読み"]; ok {
		reading = o.StringValue()
	}
	if err := global.DB.AddWord(i.GuildID, word, reading); err != nil {
		respondError(s, i, fmt.Sprintf("**単語「%s」の登録に失敗しました。**", word))
		return
	}
	respondSuccess(s, i, fmt.Sprintf("**単語「%s」を読み「%s」で登録しました。**", word, reading))
}

func Delete_word(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var word string
	if o, ok := opts["削除する単語"]; ok {
		word = o.StringValue()
	}
	wordsList, err := global.DB.ListWords(i.GuildID)
	if err != nil {
		respondError(s, i, msgGetInfoFailed)
		return
	}
	if _, ok := wordsList[word]; !ok {
		respondError(s, i, fmt.Sprintf("**単語「%s」は見つかりませんでした。**", word))
		return
	}
	if err := global.DB.DeleteWord(i.GuildID, word); err != nil {
		respondError(s, i, fmt.Sprintf("**単語「%s」の削除に失敗しました**", word))
		return
	}
	respondSuccess(s, i, fmt.Sprintf("**単語「%s」を削除しました。**", word))
}

func Status(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	if user == nil {
		respondError(s, i, msgGetInfoFailed)
		return
	}
	DBUser, err := global.DB.GetUser(user.ID)
	if err != nil {
		log.Println("INFO: Cannot Get User info")
		DBUser, err = global.DB.NewUser(user.ID)
		if err != nil {
			log.Println("ERR: Cannot initialize User")
			respondError(s, i, msgGetInfoFailed)
			return
		}
	}
	respondInfo(s, i, "現在の音声情報", "```"+FormatUserInfo(DBUser.UserInfo)+"```")
}

// voiceUpdateErrorMessage explains a rejected voice update; only real
// failures get the generic "contact the developer" message.
func voiceUpdateErrorMessage(err error) string {
	if errors.Is(err, voice.ErrOutOfRange) {
		return msgOutOfRange
	}
	return msgGetInfoFailed
}

func Update_voice(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	if user == nil {
		respondError(s, i, msgGetInfoFailed)
		return
	}
	ui := parseVoiceOptions(optionMap(i))
	if _, ok := voice.Voices()[ui.Voice]; !ok {
		log.Println("Not find key", ui.Voice)
		respondError(s, i, fmt.Sprintf("**「%s」というボイスが見つかりませんでした。**", ui.Voice))
		return
	}
	if err := applyVoiceUpdate(user.ID, ui); err != nil {
		respondError(s, i, voiceUpdateErrorMessage(err))
		return
	}
	respondSuccess(s, i, "**以下の情報で登録しました。\n"+FormatUserInfo(ui)+"**")
}

func Add_bot(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var botID string
	wavList := []string{}
	if o, ok := opts["登録するbotのid"]; ok {
		botID = o.StringValue()
	}
	if o, ok := opts["登録するwav-list"]; ok {
		wavList = strings.Split(o.StringValue(), ",")
	}
	name, _, found := resolveBotName(botID)
	if !found {
		respondError(s, i, fmt.Sprintf("**ID「%s」のBOTは見つかりませんでした。**", botID))
		return
	}
	if err := global.DB.AddBot(i.GuildID, botID, wavList); err != nil {
		respondError(s, i, fmt.Sprintf("**BOT「%s」の登録に失敗しました。**", name))
		return
	}
	respondSuccess(s, i, fmt.Sprintf("**BOT「%s」を読み上げ対象に登録しました。**", name))
}

func Delete_bot(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var botID string
	if o, ok := opts["削除するbotのid"]; ok {
		botID = o.StringValue()
	}
	registered, err := isRegisteredBot(i.GuildID, botID)
	if err != nil {
		respondError(s, i, "**読み上げ対象BOTの一覧の取得に失敗しました。**")
		return
	}
	if !registered {
		respondError(s, i, fmt.Sprintf("**BOT ID「%s」は見つかりませんでした。**", botID))
		return
	}
	if err := global.DB.DeleteBot(i.GuildID, botID); err != nil {
		respondError(s, i, fmt.Sprintf("**BOT ID「%s」の削除に失敗しました。**", botID))
		return
	}
	respondSuccess(s, i, fmt.Sprintf("**BOT ID「%s」を削除しました。**", botID))
}

func Bots_list(s *discordgo.Session, i *discordgo.InteractionCreate) {
	botList, err := global.DB.ListBots(i.GuildID)
	if err != nil {
		respondError(s, i, "**読み上げ対象BOTの一覧の取得に失敗しました。**")
		return
	}
	if len(botList) == 0 {
		respondInfo(s, i, "登録されているBOT一覧", "**現在何も登録されていません。**")
		return
	}
	fields := []*discordgo.MessageEmbedField{}
	for botID, wav := range botList {
		if len(wav) > 0 && wav[0] == "" {
			wav[0] = "None"
		}
		name, _, found := resolveBotName(botID)
		if !found {
			name = botID
		}
		DBUser, err := global.DB.GetUser(botID)
		if err != nil {
			log.Println("INFO: Cannot Get User info")
			DBUser, err = global.DB.NewUser(botID)
			if err != nil {
				log.Println("ERR: Cannot initialize User")
				respondError(s, i, msgGetInfoFailed)
				return
			}
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s(%s) WAV LIST: %s\n", name, botID, strings.Join(wav, ",")),
			Value:  "```" + FormatUserInfo(DBUser.UserInfo) + "```",
			Inline: false,
		})
	}
	respondInfoFields(s, i, "登録されているBOT一覧", fields)
}

func Random(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	if user == nil {
		respondError(s, i, msgGetInfoFailed)
		return
	}
	ui, err := applyRandom(user.ID)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		respondError(s, i, msgGetInfoFailed)
		return
	}
	respondSuccess(s, i, "**以下の情報で登録しました。\n"+FormatUserInfo(ui)+"**")
}

func Update_bot_voice(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var botID string
	if o, ok := opts["変更するbotのid"]; ok {
		botID = o.StringValue()
	}
	name, isBot, found := resolveBotName(botID)
	if !found {
		respondError(s, i, fmt.Sprintf("**ID「%s」のBOTは見つかりませんでした。**", botID))
		return
	}
	if !isBot {
		respondError(s, i, "**声変えられるのはBotのみです。**")
		return
	}
	registered, err := isRegisteredBot(i.GuildID, botID)
	if err != nil {
		respondError(s, i, "**読み上げ対象BOTの一覧の取得に失敗しました。**")
		return
	}
	if !registered {
		respondError(s, i, fmt.Sprintf("**BOT ID「%s」は読み上げ登録されていません。**", botID))
		return
	}
	ui := parseVoiceOptions(opts)
	if _, ok := voice.Voices()[ui.Voice]; !ok {
		log.Println("Not find key", ui.Voice)
		respondError(s, i, fmt.Sprintf("**「%s」というボイスが見つかりませんでした。**", ui.Voice))
		return
	}
	if err := applyVoiceUpdate(botID, ui); err != nil {
		respondError(s, i, voiceUpdateErrorMessage(err))
		return
	}
	respondSuccess(s, i, fmt.Sprintf("**BOT「%s」の音声を以下の情報で登録しました。**\n", name)+FormatUserInfo(ui))
}

func Random_bot(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var botID string
	if o, ok := opts["変更するbotのid"]; ok {
		botID = o.StringValue()
	}
	name, isBot, found := resolveBotName(botID)
	if !found {
		respondError(s, i, fmt.Sprintf("**ID「%s」のBOTは見つかりませんでした。**", botID))
		return
	}
	if !isBot {
		respondError(s, i, "**声変えられるのはBotのみです。**")
		return
	}
	registered, err := isRegisteredBot(i.GuildID, botID)
	if err != nil {
		respondError(s, i, msgGetInfoFailed)
		return
	}
	if !registered {
		respondError(s, i, fmt.Sprintf("**BOT ID「%s」は見つかりませんでした。**", botID))
		return
	}
	ui, err := applyRandom(botID)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		respondError(s, i, msgGetInfoFailed)
		return
	}
	respondSuccess(s, i, fmt.Sprintf("**BOT「%s」の音声を以下の情報で登録しました。**\n", name)+FormatUserInfo(ui))
}

func Reboot(s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optionMap(i)
	var secret string
	if o, ok := opts["パスワード"]; ok {
		secret = o.StringValue()
	}

	invoker := "unknown"
	if user := interactionUser(i); user != nil {
		invoker = fmt.Sprintf("%s(%s)", user.Username, user.ID)
	}

	if secret != config.O().Discord.Secret {
		log.Printf("WARN: reboot DENIED for %s in guild %s", invoker, i.GuildID)
		respondError(s, i, "**パスワードが違います。**")
		return
	}

	log.Printf("INFO: reboot requested by %s in guild %s", invoker, i.GuildID)
	respondSuccess(s, i, "**再起動します。**")
	// Graceful: disconnect voice connections, then exit non-zero so the
	// supervisor (Docker restart policy) brings the process back up.
	go func() {
		Shutdown()
		os.Exit(1)
	}()
}
