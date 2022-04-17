package main

import (
	"fmt"
	"log"
	"strings"
	"time"
	"github.com/bwmarrin/discordgo"
)
type ApplicationCommandOptionChoice struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// DiscordConnect make a new connection to Discord
func DiscordConnect() (err error) {
	dg, err = discordgo.New("Bot " + o.DiscordToken)
	if err != nil {
		log.Println("FATA: error creating Discord session,", err)
		return
	}
	speedMinValue := 0.5
	twentyMinValue :=-20.0
	ZeroMiinValue := 0.0
	commands := []*discordgo.ApplicationCommand{
		{
			Name: "help",
			Description: "コマンド一覧と簡単な説明を表示",
		},
		{
			Name: "summon",
			Description: "読み上げを開始",
		},
		{
			Name: "bye",
			Description: "読み上げを終了",
		},
		{
			Name: "stop",
			Description: "読み上げを一時停止",
		},
		{
			Name: "words_list",
			Description: "辞書一覧を表示",
		},
		{
			Name: "add_word",
			Description: "辞書登録",
			Options: []*discordgo.ApplicationCommandOption{

				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "登録する単語",
					Description: "Set-word",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "登録する読み",
					Description: "Set-reading",
					Required:    true,
				},
			},
		},
		{
			Name: "delete_word",
			Description: "辞書削除",
			Options: []*discordgo.ApplicationCommandOption{

				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "削除する単語",
					Description: "Set-word",
					Required:    true,
				},
			},
		},
		{
			Name: "status",
			Description: "現在の声の設定を表示",
		},
		{
			Name: "update_voice",
			Description: "声の設定を変更",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "声の種類",
					Description: "voice",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "話す速度",
					Description: "speed",
					MaxValue:    2.0,
					MinValue:    &speedMinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "声のトーン",
					Description: "tone",
					MaxValue:    20.0,
					MinValue:    &twentyMinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "声のイントネーション",
					Description: "intone",
					MaxValue:    4.0,
					MinValue:    &ZeroMiinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "閾値",
					Description: "threshold",
					MaxValue:    1.0,
					MinValue:    &ZeroMiinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "オールパス",
					Description: "allpass",
					MaxValue:    1.0,
					MinValue:    &ZeroMiinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "音量",
					Description: "volume",
					MaxValue:    20.0,
					MinValue:    &twentyMinValue,
					Required:    true,
				},
			},
		},
		{
			Name: "add_bot",
			Description: "BOTを読み上げ対象に登録",
			Options: []*discordgo.ApplicationCommandOption{

				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "登録するbotのid",
					Description: "Bot-id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "登録するwav-list",
					Description: "Wav-list",
				},
			},
		},
		{
			Name: "delete_bot",
			Description: "BOTを読み上げ対象に登録",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "削除するbotのid",
					Description: "Bot-id",
					Required:    true,
				},
			},
		},
		{
			Name:        "bots_list",
			Description: "読み上げ対象BOTの一覧を表示",
		},
		{
			Name:        "random",
			Description: "自分の声をﾗﾝﾀﾞﾑで変更する",
		},
		{
			Name: "update_bot_voice",
			Description: "BOTの音声を変更",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "変更するbotのid",
					Description: "Bot-id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "声の種類",
					Description: "voice",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "話す速度",
					Description: "speed",
					MaxValue:    2.0,
					MinValue:    &speedMinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "声のトーン",
					Description: "tone",
					MaxValue:    20.0,
					MinValue:    &twentyMinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "声のイントネーション",
					Description: "intone",
					MaxValue:    4.0,
					MinValue:    &ZeroMiinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "閾値",
					Description: "threshold",
					MaxValue:    1.0,
					MinValue:    &ZeroMiinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "オールパス",
					Description: "allpass",
					MaxValue:    1.0,
					MinValue:    &ZeroMiinValue,
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "音量",
					Description: "volume",
					MaxValue:    20.0,
					MinValue:    &twentyMinValue,
					Required:    true,
				},
			},
		},
		{
			Name: "random_bot",
			Description: "BOTの声をﾗﾝﾀﾞﾑで変更する",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "変更するbotのid",
					Description: "Bot-id",
					Required:    true,
				},
			},
		},
		{
			Name: "reboot",
			Description: "BOTを再起動する",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "パスワード",
					Description: "Secret-word",
					Required:    true,
				},
			},
		},

	}
	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"help": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Println("INFO:", i.Member.User, "send 'help'")
			help := "コマンド一覧\n" +
				o.DiscordPrefix + "help or " + o.DiscordPrefix + "h  ->  コマンド一覧と簡単な説明を表示.\n" +
				o.DiscordPrefix + "summon or " + o.DiscordPrefix + "s  ->  読み上げを開始.\n" +
				o.DiscordPrefix + "bye or " + o.DiscordPrefix + "b  ->  読み上げを終了.\n" +
				o.DiscordPrefix + "add_word or " + o.DiscordPrefix + "aw  ->  辞書登録. (" + o.DiscordPrefix + "aw 単語 読み" + ")\n" +
				o.DiscordPrefix + "delete_word or " + o.DiscordPrefix + "dw  ->  辞書削除. (" + o.DiscordPrefix + "dw 単語" + ")\n" +
				o.DiscordPrefix + "words_list or " + o.DiscordPrefix + "wl  ->  辞書一覧を表示.\n" +
				o.DiscordPrefix + "add_bot or " + o.DiscordPrefix + "ab  ->  BOTを読み上げ対象に登録. (" + o.DiscordPrefix + "ab <BOT ID> <WAV LIST>" + ")\n" +
				o.DiscordPrefix + "delete_bot or " + o.DiscordPrefix + "db  ->  BOTを読み上げ対象から削除. (" + o.DiscordPrefix + "db <BOT ID>" + ")\n" +
				o.DiscordPrefix + "bots_list or " + o.DiscordPrefix + "bl  ->  読み上げ対象BOTの一覧を表示.\n" +
				o.DiscordPrefix + "random or " + o.DiscordPrefix + "r  ->  自分の声をﾗﾝﾀﾞﾑで変更する.\n" +
				o.DiscordPrefix + "status ->  現在の声の設定を表示.\n" +
				o.DiscordPrefix + "update_voice or " + o.DiscordPrefix + "uv  ->  声の設定を変更. (" + o.DiscordPrefix + "uv voice speed tone intone threshold volume" + ")\n" +
				"   voice: 声の種類 - " + strings.Join(VoiceList(), "\n                  - ") + "\n" +
				"   speed: 話す速度 範囲(0.5~2.0) \n" +
				"   tone : 声のトーン 範囲(-20~20) [VOICEROIDは 0.5 ~ 2] \n" +
				"   intone : 声のイントネーション 範囲(0.0~4.0)(初期値 1.0) [VOICEROIDは 0 ~ 2] \n" +
				"   threshold : ブツブツするときとか改善するかも?? 範囲(0.0~1.0)(初期値 0.5) \n" +
				"   allpass : よくわからん 範囲(0 - 1.0) (0はauto)  \n" +
				"   volume : 音量（dB） 範囲(-20~20)(初期値 1) \n" +
				o.DiscordPrefix + "update_bot_voice -> BOTの音声を変更.(基本update_voiceと同じ)\n"+
				o.DiscordPrefix + "random_bot -> BOTの声をﾗﾝﾀﾞﾑで変更する.(基本randomと同じ)\n"+
				o.DiscordPrefix + "reboot -> BOTを再起動します.当然他のサーバーにも影響が出るので注意.\n"+
				o.DiscordPrefix + "stop  ->  読み上げを一時停止."
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "旧コマンドを残していますが、基本的にはスラッシュコマンドを使用してください。今後旧コマンドの改修は行いません。",
					Files: []*discordgo.File{
						{
							ContentType: "text/plain",
							Name:        "Help Menu.txt",
							Reader:      strings.NewReader(help),
						},
					},
				},
			})
		},
		"summon": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Println("INFO:", i.Member.User, "send 'join'")
			voiceChannelID := SearchVoiceChannel(i.Member.User.ID)
			guildID :=i.GuildID
			if voiceChannelID == "" {
				log.Println("ERROR: Voice channel id not found.")
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "<@"+i.Member.User.ID+"> VCに参加者がいません。",
					},
				})
				return
			}
			v := voiceInstances[guildID]
			if v != nil {
				log.Println("INFO: Creating bran new voice instance.")
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "すでに参加しています。",
					},
				})
				return
			}
			mutex.Lock()
			v = new(VoiceInstance)
			voiceInstances[guildID] = v
			v.guildID = guildID
			v.session = s
			v.stop = make(chan bool, 1)
			mutex.Unlock()
			var err error
			v.channelID = i.ChannelID
			v.voice, err = dg.ChannelVoiceJoin(v.guildID, voiceChannelID, false, false)
			if err != nil {
				v.Stop()
				log.Println("ERROR: Error to join in a voice channel: ", err)
				return
			}
			if o.Debug {
				v.voice.LogLevel = discordgo.LogDebug
			}
			log.Println("INFO: New Voice Instance created")
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "読み上げを開始します。",
				},
			})
		},
		"bye": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Println("INFO:", i.Member.User, "send 'leave'")
			guildID :=i.GuildID
			v := voiceInstances[guildID]
			if v == nil {
				log.Println("INFO: The bot is not joined in a voice channel")
				return
			}
			closeConnection(v)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "終了します。",
				},
			})
		},
		"stop": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Println("INFO:", i.Member.User, "send 'stop'")
			guildID :=i.GuildID
			v := voiceInstances[guildID]
			if v == nil {
				log.Println("INFO: The bot is not joined in a voice channel")
				return
			}
			voiceChannelID := SearchVoiceChannel(i.Member.User.ID)
			if v.voice.ChannelID != voiceChannelID {
				return
			}
			v.Stop()
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "読み上げを一時停止します",
				},
			})
		},
		"words_list": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			wordsList, err := ListWords(i.GuildID)
			if err != nil {
				return
			}
			msg := "```\n登録されている単語一覧\n\n"
			for k, v := range wordsList {
				msg += fmt.Sprintf("・単語: %s、読み: %s\n", k, v)
			}
			msg += "```"
			//closeConnection(v)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
				},
			})
		},
		"add_word": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// Access options in the order provided by the user.
			options := i.ApplicationCommandData().Options
			// Or convert the slice into a map
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var margs[2] string
			// Get the value from the option map.
			// When the option exists, ok = true
			if option, ok := optionMap["登録する単語"]; ok {
				margs[0] = option.StringValue()
				log.Println("INFO:", option.StringValue())
			}
			if option, ok := optionMap["登録する読み"]; ok {
				margs[1] = option.StringValue()
				log.Println("INFO:", option.StringValue())
			}
			err := AddWord(i.GuildID, margs[0], margs[1])
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("単語「%s」の登録に失敗しました", margs[0]),
					},
				})
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("単語「%s」を読み「%s」で登録しました", margs[0], margs[1]),
				},
			})
		},
		"delete_word": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var margs[1] string
			if option, ok := optionMap["削除する単語"]; ok {
				margs[0] = option.StringValue()
				log.Println("INFO:", option.StringValue())
			}
			err := DeleteWord(i.GuildID, margs[0])
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("単語「%s」の削除に失敗しました", margs[0]),
					},
				})
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("単語「%s」を削除しました", margs[0]),
				},
			})
		},
		"status": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			userInfo, err := GetUserInfo(i.Member.User.ID)
			if err != nil {
				log.Println("ERROR: Cannot get user information.")
				return
			}
			msg := fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
				},
			})
		},
		"update_voice": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var voice string
			var margs[6] float64
			// Get the value from the option map.
			// When the option exists, ok = true
			if option, ok := optionMap["声の種類"]; ok {
				voice = option.StringValue()
			}
			if option, ok := optionMap["話す速度"]; ok {
				margs[0] = option.FloatValue()
			}
			if option, ok := optionMap["声のトーン"]; ok {
				margs[1] = option.FloatValue()
			}
			if option, ok := optionMap["声のイントネーション"]; ok {
				margs[2] = option.FloatValue()
			}
			if option, ok := optionMap["閾値"]; ok {
				margs[3] = option.FloatValue()
			}
			if option, ok := optionMap["オールパス"]; ok {
				margs[4] = option.FloatValue()
			}
			if option, ok := optionMap["音量"]; ok {
				margs[5] = option.FloatValue()
			}
			_, ok := Voices()[voice]
			if !ok {
				log.Println("Not find key",voice)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("「%s」というボイスが見つかりませんでした", voice),
					},
				})
				return
			}
			userInfo := UserInfo{}
			userInfo.Voice = voice
			userInfo.Speed = margs[0]
			userInfo.Tone = margs[1]
			userInfo.Intone = margs[2]
			userInfo.Threshold = margs[3]
			userInfo.AllPass = margs[4]
			userInfo.Volume = margs[5]
			PutUser(i.Member.User.ID, userInfo)

			userInfo, err := GetUserInfo(i.Member.User.ID)
			if err != nil {
				log.Println("ERROR: Cannot get user information.")
				return
			}
			msg := fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "以下の情報で登録しました\n"+msg,
				},
			})
		},
		"add_bot": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var bot_id string
			var username string
			wavList := []string{}
			if option, ok := optionMap["登録するbotのid"]; ok {
				bot_id = option.StringValue()
			}
			if option, ok := optionMap["登録するwav-list"]; ok {
				wavList = strings.Split(option.StringValue(), ",")
			}
			botUser, err := dg.User(bot_id)
			if err != nil {
				webHook, err := dg.Webhook(bot_id)
				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Flags:   1 << 6,
							Content:  fmt.Sprintf("ID「%s」のBOTは見つかりませんでした。", bot_id),
						},
					})
					return
				}
				username = webHook.Name
			} else {
				username = botUser.Username
			}
			err = Addbot(i.GuildID, bot_id, wavList)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("BOT「%s」の登録に失敗しました。", username),
					},
				})
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("BOT「%s」を読み上げ対象に登録しました。", username),
				},
			})
		},
		"delete_bot": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var bot_id string
			if option, ok := optionMap["削除するbotのid"]; ok {
				bot_id = option.StringValue()
			}
			botList, err := ListBots(i.GuildID)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "読み上げ対象BOTの一覧の取得に失敗しました。",
					},
				})
				return
			}
			flag :=true
			for k, _ := range botList {
				if k==bot_id{
					flag=false
				}
			}
			if flag{
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("BOT ID「%s」は見つかりませんでした。", bot_id),
					},
				})
				return
			}
			err = DeleteBot(i.GuildID, bot_id)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("BOT ID「%s」の削除に失敗しました", bot_id),
					},
				})
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("BOT ID「%s」を削除しました", bot_id),
				},
			})
		},
		"bots_list": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			botList, err := ListBots(i.GuildID)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "読み上げ対象BOTの一覧の取得に失敗しました。",
					},
				})
				return
			}

			msg := "```\n登録されているBOT一覧\n\n"
			for k, v := range botList {
				name := k
				botUser, err := dg.User(k)
				if err == nil {
					name = botUser.Username
				} else {
					webhook, err := dg.Webhook(k)
					if err == nil {
						name = webhook.Name
					}
				}
				userInfo, err := GetUserInfo(k)
				msg += fmt.Sprintf("・BOT: %s(%s)、WAV LIST: %s\n", name, k, strings.Join(v, ","))
				msg += fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f\n",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume)
			}
			msg += "```"
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
				},
			})
		},
		"random": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			userID:=i.Member.User.ID
			user := MakeRandom()
			PutUser(userID, user)
			userInfo, err := GetUserInfo(i.Member.User.ID)
			if err != nil {
				log.Println("ERROR: Cannot get user information.")
				return
			}
			msg := fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "以下の情報で登録しました\n"+msg,
				},
			})
		},
		"update_bot_voice": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			var bot_id string
			var voice string
			var username string
			var margs[6] float64
			// Get the value from the option map.
			// When the option exists, ok = true
			if option, ok := optionMap["変更するbotのid"]; ok {
				bot_id = option.StringValue()
			}
			if option, ok := optionMap["声の種類"]; ok {
				voice = option.StringValue()
			}
			if option, ok := optionMap["話す速度"]; ok {
				margs[0] = option.FloatValue()
			}
			if option, ok := optionMap["声のトーン"]; ok {
				margs[1] = option.FloatValue()
			}
			if option, ok := optionMap["声のイントネーション"]; ok {
				margs[2] = option.FloatValue()
			}
			if option, ok := optionMap["閾値"]; ok {
				margs[3] = option.FloatValue()
			}
			if option, ok := optionMap["オールパス"]; ok {
				margs[4] = option.FloatValue()
			}
			if option, ok := optionMap["音量"]; ok {
				margs[5] = option.FloatValue()
			}

			bot_user, err := dg.User(bot_id)
			if err != nil {
				webHook, err := dg.Webhook(bot_id)
				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Flags:   1 << 6,
							Content:  fmt.Sprintf("ID「%s」のBOTは見つかりませんでした。", bot_id),
						},
					})
					return
				}
				username=webHook.Name
			} else {
				if !bot_user.Bot {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Flags:   1 << 6,
							Content: "声変えられるのはBotのみです。",
						},
					})
					return
				}
				username=bot_user.Username
			}

			botList, err := ListBots(i.GuildID)
			flag :=true
			for k, _ := range botList {
				if k==bot_id{
					flag=false
				}
			}
			if flag{
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("BOT ID「%s」は見つかりませんでした。", bot_id),
					},
				})
				return
			}

			_, ok := Voices()[voice]
			if !ok {
				log.Println("Not find key",voice)
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("「%s」というボイスが見つかりませんでした", voice),
					},
				})
				return
			}
			userInfo := UserInfo{}
			userInfo.Voice = voice
			userInfo.Speed = margs[0]
			userInfo.Tone = margs[1]
			userInfo.Intone = margs[2]
			userInfo.Threshold = margs[3]
			userInfo.AllPass = margs[4]
			userInfo.Volume = margs[5]
			PutUser(bot_id, userInfo)

			userInfo, err = GetUserInfo(bot_id)
			if err != nil {
				log.Println("ERROR: Cannot get user information.")
				return
			}
			msg := fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("BOT「%s」の音声を以下の情報で登録しました。\n", username)+msg,
				},
			})
		},
		"random_bot": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			var bot_id string
			var username string
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			if option, ok := optionMap["変更するbotのid"]; ok {
				bot_id = option.StringValue()
			}
			bot_user, err := dg.User(bot_id)
			if err != nil {
				webHook, err := dg.Webhook(bot_id)
				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Flags:   1 << 6,
							Content:  fmt.Sprintf("ID「%s」のBOTは見つかりませんでした。", bot_id),
						},
					})
					return
				}
				username=webHook.Name
			} else {
				if !bot_user.Bot {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Flags:   1 << 6,
							Content: "声変えられるのはBotのみです。",
						},
					})
					return
				}
				username=bot_user.Username
			}
			botList, err := ListBots(i.GuildID)
			flag :=true
			for k, _ := range botList {
				if k==bot_id{
					flag=false
				}
			}
			if flag{
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("BOT ID「%s」は見つかりませんでした。", bot_id),
					},
				})
				return
			}

			user := MakeRandom()
			PutUser(bot_id, user)

			userInfo, err := GetUserInfo(bot_id)
			if err != nil {
				log.Println("ERROR: Cannot get user information.")
				return
			}
			msg := fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("BOT「%s」の音声を以下の情報で登録しました。\n", username)+msg,
				},
			})
		},
		"reboot": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			var secret string
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}
			if option, ok := optionMap["パスワード"]; ok {
				secret = option.StringValue()
			}
			if secret == o.Secret {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("再起動します"),
					},
				})
				panic("Rebooting")
			}else{
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content:  fmt.Sprintf("パスワードが違います"),
					},
				})
				return
			}
		},
	}
	log.Println("INFO: Bot is Opening")
	dg.AddHandler(MessageCreateHandler)
	dg.AddHandler(GuildCreateHandler)
	// dg.AddHandler(GuildDeleteHandler)
	dg.AddHandler(VoiceStatusUpdateHandler)
	dg.AddHandler(ConnectHandler)
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	if o.DiscordNumShard > 1 {
		dg.ShardCount = o.DiscordNumShard
		dg.ShardID = o.DiscordShardID
	}

	if o.Debug {
		dg.LogLevel = discordgo.LogDebug
	}
	// Open Websocket
	err = dg.Open()
	if err != nil {
		log.Println("FATA: Error Open():", err)
		return
	}
	_, err = dg.User("@me")
	if err != nil {
		// Login unsuccessful
		log.Println("FATA:", err)
		return
	} // Login successful
	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}
	log.Println("INFO: Bot is now running. Press CTRL-C to exit.")
	initRoutine()
	dg.UpdateGameStatus(0, o.DiscordStatus)
	return nil
}

// SearchVoiceChannel search the voice channel id into from guild.
func SearchVoiceChannel(user string) (voiceChannelID string) {
	for _, g := range dg.State.Guilds {
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
	for _, g := range dg.State.Guilds {
		for _, v := range g.VoiceStates {
			user, _ := dg.User(v.UserID)
			if !user.Bot {
				if v.ChannelID == voiceChannel {
					count++
				}
			}
		}
	}
	return count
}

// SearchGuild search the guild ID
func SearchGuild(textChannelID string) (guildID string) {
	channel, _ := dg.Channel(textChannelID)
	guildID = channel.GuildID
	return
}

// ChMessageSend send a message and auto-remove it in a time
func ChMessageSend(textChannelID, message string) {
	for i := 0; i < 10; i++ {
		_, err := dg.ChannelMessageSend(textChannelID, message)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
}

func ChFileSend(textChannelID, name, message string) {
	dg.ChannelFileSend(textChannelID, name, strings.NewReader(message))
}

// ChMessageSendEmbed send an embeded messages.
func ChMessageSendEmbed(textChannelID, title, description string, user discordgo.User) {
	embed := discordgo.MessageEmbed{}
	embed.Title = title
	embed.Description = description
	embed.Color = 0xb20000
	author := discordgo.MessageEmbedAuthor{}
	author.Name = user.Username
	author.IconURL = user.AvatarURL("")
	embed.Author = &author
	for i := 0; i < 10; i++ {
		_, err := dg.ChannelMessageSendEmbed(textChannelID, &embed)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
}

func initRoutine() {
	speechSignal = make(chan SpeechSignal)
	go GlobalPlay(speechSignal)
}

// ConnectHandler
func ConnectHandler(s *discordgo.Session, connect *discordgo.Connect) {
	s.UpdateGameStatus(0, o.DiscordStatus)
}

// GuildCreateHandler
func GuildCreateHandler(s *discordgo.Session, guild *discordgo.GuildCreate) {
	log.Println("INFO: Guild Create:", guild.ID)
	err := CreateGuildDB(guild.ID)
	if err != nil {
		log.Println("FATA: DB", err)
		return
	}
}

func VoiceStatusUpdateHandler(s *discordgo.Session, voice *discordgo.VoiceStateUpdate) {
	v := voiceInstances[voice.GuildID]
	if v == nil {
		return
	}
	if v.voice == nil {
		return
	}
	user, _ := dg.User(voice.UserID)
	botUser, _ := dg.User("@me")

	if voice.UserID == botUser.ID {
		if voice == nil || voice.BeforeUpdate == nil || voice.ChannelID == "" {
			return
		}
		if voice.BeforeUpdate.ChannelID != voice.ChannelID {
			v.voice, _ = dg.ChannelVoiceJoin(v.guildID, voice.ChannelID, false, false)
		}
	}

	if user.Bot && voice.UserID != botUser.ID {
		// Ignore Bot
		return
	}

	userCount := UserCountVoiceChannel(v.voice.ChannelID)
	if userCount == 0 {
		v.Lock()
		defer v.Unlock()
		if v.voice == nil {
			log.Println("INFO: Voice channel has already been destroyed")
			return
		}
		if v.session.VoiceConnections[v.guildID] != nil {
			v.voice.Disconnect()
			log.Println("INFO: Voice channel destroyed")
			mutex.Lock()
			delete(voiceInstances, v.guildID)
			mutex.Unlock()
			ChMessageSend(v.channelID, "終了します")
		}
	}
}

// MessageCreateHandler
func MessageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	guildID := SearchGuild(m.ChannelID)
	botList, _ := ListBots(guildID)
	isSpecial := false
	if m.Author.Bot {
		if _, ok := botList[m.Author.ID]; !ok {
			return
		}
		isSpecial = true
	}
	v := voiceInstances[guildID]
	if strings.HasPrefix(m.Content, o.DiscordPrefix) {
		content := strings.Replace(m.Content, o.DiscordPrefix, "", 1)
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
	if v != nil {
		if !isSpecial && v.channelID != m.ChannelID {
			return
		}
		SpeechText(v, m)
	}
}