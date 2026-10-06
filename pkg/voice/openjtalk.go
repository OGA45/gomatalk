package voice

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
)

const (
	dictDir       string = "/var/lib/mecab/dic/open-jtalk/naist-jdic"
	sysVoiceDir   string = "/usr/share/open_jtalk/voices"
	localVoiceDir string = "voices"
)

// builtinVoices is the immutable set of Open JTalk voices shipped with the bot.
// It must never be mutated after init — Voices() builds a fresh merged map.
var builtinVoices = map[string]string{
	"normal":  fmt.Sprintf("%s/%s", sysVoiceDir, "mei_normal.htsvoice"),
	"happy":   fmt.Sprintf("%s/%s", sysVoiceDir, "mei_happy.htsvoice"),
	"bashful": fmt.Sprintf("%s/%s", sysVoiceDir, "mei_bashful.htsvoice"),
	"angry":   fmt.Sprintf("%s/%s", sysVoiceDir, "mei_angry.htsvoice"),
	"sad":     fmt.Sprintf("%s/%s", sysVoiceDir, "mei_sad.htsvoice"),
	"male":    fmt.Sprintf("%s/%s", sysVoiceDir, "nitech_jp_atr503_m001.htsvoice"),
}

// Voices() is on the message hot path, so the merged result (which globs the
// local voices dir and walks the config lists) is cached behind an RWMutex.
// The cache is invalidated when the config generation changes (hot reload) or
// after a short TTL — the TTL lets late-loading VOICEVOX speakers and newly
// dropped local voice files appear without requiring a config edit.
const voicesCacheTTL = 5 * time.Second

var (
	voicesCacheMu   sync.RWMutex
	voicesCache     map[string]string
	voicesCacheGen  uint64
	voicesCacheTime time.Time
)

func VoiceList() []string {
	ans := Voices()
	keys := make([]string, len(ans))
	i := 0
	for k := range ans {
		keys[i] = k
		i++
	}
	return keys
}

// CategorizedVoiceList はエンジン種別ごとにカテゴリ分けした音声一覧を返す
func CategorizedVoiceList() string {
	var result string

	// Open JTalk (built-in)
	builtinNames := []string{}
	for k := range builtinVoices {
		builtinNames = append(builtinNames, k)
	}
	sort.Strings(builtinNames)
	if len(builtinNames) > 0 {
		result += "[Open JTalk]\n"
		for _, name := range builtinNames {
			result += "  " + name + "\n"
		}
	}

	// Local voices
	localNames := []string{}
	for k := range LocalVoiceList() {
		localNames = append(localNames, k)
	}
	sort.Strings(localNames)
	if len(localNames) > 0 {
		result += "\n[Local]\n"
		for _, name := range localNames {
			result += "  " + name + "\n"
		}
	}

	// VOICEVOX
	vvNames := []string{}
	for k := range VoicevoxList() {
		vvNames = append(vvNames, k)
	}
	sort.Strings(vvNames)
	if len(vvNames) > 0 {
		result += "\n[VOICEVOX]\n"
		for _, name := range vvNames {
			result += "  " + name + "\n"
		}
	}

	// AivisSpeech
	avNames := []string{}
	for k := range AivisSpeechList() {
		avNames = append(avNames, k)
	}
	sort.Strings(avNames)
	if len(avNames) > 0 {
		result += "\n[AivisSpeech]\n"
		for _, name := range avNames {
			result += "  " + name + "\n"
		}
	}

	// VOICEROID
	vrNames := []string{}
	for k := range VoiceRoidList() {
		vrNames = append(vrNames, k)
	}
	sort.Strings(vrNames)
	if len(vrNames) > 0 {
		result += "\n[VOICEROID]\n"
		for _, name := range vrNames {
			result += "  " + name + "\n"
		}
	}

	// AquesTalk
	aqNames := []string{}
	for k := range AquestalkList() {
		aqNames = append(aqNames, k)
	}
	sort.Strings(aqNames)
	if len(aqNames) > 0 {
		result += "\n[AquesTalk]\n"
		for _, name := range aqNames {
			result += "  " + name + "\n"
		}
	}

	return result
}

// Voices returns the merged set of all available voices. It is pure: it builds
// and returns a fresh (or cached, copy-on-write) map and never mutates shared
// state, so concurrent callers are safe.
func Voices() map[string]string {
	gen := config.Generation()
	now := time.Now()

	voicesCacheMu.RLock()
	if voicesCache != nil && voicesCacheGen == gen && now.Sub(voicesCacheTime) < voicesCacheTTL {
		m := voicesCache
		voicesCacheMu.RUnlock()
		return m
	}
	voicesCacheMu.RUnlock()

	m := buildVoices()

	voicesCacheMu.Lock()
	voicesCache = m
	voicesCacheGen = gen
	voicesCacheTime = now
	voicesCacheMu.Unlock()
	return m
}

func buildVoices() map[string]string {
	m := make(map[string]string, len(builtinVoices))
	for k, v := range builtinVoices {
		m[k] = v
	}
	m = merge(m, VoiceRoidList())
	m = merge(m, LocalVoiceList())
	m = merge(m, VoicevoxList())
	m = merge(m, AivisSpeechList())
	m = merge(m, AquestalkList())
	return m
}

func AquestalkList() map[string]string {
	list := map[string]string{}
	for _, v := range config.Aq().Aquestalk.Voice {
		list[v.Name] = v.Name
	}
	return list
}

func VoiceRoidList() map[string]string {
	list := map[string]string{}
	for _, v := range config.Vo().Voiceroid.Voice {
		list[v.Name] = v.Name
	}
	return list
}

func VoicevoxList() map[string]string {
	list := map[string]string{}
	for _, v := range GetVoicevoxVoices() {
		list[v.Name] = v.Name
	}
	return list
}

func IsVoiceRoid(name string) bool {
	for _, v := range config.Vo().Voiceroid.Voice {
		if v.Name == name {
			return true
		}
	}
	return false
}

func AivisSpeechList() map[string]string {
	list := map[string]string{}
	for _, v := range GetAivisSpeechVoices() {
		list[v.Name] = v.Name
	}
	return list
}

// IsVoiceVox reports whether a VOICEVOX-compatible engine (VOICEVOX or
// AivisSpeech) synthesizes the named voice.
func IsVoiceVox(name string) bool {
	return vvEngineFor(name) != nil
}

// IsAivisSpeech reports whether the named voice belongs to AivisSpeech.
func IsAivisSpeech(name string) bool {
	return vvEngineFor(name) == aivisEngine
}

func IsAquesTalk(name string) bool {
	for _, v := range config.Aq().Aquestalk.Voice {
		if v.Name == name {
			return true
		}
	}
	return false
}

func LocalVoiceList() map[string]string {
	pattern := localVoiceDir + "/*.htsvoice"
	files, err := filepath.Glob(pattern)
	if err != nil {
		return make(map[string]string)
	}
	list := map[string]string{}
	for _, file := range files {
		_, filename := filepath.Split(file)
		filename = filename[0 : len(filename)-len(".htsvoice")]
		list[filename] = file
	}
	return list
}

func merge(m1, m2 map[string]string) map[string]string {
	ans := map[string]string{}

	for k, v := range m1 {
		ans[k] = v
	}
	for k, v := range m2 {
		ans[k] = v
	}
	return ans
}

func CreateWav(speech Speech) (string, error) {
	// An unknown voice (e.g. an engine whose list is unavailable) would run
	// open_jtalk with an empty -m and fail with a misleading error.
	model, ok := Voices()[speech.UserInfo.Voice]
	if !ok || model == "" {
		return "", fmt.Errorf("voice not available: %s", speech.UserInfo.Voice)
	}

	// Unique temp files (os.CreateTemp is collision-safe across concurrent
	// per-guild synthesis); both share the "voice-" prefix so WavGC can sweep
	// either extension.
	wavFile, err := os.CreateTemp("", "voice-*.wav")
	if err != nil {
		log.Println("ERROR: cannot create wav temp file:", err)
		return "", err
	}
	wavFileName := wavFile.Name()
	wavFile.Close()

	textFileName, err := writeTempText(speech.Text)
	if err != nil {
		os.Remove(wavFileName)
		return "", err
	}
	defer os.Remove(textFileName)

	cmd := []string{
		"-x", dictDir,
		"-m", model,
		"-ow", wavFileName,
		"-r", fmt.Sprintf("%g", speech.UserInfo.Speed),
		"-fm", fmt.Sprintf("%g", speech.UserInfo.Tone),
		"-jf", fmt.Sprintf("%g", speech.UserInfo.Intone),
		"-u", fmt.Sprintf("%g", speech.UserInfo.Threshold),
		"-g", fmt.Sprintf("%g", speech.UserInfo.Volume),
	}

	if speech.UserInfo.AllPass > 0 {
		cmd = append(cmd, "-a")
		cmd = append(cmd, fmt.Sprintf("%g", speech.UserInfo.AllPass))
	}

	cmd = append(cmd, textFileName)

	run := exec.Command("open_jtalk", cmd...)

	if err := run.Run(); err != nil {
		log.Println("ERROR: open_jtalk run failed:", err)
		os.Remove(wavFileName)
		return "", err
	}

	return wavFileName, nil
}

// writeTempText writes content to a unique /tmp/voice-*.txt file and returns
// its path. The caller is responsible for removing it.
func writeTempText(content string) (string, error) {
	f, err := os.CreateTemp("", "voice-*.txt")
	if err != nil {
		log.Println("ERROR: cannot create text temp file:", err)
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		log.Println("ERROR: cannot write text temp file:", err)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
