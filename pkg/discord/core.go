package discord

import (
	"github.com/OGA45/gomatalk/pkg/db"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/OGA45/gomatalk/pkg/voice"
)

// validateUserInfo delegates to voice.ValidateUserInfo — the shared source of
// truth for voice existence and per-engine parameter ranges.
func validateUserInfo(ui model.UserInfo) error {
	return voice.ValidateUserInfo(ui)
}

// applyVoiceUpdate validates ui and persists it for targetID.
func applyVoiceUpdate(targetID string, ui model.UserInfo) error {
	if err := validateUserInfo(ui); err != nil {
		return err
	}
	return global.DB.AddUser(targetID, ui)
}

// applyRandom assigns a random voice setting to targetID and returns it.
func applyRandom(targetID string) (model.UserInfo, error) {
	ui := db.MakeRandom()
	if err := global.DB.AddUser(targetID, ui); err != nil {
		return ui, err
	}
	return ui, nil
}

// isRegisteredBot reports whether botID is registered for TTS in the guild.
func isRegisteredBot(guildID, botID string) (bool, error) {
	botList, err := global.DB.ListBots(guildID)
	if err != nil {
		return false, err
	}
	_, ok := botList[botID]
	return ok, nil
}
