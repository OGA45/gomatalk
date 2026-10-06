package discord

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/OGA45/gomatalk/pkg/play"
	"github.com/OGA45/gomatalk/pkg/util"
	"github.com/OGA45/gomatalk/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// Compiled once at init; *Regexp is safe for concurrent use. Previously these
// were recompiled on every message.
var (
	reCustomEmoji  = regexp.MustCompile(`<:([^:]+):\d+>`)
	reAnimEmoji    = regexp.MustCompile(`<a:([^:]+):\d+>`)
	reURL          = regexp.MustCompile(`https?://[\w!\?/\+\-_~=;\.,\*&@#\$%\(\)'\[\]]+`)
	reSlashCommand = regexp.MustCompile(`</([^:]+):\d+>`)
	reSplitArgs    = regexp.MustCompile(`['"](\s*[^'"]+)\s*['"]|(\S+)`)
)

// HelpReporter
func HelpReporter(m *discordgo.MessageCreate) {
	log.Println("INFO:", m.Author.Username, "send 'help'")
	p := config.O().Discord.Prefix
	help := "コマンド一覧\n" +
		p + "help or " + p + "h  ->  コマンド一覧と簡単な説明を表示.\n" +
		p + "summon or " + p + "s  ->  読み上げを開始.\n" +
		p + "bye or " + p + "b  ->  読み上げを終了.\n" +
		p + "add_word or " + p + "aw  ->  辞書登録. (" + p + "aw 単語 読み" + ")\n" +
		p + "delete_word or " + p + "dw  ->  辞書削除. (" + p + "dw 単語" + ")\n" +
		p + "words_list or " + p + "wl  ->  辞書一覧を表示.\n" +
		p + "add_bot or " + p + "ab  ->  BOTを読み上げ対象に登録. (" + p + "ab <BOT ID> <WAV LIST>" + ")\n" +
		p + "delete_bot or " + p + "db  ->  BOTを読み上げ対象から削除. (" + p + "db <BOT ID>" + ")\n" +
		p + "bots_list or " + p + "bl  ->  読み上げ対象BOTの一覧を表示.\n" +
		p + "random or " + p + "r  ->  自分の声をﾗﾝﾀﾞﾑで変更する.\n" +
		p + "status ->  現在の声の設定を表示.\n" +
		p + "update_voice or " + p + "uv  ->  声の設定を変更. (" + p + "uv voice speed tone intone threshold volume" + ")\n" +
		"   voice: 声の種類 (/voices_list で一覧表示)\n" +
		"   speed: 話す速度 範囲(0.5~2.0) \n" +
		"   tone : 声のトーン 範囲(-20~20) [VOICEROIDは 0.5 ~ 2] \n" +
		"   intone : 声のイントネーション 範囲(0.0~4.0)(初期値 1.0) [VOICEROID・AivisSpeech(@Aivis)は 0 ~ 2] \n" +
		"   threshold : ブツブツするときとか改善するかも?? 範囲(0.0~1.0)(初期値 0.5) \n" +
		"   allpass : よくわからん 範囲(0 - 1.0) (0はauto)  \n" +
		"   volume : 音量（dB） 範囲(-20~20)(初期値 1) \n" +
		p + "stop  ->  読み上げを一時停止.\n\n" +
		"音声クレジット\n" + voiceCredits
	ChFileSend(m.ChannelID, "help.txt", help)
}

// JoinReporter
func JoinReporter(v *voice.VoiceInstance, m *discordgo.MessageCreate, s *discordgo.Session) {
	log.Println("INFO:", m.Author.Username, "send 'join'")
	voiceChannelID := SearchVoiceChannel(m.Author.ID)
	if voiceChannelID == "" {
		log.Println("ERROR: Voice channel id not found.")
		ChMessageSend(m.ChannelID, "<@"+m.Author.ID+"> VCに参加者がいません。")
		return
	}
	already := false
	if v != nil {
		log.Println("INFO: A voice instance is already created.")
		ChMessageSend(m.ChannelID, "すでに参加しています。")
		if v.ChannelID == m.ChannelID {
			already = true
		}
	} else {
		log.Println("INFO: New Voice Instance created")
		guildID := SearchGuild(m.ChannelID)
		if guildID == "" {
			return
		}
		v, _ = global.CreateInstanceIfAbsent(guildID, func() *voice.VoiceInstance {
			return &voice.VoiceInstance{
				GuildID:   guildID,
				Session:   s,
				ChannelID: m.ChannelID,
				Stop:      make(chan bool, 1),
			}
		})
	}
	v.ChannelID = m.ChannelID
	vc, err := Dg.ChannelVoiceJoin(v.GuildID, voiceChannelID, false, false)
	if err != nil {
		v.StopTalking()
		log.Println("ERROR: Error to join in a voice channel: ", err)
		return
	}
	v.SetVoice(vc)
	if config.O().Discord.Debug {
		vc.LogLevel = discordgo.LogDebug
	}
	if !already {
		ChMessageSend(v.ChannelID, config.O().Greeting["join"])
	}
	ChMessageSend(m.ChannelID, "読み上げを開始します。")
}

// LeaveReporter
func LeaveReporter(v *voice.VoiceInstance, m *discordgo.MessageCreate) {
	log.Println("INFO:", m.Author.Username, "send 'leave'")
	if v == nil {
		log.Println("INFO: The bot is not joined in a voice channel")
		return
	}
	closeConnection(v)
	ChMessageSend(v.ChannelID, config.O().Greeting["leave"])
}

func closeConnection(v *voice.VoiceInstance) {
	time.Sleep(200 * time.Millisecond)
	v.Close() // idempotent + nil-safe
	log.Println("INFO: Voice channel destroyed")
	global.DeleteInstance(v.GuildID)
	Dg.UpdateGameStatus(0, config.O().Discord.Status)
}

func ListBotReporter(m *discordgo.MessageCreate) {
	botList, err := global.DB.ListBots(m.GuildID)
	if err != nil {
		return
	}

	msg := "```\n登録されているBOT一覧\n\n"
	for botID, wav := range botList {
		name, _, found := resolveBotName(botID)
		if !found {
			name = botID
		}
		msg += fmt.Sprintf("・BOT: %s(%s)、WAV LIST: %s\n", name, botID, strings.Join(wav, ","))
	}
	msg += "```"

	ChMessageSend(m.ChannelID, msg)
}

func AddBotReporter(m *discordgo.MessageCreate) {
	commands := splitString(m.Content)
	if len(commands) < 2 {
		HelpReporter(m)
		return
	}
	botID := commands[1]
	name, _, found := resolveBotName(botID)
	if !found {
		ChMessageSend(m.ChannelID, fmt.Sprintf("ID「%s」のBOTは見つかりませんでした。", botID))
		return
	}
	wavList := []string{}
	if len(commands) > 2 {
		wavList = strings.Split(commands[2], ",")
	}
	if err := global.DB.AddBot(m.GuildID, botID, wavList); err != nil {
		ChMessageSend(m.ChannelID, fmt.Sprintf("BOT「%s」の登録に失敗しました。", name))
		return
	}
	ChMessageSend(m.ChannelID, fmt.Sprintf("BOT「%s」を読み上げ対象に登録しました。", name))
}

func DeleteBotReporter(m *discordgo.MessageCreate) {
	commands := splitString(m.Content)
	if len(commands) != 2 {
		HelpReporter(m)
		return
	}
	if err := global.DB.DeleteBot(m.GuildID, commands[1]); err != nil {
		ChMessageSend(m.ChannelID, fmt.Sprintf("BOT ID「%s」の削除に失敗しました", commands[1]))
		return
	}
	ChMessageSend(m.ChannelID, fmt.Sprintf("BOT ID「%s」を削除しました", commands[1]))
}

func ListWordsReporter(m *discordgo.MessageCreate) {
	wordsList, err := global.DB.ListWords(m.GuildID)
	if err != nil {
		return
	}

	msg := "```\n登録されている単語一覧\n\n"
	for k, v := range wordsList {
		msg += fmt.Sprintf("・単語: %s、読み: %s\n", k, v)
	}
	msg += "```"

	ChMessageSend(m.ChannelID, msg)
}

func AddWordReporter(m *discordgo.MessageCreate) {
	commands := splitString(m.Content)
	if len(commands) != 3 {
		HelpReporter(m)
		return
	}
	if err := global.DB.AddWord(m.GuildID, commands[1], commands[2]); err != nil {
		ChMessageSend(m.ChannelID, fmt.Sprintf("単語「%s」の登録に失敗しました", commands[1]))
		return
	}
	ChMessageSend(m.ChannelID, fmt.Sprintf("単語「%s」を読み「%s」で登録しました", commands[1], commands[2]))
}

func DeleteWordReporter(m *discordgo.MessageCreate) {
	commands := splitString(m.Content)
	if len(commands) != 2 {
		HelpReporter(m)
		return
	}
	if err := global.DB.DeleteWord(m.GuildID, commands[1]); err != nil {
		ChMessageSend(m.ChannelID, fmt.Sprintf("単語「%s」の削除に失敗しました", commands[1]))
		return
	}
	ChMessageSend(m.ChannelID, fmt.Sprintf("単語「%s」を削除しました", commands[1]))
}

func splitString(s string) []string {
	// Split string with space, honoring single/double quoted groups.
	result := reSplitArgs.FindAllStringSubmatch(s, -1)
	var fields []string
	for _, val := range result {
		if val[1] != "" {
			fields = append(fields, val[1])
		} else {
			fields = append(fields, val[0])
		}
	}
	return fields
}

func StatusReporter(m *discordgo.MessageCreate) {
	statusReporterInternal(m.Author.ID, m)
}

func statusReporterInternal(userID string, m *discordgo.MessageCreate) {
	user, ok := resolveUser(userID)
	if !ok {
		log.Println("ERROR: Cannot find user information.")
		return
	}
	DBUser, err := global.DB.GetUser(userID)
	if err != nil {
		log.Println("INFO: Cannot Get User info")
		DBUser, err = global.DB.NewUser(userID)
		if err != nil {
			log.Println("ERROR: Cannot get user information.")
			return
		}
	}
	userInfo := DBUser.UserInfo
	msg := fmt.Sprintf("%s\n%suv %s %.1f %.1f %.1f %.1f %.1f %.1f",
		FormatUserInfo(userInfo),
		config.O().Discord.Prefix,
		userInfo.Voice,
		userInfo.Speed,
		userInfo.Tone,
		userInfo.Intone,
		userInfo.Threshold,
		userInfo.AllPass,
		userInfo.Volume)
	ChMessageSendEmbed(m.ChannelID, msg, "", *user)
}

func MakeRandomForOther(m *discordgo.MessageCreate) {
	commands := strings.Fields(m.Content)
	if len(commands) != 2 {
		HelpReporter(m)
		return
	}
	userID := commands[1]
	_, isBot, found := resolveBotName(userID)
	if !found {
		ChMessageSend(m.ChannelID, fmt.Sprintf("ID「%s」のBOTは見つかりませんでした。", userID))
		return
	}
	if !isBot {
		ChMessageSend(m.ChannelID, "声変えられるのはBotのみです。")
		return
	}
	makeRandomHandlerInternal(userID, m)
}

func MakeRandomHandler(m *discordgo.MessageCreate) {
	makeRandomHandlerInternal(m.Author.ID, m)
}

func makeRandomHandlerInternal(userID string, m *discordgo.MessageCreate) {
	if _, err := applyRandom(userID); err != nil {
		log.Println("ERROR: applyRandom:", err)
		return
	}
	statusReporterInternal(userID, m)
}

func setStatusHandlerInternal(userID string, userInfo model.UserInfo, m *discordgo.MessageCreate) {
	if err := applyVoiceUpdate(userID, userInfo); err != nil {
		log.Println("INFO: invalid voice setting:", err)
		HelpReporter(m)
		return
	}
	statusReporterInternal(userID, m)
}

// parseUserInfoArgs parses a voice name plus six numeric parameters
// (speed, tone, intone, threshold, allpass, volume), returning an error if any
// numeric arg is malformed instead of silently substituting 0.
func parseUserInfoArgs(voiceName string, nums []string) (model.UserInfo, error) {
	if len(nums) != 6 {
		return model.UserInfo{}, errors.New("wrong number of args")
	}
	vals := make([]float64, 6)
	for idx, n := range nums {
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return model.UserInfo{}, err
		}
		vals[idx] = f
	}
	return model.UserInfo{
		Voice:     voiceName,
		Speed:     vals[0],
		Tone:      vals[1],
		Intone:    vals[2],
		Threshold: vals[3],
		AllPass:   vals[4],
		Volume:    vals[5],
	}, nil
}

func SetStatusForOtherHandler(m *discordgo.MessageCreate) {
	commands := strings.Fields(m.Content)
	if len(commands) != 9 {
		HelpReporter(m)
		return
	}
	userID := commands[1]
	_, isBot, found := resolveBotName(userID)
	if !found {
		ChMessageSend(m.ChannelID, fmt.Sprintf("ID「%s」のBOTは見つかりませんでした。", userID))
		return
	}
	if !isBot {
		ChMessageSend(m.ChannelID, "声変えられるのはBotのみです。")
		return
	}
	userInfo, err := parseUserInfoArgs(commands[2], commands[3:9])
	if err != nil {
		HelpReporter(m)
		return
	}
	setStatusHandlerInternal(userID, userInfo, m)
}

func SetStatusHandler(m *discordgo.MessageCreate) {
	commands := strings.Fields(m.Content)
	if len(commands) != 8 {
		HelpReporter(m)
		return
	}
	userInfo, err := parseUserInfoArgs(commands[1], commands[2:8])
	if err != nil {
		HelpReporter(m)
		return
	}
	setStatusHandlerInternal(m.Author.ID, userInfo, m)
}

func StopReporter(v *voice.VoiceInstance, m *discordgo.MessageCreate) {
	log.Println("INFO:", m.Author.Username, "send 'stop'")
	if v == nil {
		log.Println("INFO: The bot is not joined in a voice channel")
		return
	}
	if v.ChannelID != SearchVoiceChannel(m.Author.ID) {
		return
	}
	v.StopTalking()
}

func RebootReporter(m *discordgo.MessageCreate) {
	commands := strings.Fields(m.Content)
	if len(commands) != 2 {
		return
	}
	if commands[1] != config.O().Discord.Secret {
		log.Printf("WARN: reboot DENIED for %s(%s)", m.Author.Username, m.Author.ID)
		return
	}
	log.Printf("INFO: reboot requested by %s(%s)", m.Author.Username, m.Author.ID)
	Shutdown()
	os.Exit(1)
}

func SpeechText(v *voice.VoiceInstance, m *discordgo.MessageCreate, botList map[string][]string) {
	content, err := m.Message.ContentWithMoreMentionsReplaced(v.Session)
	if err != nil {
		log.Println("ERROR: Convert Error.")
		return
	}
	content = reCustomEmoji.ReplaceAllString(content, "えもじ")
	content = reAnimEmoji.ReplaceAllString(content, "えもじ")
	content = reURL.ReplaceAllString(content, "URL")
	content = reSlashCommand.ReplaceAllString(content, "$1")

	play.ReplaceWords(v.GuildID, &content)

	user, err := global.DB.GetUser(m.Author.ID)
	if err != nil {
		log.Println("INFO: Cannot Get User info")
		user, err = global.DB.NewUser(m.Author.ID)
		if err != nil {
			log.Println("ERR: Cannot initialize User")
			return
		}
	}

	wavFileName := ""
	if wavs, ok := botList[m.Author.ID]; ok && len(wavs) != 0 {
		wavFileName = wavs[util.RandomInt(0, len(wavs))]
	}

	speech := voice.Speech{Text: content, UserInfo: user.UserInfo, WavFile: wavFileName}
	speechSig := voice.SpeechSignal{Data: speech, V: v}
	go func() {
		global.SpeechSignal <- speechSig
	}()
}
