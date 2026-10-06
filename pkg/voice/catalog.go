package voice

import (
	"errors"
	"sort"

	"github.com/OGA45/gomatalk/pkg/model"
)

// ErrVoiceNotFound is returned by ValidateUserInfo when the requested voice
// does not exist in the merged voice set.
var ErrVoiceNotFound = errors.New("voice not found")

// ErrOutOfRange is returned by ValidateUserInfo when a parameter is outside
// the selected engine's range.
var ErrOutOfRange = errors.New("parameter out of range")

// VoiceEntry is one selectable voice tagged with the engine that synthesizes it.
type VoiceEntry struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
}

// Engine identifiers used by both the API catalog and the frontend.
const (
	EngineOpenJTalk   = "openjtalk"
	EngineLocal       = "local"
	EngineVoicevox    = "voicevox"
	EngineAivisSpeech = "aivisspeech"
	EngineVoiceRoid   = "voiceroid"
	EngineAquesTalk   = "aquestalk"
)

// VoiceCatalog returns every available voice grouped by engine. Engines are
// emitted in a stable display order (openjtalk, local, voicevox, aivisspeech,
// voiceroid, aquestalk) and names sort ascending within each engine. A name is emitted
// once (first engine wins) so the frontend <select> never carries duplicate
// option values — voice names are engine-distinct in practice, so collisions
// only arise from deliberate misconfiguration.
func VoiceCatalog() []VoiceEntry {
	sources := []struct {
		engine string
		names  []string
	}{
		{EngineOpenJTalk, keys(builtinVoices)},
		{EngineLocal, keys(LocalVoiceList())},
		{EngineVoicevox, keys(VoicevoxList())},
		{EngineAivisSpeech, keys(AivisSpeechList())},
		{EngineVoiceRoid, keys(VoiceRoidList())},
		{EngineAquesTalk, keys(AquestalkList())},
	}

	seen := map[string]bool{}
	entries := []VoiceEntry{}
	for _, src := range sources {
		sort.Strings(src.names)
		for _, name := range src.names {
			if seen[name] {
				continue
			}
			seen[name] = true
			entries = append(entries, VoiceEntry{Name: name, Engine: src.engine})
		}
	}
	return entries
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ValidateUserInfo enforces voice existence and per-engine parameter ranges.
// It is the single source of truth shared by the prefix, slash and activity
// transports (moved here from pkg/discord to break the dependency edge).
func ValidateUserInfo(ui model.UserInfo) error {
	if _, ok := Voices()[ui.Voice]; !ok {
		return ErrVoiceNotFound
	}
	if err := checkRange(ui.Speed, 0.5, 2.0); err != nil {
		return err
	}
	if IsVoiceRoid(ui.Voice) {
		if err := checkRange(ui.Tone, 0.5, 2); err != nil {
			return err
		}
		if err := checkRange(ui.Intone, 0, 2); err != nil {
			return err
		}
	} else if IsAivisSpeech(ui.Voice) {
		// AivisSpeech の intonationScale は感情表現の強さで 0〜2。
		if err := checkRange(ui.Tone, -20, 20); err != nil {
			return err
		}
		if err := checkRange(ui.Intone, 0, 2); err != nil {
			return err
		}
	} else {
		if err := checkRange(ui.Tone, -20, 20); err != nil {
			return err
		}
		if err := checkRange(ui.Intone, 0, 4); err != nil {
			return err
		}
	}
	if err := checkRange(ui.Threshold, 0, 1); err != nil {
		return err
	}
	if err := checkRange(ui.Volume, -20, 20); err != nil {
		return err
	}
	if err := checkRange(ui.AllPass, 0, 1); err != nil {
		return err
	}
	return nil
}

func checkRange(val, min, max float64) error {
	if val < min || max < val {
		return ErrOutOfRange
	}
	return nil
}
