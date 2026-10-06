package global

import (
	"sync"

	"github.com/OGA45/gomatalk/pkg/db"
	"github.com/OGA45/gomatalk/pkg/voice"
)

var (
	// instances maps guildID -> active voice instance. It is ONLY accessed
	// through the locked accessors below; discordgo dispatches every gateway
	// event in its own goroutine, so a bare map access here would race and
	// trigger a fatal "concurrent map read and map write" crash.
	instances   = map[string]*voice.VoiceInstance{}
	instancesMu sync.RWMutex

	SpeechSignal chan voice.SpeechSignal
	DB           *db.Database
)

// GetInstance returns the voice instance for guildID, or nil if none exists.
func GetInstance(guildID string) *voice.VoiceInstance {
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	return instances[guildID]
}

// SetInstance stores v as the voice instance for guildID.
func SetInstance(guildID string, v *voice.VoiceInstance) {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	instances[guildID] = v
}

// DeleteInstance removes the voice instance for guildID.
func DeleteInstance(guildID string) {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	delete(instances, guildID)
}

// CreateInstanceIfAbsent atomically returns the existing instance for guildID
// (created=false), or builds one via newFn, stores it and returns it
// (created=true). This closes the check-then-act race in the join handlers.
func CreateInstanceIfAbsent(guildID string, newFn func() *voice.VoiceInstance) (v *voice.VoiceInstance, created bool) {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	if existing, ok := instances[guildID]; ok {
		return existing, false
	}
	v = newFn()
	instances[guildID] = v
	return v, true
}

// CloseAllInstances disconnects every active voice instance and clears the map.
// Used during graceful shutdown.
func CloseAllInstances() {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	for id, v := range instances {
		v.Close()
		delete(instances, id)
	}
}
