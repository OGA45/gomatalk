package main

import (
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Options gomatalk option
type Options struct {
	DiscordToken    string
	DiscordStatus   string
	DiscordPrefix   string
	DiscordNumShard int
	DiscordShardID  int
	Debug           bool
}

// UserInfo user information for talk
type UserInfo struct {
	Voice     string
	Speed     float64
	Tone      float64
	Intone    float64
	Threshold float64
	AllPass   float64
	Volume    float64
}

type Speech struct {
	Text     string
	UserInfo UserInfo
	WavFile  string
}

type SpeechSignal struct {
	data Speech
	v    *VoiceInstance
}

type VoiceInstance struct {
	sync.Mutex
	voice      *discordgo.VoiceConnection
	session    *discordgo.Session
	queueMutex sync.Mutex
	voiceMutex sync.Mutex
	nowTalking Speech
	queue      []Speech
	recv       []int16
	guildID    string
	channelID  string
	speaking   bool
	stop       chan bool
}

type VoiceRoidConfig struct {
	baseURL string
	Voice   []VoiceRoid
}

type VoiceRoid struct {
	Name string
}

type VoicevoxConfig struct {
	baseURL string
	Voice   []VoiceVox
}

type VoiceVox struct {
	Name string
	Id   int
}

type AudioQuery struct {
	AccentPhrases   []AccentPhrase `json:"accent_phrases"`
	SpeedScale      float64        `json:"speedScale"`
	PitchScale      float64        `json:"pitchScale"`
	IntonationScale float64        `json:"intonationScale"`
}

type Mora struct {
	Text      string  `json:"text,omitempty"`
	Consonant string  `json:"consonant,omitempty"`
	Vowel     string  `json:"vowel,omitempty"`
	Pitch     float64 `json:"pitch"`
}

type AccentPhrase struct {
	Moras  []Mora `json:"moras"`
	Accent int    `json:"accent"`
}

type AquestalkConfig struct {
	ExePath string
	Voice   []Aquestalk
}

type Aquestalk struct {
	Name string
}
