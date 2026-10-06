package config

import (
	"errors"
	"log"
	"sync/atomic"

	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config sections are stored behind atomic pointers so that concurrent handler
// goroutines always observe a fully-built, consistent snapshot. A (re)load
// builds fresh structs into locals, validates them, and only then swaps the
// pointers — so a parse failure during hot-reload keeps the last good config
// instead of wiping it to empty.
var (
	optsPtr atomic.Pointer[model.Options]
	voPtr   atomic.Pointer[model.VoiceRoidConfig]
	vvPtr   atomic.Pointer[model.VoicevoxConfig]
	aqPtr   atomic.Pointer[model.AquestalkConfig]

	// generation increments on every successful (re)load. Other packages use
	// it to invalidate derived caches (e.g. the merged voice list).
	generation atomic.Uint64
)

func init() {
	optsPtr.Store(&model.Options{})
	voPtr.Store(&model.VoiceRoidConfig{})
	vvPtr.Store(&model.VoicevoxConfig{})
	aqPtr.Store(&model.AquestalkConfig{})
}

// O returns the current bot options (never nil).
func O() *model.Options { return optsPtr.Load() }

// Vo returns the current VOICEROID config (never nil).
func Vo() *model.VoiceRoidConfig { return voPtr.Load() }

// Vv returns the current VOICEVOX config (never nil).
func Vv() *model.VoicevoxConfig { return vvPtr.Load() }

// Aq returns the current AquesTalk config (never nil).
func Aq() *model.AquestalkConfig { return aqPtr.Load() }

// Generation returns a counter that increments on every successful load.
func Generation() uint64 { return generation.Load() }

// Load reads the config file once, parses every section into locals, validates
// the required Discord fields, and atomically swaps them in on success.
func Load(filename string) error {
	viper.SetConfigType("toml")
	viper.SetConfigFile(filename)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	o := &model.Options{}
	if err := viper.Unmarshal(o); err != nil {
		return errors.New("cannot load config")
	}
	if o.Discord.Token == "" {
		return errors.New("'token' must be present in config file")
	}
	if o.Discord.Status == "" {
		return errors.New("'status' must be present in config file")
	}
	if o.Discord.Prefix == "" {
		return errors.New("'prefix' must be present in config file")
	}
	if o.Activity.Enabled {
		if o.Activity.ClientID == "" {
			return errors.New("'activity.clientID' must be present when activity is enabled")
		}
		if o.Activity.ClientSecret == "" {
			return errors.New("'activity.clientSecret' must be present when activity is enabled")
		}
		if o.Activity.Listen == "" {
			o.Activity.Listen = ":8080"
		}
	}

	vo := &model.VoiceRoidConfig{}
	if err := viper.Unmarshal(vo); err != nil {
		return errors.New("cannot load voiceroid config")
	}
	vv := &model.VoicevoxConfig{}
	if err := viper.Unmarshal(vv); err != nil {
		return errors.New("cannot load voicevox config")
	}
	aq := &model.AquestalkConfig{}
	if err := viper.Unmarshal(aq); err != nil {
		return errors.New("cannot load aquestalk config")
	}

	// All sections parsed & validated — swap atomically, then bump generation.
	optsPtr.Store(o)
	voPtr.Store(vo)
	vvPtr.Store(vv)
	aqPtr.Store(aq)
	generation.Add(1)
	return nil
}

// Watch enables hot reloading of the config file.
func Watch() {
	viper.WatchConfig()
	viper.OnConfigChange(reload)
}

func reload(e fsnotify.Event) {
	log.Println("INFO: The config file changed:", e.Name)
	if err := Load(e.Name); err != nil {
		log.Println("ERROR: config reload failed, keeping previous config:", err)
	}
}
