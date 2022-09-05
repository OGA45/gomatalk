package discord

import (
	"fmt"
	"log"
	"strings"

	"github.com/OGA45/gomatalk/pkg/config"
	"github.com/OGA45/gomatalk/pkg/db"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

func Adding_slash_commands() {
	log.Println("INFO: Adding commands...")
	speedMinValue := 0.5
	twentyMinValue := -20.0
	ZeroMiinValue := 0.0
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: "コマンド一覧と簡単な説明を表示",
		},
		{
			Name:        "summon",
			Description: "読み上げを開始",
		},
		{
			Name:        "bye",
			Description: "読み上げを終了",
		},
		{
			Name:        "stop",
			Description: "読み上げを一時停止",
		},
		{
			Name:        "words_list",
			Description: "辞書一覧を表示",
		},
		{
			Name:        "add_word",
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
			Name:        "delete_word",
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
			Name:        "status",
			Description: "現在の声の設定を表示",
		},
		{
			Name:        "update_voice",
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
			Name:        "add_bot",
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
			Name:        "delete_bot",
			Description: "BOTを読み上げ対象から削除",
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
			Name:        "update_bot_voice",
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
			Name:        "random_bot",
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
			Name:        "reboot",
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
		{
			Name:        "voices_list",
			Description: "登録されている声の一覧を表示",
		},
	}
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := Dg.ApplicationCommandCreate(Dg.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}
}

func Help(s *discordgo.Session, i *discordgo.InteractionCreate) {
	log.Println("INFO:", i.Member.User, "send 'help'")
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       "コマンド一覧",
				Description: ":warning: **注意:旧コマンドは非推奨なので表記から消しています。**",
				Color:       0x0000FF,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "/help",
						Value: "```コマンド一覧と簡単な説明を表示する```",
					},
					{
						Name:  "/summon",
						Value: "```読み上げを開始する```",
					},
					{
						Name:  "/bye",
						Value: "```読み上げを終了する```",
					},
					{
						Name:  "/add_word",
						Value: "```単語を辞書に登録する```",
					},
					{
						Name:  "/delete_word",
						Value: "```単語を辞書から削除する```",
					},
					{
						Name:  "/words_list",
						Value: "```辞書一覧を表示する```",
					},
					{
						Name:  "/add_bot",
						Value: "```BOTを読み上げ対象に登録する```",
					},
					{
						Name:  "/delete_bot",
						Value: "```BOTを読み上げ対象から削除する```",
					},
					{
						Name:  "/bots_list",
						Value: "```読み上げ対象BOTの一覧を表示する```",
					},
					{
						Name:  "/random",
						Value: "```自分の声をﾗﾝﾀﾞﾑで変更する```",
					},
					{
						Name:  "/status",
						Value: "```自分の現在の声の設定を表示する```",
					},
					{
						Name:  "/update_voice",
						Value: "```自分の声の設定を変更する```",
					},
					{
						Name:  "/update_bot_voice",
						Value: "```BOTの声の設定を変更する```",
					},
					{
						Name:  "/random_bot",
						Value: "```BOTの声をﾗﾝﾀﾞﾑで変更する```",
					},
					{
						Name:  "/reboot",
						Value: "```BOTを再起動する\n当然他のサーバーにも影響が出るので注意```",
					},
					{
						Name:  "/stop",
						Value: "```読み上げを一時停止```",
					},
					{
						Name:  "/voices_list",
						Value: "```登録されている声の一覧を表示する```",
					},
				},
			}},
		},
	})
}

func Voices_list(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       "登録されている声の一覧",
				Description: "```" + strings.Join(voice.VoiceList(), "\n") + "```",
				Color:       0x0000FF,
			}},
		},
	})
}

func Summon(s *discordgo.Session, i *discordgo.InteractionCreate) {
	log.Println("INFO:", i.Member.User, "send 'join'")
	voiceChannelID := SearchVoiceChannel(i.Member.User.ID)
	guildID := i.GuildID
	if voiceChannelID == "" {
		log.Println("ERROR: Voice channel id not found.")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**貴方はVCに参加していません。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	//already := false
	v := global.VoiceInstances[guildID]
	if v != nil {
		log.Println("INFO: Creating bran new voice instance.")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**すでに参加しています。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	} else {
		log.Println("INFO: New Voice Instance created")
		guildID := guildID
		// create new voice instance
		global.Mutex.Lock()
		v = new(voice.VoiceInstance)
		global.VoiceInstances[guildID] = v
		v.GuildID = guildID
		v.Session = s
		v.Stop = make(chan bool, 1)
		global.Mutex.Unlock()
		//v.InitVoice()
	}
	var err error
	v.ChannelID = i.ChannelID
	v.Voice, err = Dg.ChannelVoiceJoin(v.GuildID, voiceChannelID, false, false)
	if err != nil {
		v.StopTalking()
		log.Println("ERROR: Error to join in a voice channel: ", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**VCに参加できませんでした。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	if config.O.Discord.Debug {
		v.Voice.LogLevel = discordgo.LogDebug
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: "**読み上げを開始します。**",
				Color:       0x00FF00,
			}},
		},
	})
}

func Bye(s *discordgo.Session, i *discordgo.InteractionCreate) {
	log.Println("INFO:", i.Member.User, "send 'leave'")
	v := global.VoiceInstances[i.GuildID]
	if v == nil {
		log.Println("INFO: The bot is not joined in a voice channel")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**VCに参加していません。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	closeConnection(v)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: "**終了します。**",
				Color:       0x00FF00,
			}},
		},
	})
}

func Stop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	log.Println("INFO:", i.Member.User, "send 'stop'")
	v := global.VoiceInstances[i.GuildID]
	if v == nil {
		log.Println("INFO: The bot is not joined in a voice channel")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**VCに参加していません。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	voiceChannelID := SearchVoiceChannel(i.Member.User.ID)
	if v.Voice.ChannelID != voiceChannelID {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**自分が参加していないVCの読み上げを止めることは出来ません。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	v.StopTalking()
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: "**読み上げを一時停止します。**",
				Color:       0x00FF00,
			}},
		},
	})
}

func Words_list(s *discordgo.Session, i *discordgo.InteractionCreate) {
	wordsList, err := global.DB.ListWords(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	var msg string
	for k, v := range wordsList {
		msg += fmt.Sprintf("・単語: %s、読み: %s\n", k, v)
	}
	if msg == "" {
		msg = "現在何も登録されていません。"
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       "登録されている単語一覧",
				Description: "```" + msg + "```",
				Color:       0x0000FF,
			}},
		},
	})
}

func Add_word(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var margs [2]string
	if option, ok := optionMap["登録する単語"]; ok {
		margs[0] = option.StringValue()
		log.Println("INFO:", option.StringValue())
	}
	if option, ok := optionMap["登録する読み"]; ok {
		margs[1] = option.StringValue()
		log.Println("INFO:", option.StringValue())
	}
	err := global.DB.AddWord(i.GuildID, margs[0], margs[1])
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**単語「%s」の登録に失敗しました。**", margs[0]),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: fmt.Sprintf("**単語「%s」を読み「%s」で登録しました。**", margs[0], margs[1]),
				Color:       0x00FF00,
			}},
		},
	})
}

func Delete_word(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var margs string
	var flag = true
	if option, ok := optionMap["削除する単語"]; ok {
		margs = option.StringValue()
		log.Println("INFO:", option.StringValue())
	}
	wordsList, err := global.DB.ListWords(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	for k, _ := range wordsList {
		if k == margs {
			flag = false
		}
	}
	if flag {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**単語「%s」は見つかりませんでした。**", margs),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	err = global.DB.DeleteWord(i.GuildID, margs)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**単語「%s」の削除に失敗しました**", margs),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: fmt.Sprintf("**単語「%s」を削除しました。**", margs),
				Color:       0x00FF00,
			}},
		},
	})
}

func Status(s *discordgo.Session, i *discordgo.InteractionCreate) {
	DBUser, err := global.DB.GetUser(i.Member.User.ID)
	if err != nil {
		log.Println("INFO: Cannot Get User info")
		DBUser, err = global.DB.NewUser(i.Member.User.ID)
		if err != nil {
			log.Println("ERR: Cannot initialize User")
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{{
						Title:       ":warning: 失敗",
						Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
						Color:       0xFF0000,
					}},
				},
			})
			return
		}
	}
	userInfo := DBUser.UserInfo
	msg := fmt.Sprintf("```voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f```",
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
			Embeds: []*discordgo.MessageEmbed{{
				Title:       "現在の音声情報",
				Description: msg,
				Color:       0x0000FF,
			}},
		},
	})
}

func Update_voice(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var voices string
	var margs [6]float64
	if option, ok := optionMap["声の種類"]; ok {
		voices = option.StringValue()
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
	_, ok := voice.Voices()[voices]
	if !ok {
		log.Println("Not find key", voices)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**「%s」というボイスが見つかりませんでした。**", voices),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	DBUser, err := global.DB.GetUser(i.Member.User.ID)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	userInfo := DBUser.UserInfo
	userInfo.Voice = voices
	userInfo.Speed = margs[0]
	userInfo.Tone = margs[1]
	userInfo.Intone = margs[2]
	userInfo.Threshold = margs[3]
	userInfo.AllPass = margs[4]
	userInfo.Volume = margs[5]
	global.DB.AddUser(i.Member.User.ID, userInfo)
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
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: "**以下の情報で登録しました。\n" + msg + "**",
				Color:       0x00FF00,
			}},
		},
	})
}

func Add_bot(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	botUser, err := Dg.User(bot_id)
	if err != nil {
		webHook, err := Dg.Webhook(bot_id)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{{
						Title:       ":warning: 失敗",
						Description: fmt.Sprintf("**ID「%s」のBOTは見つかりませんでした。**", bot_id),
						Color:       0xFF0000,
					}},
				},
			})
			return
		}
		username = webHook.Name
	} else {
		username = botUser.Username
	}
	err = global.DB.AddBot(i.GuildID, bot_id, wavList)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**BOT「%s」の登録に失敗しました。**", username),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: fmt.Sprintf("**BOT「%s」を読み上げ対象に登録しました。**", username),
				Color:       0x00FF00,
			}},
		},
	})
}

func Delete_bot(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var bot_id string
	if option, ok := optionMap["削除するbotのid"]; ok {
		bot_id = option.StringValue()
	}
	botList, err := global.DB.ListBots(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**読み上げ対象BOTの一覧の取得に失敗しました。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	flag := true
	for k, _ := range botList {
		if k == bot_id {
			flag = false
		}
	}
	if flag {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**BOT ID「%s」は見つかりませんでした。**", bot_id),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	err = global.DB.DeleteBot(i.GuildID, bot_id)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**BOT ID「%s」の削除に失敗しました。**", bot_id),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: fmt.Sprintf("**BOT ID「%s」を削除しました。**", bot_id),
				Color:       0x00FF00,
			}},
		},
	})
}

func Bots_list(s *discordgo.Session, i *discordgo.InteractionCreate) {
	botList, err := global.DB.ListBots(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**読み上げ対象BOTの一覧の取得に失敗しました。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	} else if len(botList) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{{
					Title:       "登録されているBOT一覧",
					Description: "**現在何も登録されていません。**",
					Color:       0x0000FF,
				}},
			},
		})
		return
	}
	fields := []*discordgo.MessageEmbedField{}
	for k, v := range botList {
		name := k
		if v[0] == "" {
			v[0] = "None"
		}
		botUser, err := Dg.User(k)
		if err == nil {
			name = botUser.Username
		} else {
			webhook, err := Dg.Webhook(k)
			if err == nil {
				name = webhook.Name
			}
		}
		DBUser, err := global.DB.GetUser(k)
		if err != nil {
			log.Println("INFO: Cannot Get User info")
			DBUser, err = global.DB.NewUser(k)
			if err != nil {
				log.Println("ERR: Cannot initialize User")
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags: 1 << 6,
						Embeds: []*discordgo.MessageEmbed{{
							Title:       ":warning: 失敗",
							Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
							Color:       0xFF0000,
						}},
					},
				})
				return
			}
		}
		userInfo := DBUser.UserInfo
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("%s(%s) WAV LIST: %s\n", name, k, strings.Join(v, ",")),
			Value: fmt.Sprintf("```%s, %.1f, %.1f, %.1f, %.1f, %.1f, %.1f```",
				userInfo.Voice,
				userInfo.Speed,
				userInfo.Tone,
				userInfo.Intone,
				userInfo.Threshold,
				userInfo.AllPass,
				userInfo.Volume),
			Inline: false,
		})
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:  "登録されているBOT一覧",
				Fields: fields,
				Color:  0x0000FF,
			}},
		},
	})
}

func Random(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := i.Member.User.ID
	user := db.MakeRandom()
	global.DB.AddUser(userID, user)
	DBUser, err := global.DB.GetUser(i.Member.User.ID)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	userInfo := DBUser.UserInfo
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
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: "**以下の情報で登録しました。\n" + msg + "**",
				Color:       0x00FF00,
			}},
		},
	})
}

func Update_bot_voice(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	var bot_id string
	var voices string
	var username string
	var margs [6]float64
	if option, ok := optionMap["変更するbotのid"]; ok {
		bot_id = option.StringValue()
	}
	if option, ok := optionMap["声の種類"]; ok {
		voices = option.StringValue()
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
	bot_user, err := Dg.User(bot_id)
	if err != nil {
		webHook, err := Dg.Webhook(bot_id)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{{
						Title:       ":warning: 失敗",
						Description: fmt.Sprintf("**ID「%s」のBOTは見つかりませんでした。**", bot_id),
						Color:       0xFF0000,
					}},
				},
			})
			return
		}
		username = webHook.Name
	} else {
		if !bot_user.Bot {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{{
						Title:       ":warning: 失敗",
						Description: "**声変えられるのはBotのみです。**",
						Color:       0xFF0000,
					}},
				},
			})
			return
		}
		username = bot_user.Username
	}
	botList, err := global.DB.ListBots(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**読み上げ対象BOTの一覧の取得に失敗しました。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	flag := true
	for k, _ := range botList {
		if k == bot_id {
			flag = false
		}
	}
	if flag {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**BOT ID「%s」は読み上げ登録されていません。**", bot_id),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	_, ok := voice.Voices()[voices]
	if !ok {
		log.Println("Not find key", voices)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**「%s」というボイスが見つかりませんでした。**", voices),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	DBUser, err := global.DB.GetUser(bot_id)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	userInfo := DBUser.UserInfo
	userInfo.Voice = voices
	userInfo.Speed = margs[0]
	userInfo.Tone = margs[1]
	userInfo.Intone = margs[2]
	userInfo.Threshold = margs[3]
	userInfo.AllPass = margs[4]
	userInfo.Volume = margs[5]
	global.DB.AddUser(bot_id, userInfo)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
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
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: fmt.Sprintf("**BOT「%s」の音声を以下の情報で登録しました。**\n", username) + msg,
				Color:       0x00FF00,
			}},
		},
	})
}

func Random_bot(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	bot_user, err := Dg.User(bot_id)
	if err != nil {
		webHook, err := Dg.Webhook(bot_id)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{{
						Title:       ":warning: 失敗",
						Description: fmt.Sprintf("**ID「%s」のBOTは見つかりませんでした。**", bot_id),
						Color:       0xFF0000,
					}},
				},
			})
			return
		}
		username = webHook.Name
	} else {
		if !bot_user.Bot {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{{
						Title:       ":warning: 失敗",
						Description: "**声変えられるのはBotのみです。**",
						Color:       0xFF0000,
					}},
				},
			})
			return
		}
		username = bot_user.Username
	}
	botList, err := global.DB.ListBots(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**情報の取得に失敗しました。開発者にお問い合わせください。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
	flag := true
	for k, _ := range botList {
		if k == bot_id {
			flag = false
		}
	}
	if flag {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: fmt.Sprintf("**BOT ID「%s」は見つかりませんでした。**", bot_id),
					Color:       0xFF0000,
				}},
			},
		})
		return
	}

	user := db.MakeRandom()
	global.DB.AddUser(bot_id, user)
	DBUser, err := global.DB.GetUser(bot_id)
	if err != nil {
		log.Println("ERROR: Cannot get user information.")
		return
	}
	userInfo := DBUser.UserInfo
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
			Embeds: []*discordgo.MessageEmbed{{
				Title:       ":white_check_mark: 成功",
				Description: fmt.Sprintf("**BOT「%s」の音声を以下の情報で登録しました。**\n", username) + msg,
				Color:       0x00FF00,
			}},
		},
	})
}

func Reboot(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	var secret string
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	if option, ok := optionMap["パスワード"]; ok {
		secret = option.StringValue()
	}
	if secret == config.O.Discord.Secret {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":white_check_mark: 成功",
					Description: "**再起動します。**",
					Color:       0x00FF00,
				}},
			},
		})
		panic("Rebooting")
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Embeds: []*discordgo.MessageEmbed{{
					Title:       ":warning: 失敗",
					Description: "**パスワードが違います。**",
					Color:       0xFF0000,
				}},
			},
		})
		return
	}
}
