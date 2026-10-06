package model

// Options gomatalk option
type Options struct {
	Discord struct {
		Token    string `mapstructure:"token"`
		Status   string `mapstructure:"status"`
		Prefix   string `mapstructure:"prefix"`
		NumShard int    `mapstructure:"shardCount"`
		ShardID  int    `mapstructure:"shardID"`
		Debug    bool   `mapstructure:"debug"`
		Secret   string `mapstructure:"secret"`
	} `mapstructure:"discord"`
	Greeting map[string]string `mapstructure:"greeting"`
	ErrorMsg map[string]string `mapstructure:"errorMsg"`
	Activity struct {
		Enabled      bool   `mapstructure:"enabled"`
		Listen       string `mapstructure:"listen"`
		ClientID     string `mapstructure:"clientID"`
		ClientSecret string `mapstructure:"clientSecret"`
	} `mapstructure:"activity"`
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

type VoiceRoidConfig struct {
	Voiceroid struct {
		BaseURL string      `mapstructure:"baseURL"`
		Voice   []VoiceRoid `mapstructure:"voice"`
	} `mapstructure:"voiceroid"`
}

type VoiceRoid struct {
	Name string
}

// VoicevoxConfig holds the VOICEVOX-compatible engines ([voicevox] and
// [aivisspeech] sections).
type VoicevoxConfig struct {
	Voicevox struct {
		BaseURL     string     `mapstructure:"baseURL"`
		FallbackURL string     `mapstructure:"fallbackURL"` // optional; used when baseURL fails
		Voice       []VoiceVox `mapstructure:"voice"`
	} `mapstructure:"voicevox"`
	AivisSpeech struct {
		BaseURL     string `mapstructure:"baseURL"`     // empty = AivisSpeech voices disabled
		FallbackURL string `mapstructure:"fallbackURL"` // optional
	} `mapstructure:"aivisspeech"`
}

type VoiceVox struct {
	Name string
	Id   int
}

type AquestalkConfig struct {
	Aquestalk struct {
		ExePath string      `mapstructure:"exePath"`
		Voice   []Aquestalk `mapstructure:"voice"`
	} `mapstructure:"aquestalk"`
}

type Aquestalk struct {
	Name string
}
