package global

import (
	"sync"

	"github.com/OGA45/gomatalk/pkg/db"
	"github.com/OGA45/gomatalk/pkg/voice"
)

var (
	VoiceInstances = map[string]*voice.VoiceInstance{}
	Mutex          sync.Mutex
	SpeechSignal   chan voice.SpeechSignal
	DB             *db.Database
	// globalMutex sync.Mutex
	// songSignal     chan PkgSong
	// radioSignal    chan PkgRadio
	//ignore            = map[string]bool{}
)
