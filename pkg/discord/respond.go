package discord

import (
	"fmt"
	"log"

	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/bwmarrin/discordgo"
)

const (
	colorInfo     = 0x0000FF
	colorSuccess  = 0x00FF00
	colorError    = 0xFF0000
	flagEphemeral = 1 << 6

	titleSuccess = ":white_check_mark: 成功"
	titleError   = ":warning: 失敗"

	msgGetInfoFailed = "**情報の取得に失敗しました。開発者にお問い合わせください。**"
	msgOutOfRange    = "**音声パラメータが範囲外です。イントネーションは AivisSpeech(@Aivis) と VOICEROID が 0〜2、それ以外は 0〜4 です（VOICEROID のトーンは 0.5〜2）。**"
)

// respondEmbed sends a single-embed interaction response, logging any failure.
func respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, ephemeral bool) {
	data := &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	}
	if ephemeral {
		data.Flags = flagEphemeral
	}
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	}); err != nil {
		log.Println("ERROR: InteractionRespond:", err)
	}
}

// respondError sends an ephemeral failure embed.
func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, description string) {
	respondEmbed(s, i, &discordgo.MessageEmbed{
		Title:       titleError,
		Description: description,
		Color:       colorError,
	}, true)
}

// respondSuccess sends a success embed.
func respondSuccess(s *discordgo.Session, i *discordgo.InteractionCreate, description string) {
	respondEmbed(s, i, &discordgo.MessageEmbed{
		Title:       titleSuccess,
		Description: description,
		Color:       colorSuccess,
	}, false)
}

// respondInfo sends an informational (blue) embed with a custom title.
func respondInfo(s *discordgo.Session, i *discordgo.InteractionCreate, title, description string) {
	respondEmbed(s, i, &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       colorInfo,
	}, false)
}

// respondInfoFields sends an informational (blue) embed carrying fields.
func respondInfoFields(s *discordgo.Session, i *discordgo.InteractionCreate, title string, fields []*discordgo.MessageEmbedField) {
	respondEmbed(s, i, &discordgo.MessageEmbed{
		Title:  title,
		Fields: fields,
		Color:  colorInfo,
	}, false)
}

// FormatUserInfo renders a user's voice settings in the canonical one-line form.
func FormatUserInfo(u model.UserInfo) string {
	return fmt.Sprintf("voice: %s, speed: %.1f, tone: %.1f, intone: %.1f, threshold: %.1f, allpass: %.1f, volume: %.1f",
		u.Voice, u.Speed, u.Tone, u.Intone, u.Threshold, u.AllPass, u.Volume)
}

// resolveBotName resolves an id to a display name. found is false when neither
// a user nor a webhook matches. isBot reports whether the id is a bot user;
// webhooks are treated as bots. All Dg lookups are nil-checked to avoid the
// nil-pointer panics that plagued the old copy-pasted resolution blocks.
func resolveBotName(id string) (name string, isBot bool, found bool) {
	if user, err := Dg.User(id); err == nil && user != nil {
		return user.Username, user.Bot, true
	}
	if webhook, err := Dg.Webhook(id); err == nil && webhook != nil {
		return webhook.Name, true, true
	}
	return "", false, false
}

// resolveUser resolves an id to a *discordgo.User (for avatar/embed use),
// falling back to the webhook owner or a synthetic user with the webhook name.
// All lookups are nil-checked.
func resolveUser(id string) (*discordgo.User, bool) {
	if u, err := Dg.User(id); err == nil && u != nil {
		return u, true
	}
	if wh, err := Dg.Webhook(id); err == nil && wh != nil {
		if wh.User != nil {
			return wh.User, true
		}
		return &discordgo.User{Username: wh.Name}, true
	}
	return nil, false
}
