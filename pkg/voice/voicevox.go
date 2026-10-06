package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
	"github.com/OGA45/gomatalk/pkg/model"
)

// voicevoxRetryInterval throttles re-fetching the speaker list when the engine
// is unreachable, so a not-yet-started engine does not permanently disable the
// auto-discovered voices (it retries instead of caching an empty list forever).
const voicevoxRetryInterval = 30 * time.Second

// voicevoxRefreshInterval re-reads a loaded speaker list, so models added to or
// removed from an engine (AivisHub) show up without restarting the bot.
const voicevoxRefreshInterval = 10 * time.Minute

// voicevoxCooldown is how long a failed engine URL is avoided: tried after the
// fallback when there is one, or failed fast when it is the only URL, so an
// unreachable host does not add a connect timeout to every message.
const voicevoxCooldown = 60 * time.Second

// AivisSpeechSuffix ends every AivisSpeech voice name ("まお@Aivis",
// "まお(あまあま)@Aivis"), keeping them distinct from VOICEVOX and other voices.
// Users store voices by name, so changing it orphans saved settings.
const AivisSpeechSuffix = "@Aivis"

// voicevoxClient fails fast on connection setup (a dead engine host must not
// eat the whole request budget before the fallback gets a chance). Each
// request is bounded by its engine's timeout through its context.
var voicevoxClient = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	return &http.Client{Transport: t}
}()

// vvEngine is one engine speaking the VOICEVOX ENGINE HTTP API (VOICEVOX
// itself, AivisSpeech Engine, ...): where to reach it, its cached speaker list
// and the health of its primary URL.
type vvEngine struct {
	id     string // catalog engine id
	label  string // shown in logs and voice lists
	suffix string // appended to voice names so engines never collide
	// maxIntonation clamps intonationScale (0 = no clamp). AivisSpeech uses it
	// as emotion strength in 0-2.
	maxIntonation float64
	timeout       time.Duration // per request
	urls          func() (base, fallback string)
	pinned        func() []model.VoiceVox // voices fixed in config, if any

	mu         sync.Mutex
	speakers   []model.VoiceVox
	loadedAt   time.Time
	lastTry    time.Time
	refreshing bool
	src        string // the URLs the cached list was loaded from

	primaryDownUntil atomic.Int64 // unix nanoseconds; 0 = healthy
}

var (
	voicevoxEngine = &vvEngine{
		id:      EngineVoicevox,
		label:   "VOICEVOX",
		timeout: 10 * time.Second,
		urls: func() (string, string) {
			c := config.Vv().Voicevox
			return c.BaseURL, c.FallbackURL
		},
		pinned: func() []model.VoiceVox { return config.Vv().Voicevox.Voice },
	}
	aivisEngine = &vvEngine{
		id:            EngineAivisSpeech,
		label:         "AivisSpeech",
		suffix:        AivisSpeechSuffix,
		maxIntonation: 2,
		timeout:       20 * time.Second, // runs on CPU: long lines take several seconds
		urls: func() (string, string) {
			c := config.Vv().AivisSpeech
			return c.BaseURL, c.FallbackURL
		},
	}
	vvEngines = []*vvEngine{voicevoxEngine, aivisEngine}
)

// vvEngineFor returns the engine that synthesizes the named voice, or nil.
// Suffixed engines are only asked about names carrying their suffix.
func vvEngineFor(name string) *vvEngine {
	for _, e := range vvEngines {
		if e.suffix != "" && !strings.HasSuffix(name, e.suffix) {
			continue
		}
		if _, ok := e.idByName(name); ok {
			return e
		}
	}
	return nil
}

// voicevoxStatusError is a non-200 answer from an engine.
type voicevoxStatusError struct {
	op   string
	code int
}

func (e *voicevoxStatusError) Error() string {
	return fmt.Sprintf("VOICEVOX %s status %d", e.op, e.code)
}

// errEngineDown is returned without contacting an engine whose only URL failed
// within voicevoxCooldown.
var errEngineDown = errors.New("engine is down (cooling down after a failure)")

// engineFailed reports whether err means the engine itself is unavailable or
// broken (network error, timeout, 5xx, garbled response), so another engine
// may succeed. A 4xx is the request's fault and would fail everywhere.
func engineFailed(err error) bool {
	var se *voicevoxStatusError
	if errors.As(err, &se) {
		return se.code >= 500
	}
	return true
}

// order returns the base URLs to try: baseURL then fallbackURL, or the
// reverse while baseURL is cooling down after a failure.
func (e *vvEngine) order() []string {
	base, fallback := e.urls()
	if base == "" {
		if fallback == "" {
			return nil
		}
		return []string{fallback}
	}
	if fallback == "" || fallback == base {
		return []string{base}
	}
	if time.Now().UnixNano() < e.primaryDownUntil.Load() {
		return []string{fallback, base}
	}
	return []string{base, fallback}
}

// with runs fn against each URL until one succeeds and returns the URL that
// answered. Only engine failures move on to the next URL. The primary URL's
// health is tracked: after a failure it is tried second, or — when it is the
// only URL — skipped outright, for voicevoxCooldown.
func (e *vvEngine) with(op string, fn func(ctx context.Context, baseURL string) error) (string, error) {
	urls := e.order()
	if len(urls) == 0 {
		return "", fmt.Errorf("%s baseURL is not configured", e.label)
	}
	tracked, _ := e.urls()
	if len(urls) == 1 {
		tracked = urls[0]
		if time.Now().UnixNano() < e.primaryDownUntil.Load() {
			return "", fmt.Errorf("%s %s: %w", e.label, tracked, errEngineDown)
		}
	}
	var err error
	for _, base := range urls {
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		err = fn(ctx, base)
		cancel()
		if err == nil {
			if base == tracked && e.primaryDownUntil.Swap(0) != 0 {
				log.Printf("INFO: %s engine %s is back", e.label, base)
			}
			return base, nil
		}
		if !engineFailed(err) {
			return "", err
		}
		if base == tracked {
			if e.primaryDownUntil.Swap(time.Now().Add(voicevoxCooldown).UnixNano()) == 0 {
				what := "using the fallback"
				if len(urls) == 1 {
					what = "skipping it"
				}
				log.Printf("WARN: %s engine %s failed (%s: %v); %s for %v", e.label, base, op, err, what, voicevoxCooldown)
			}
		}
	}
	return "", err
}

// voicevoxSpeaker は /speakers API のレスポンス構造体
type voicevoxSpeaker struct {
	Name   string `json:"name"`
	Styles []struct {
		Name string `json:"name"`
		Id   int    `json:"id"`
	} `json:"styles"`
}

// fetchSpeakers returns the cached speaker list and keeps it current. Only the
// very first load is waited for; later refreshes and retries run in the
// background, so an unreachable engine never stalls the callers (voice
// lookups for every utterance, /voices_list, the Activity).
func (e *vvEngine) fetchSpeakers() []model.VoiceVox {
	base, fallback := e.urls()
	if base == "" && fallback == "" {
		return nil
	}
	src := base + "\x00" + fallback

	e.mu.Lock()
	if src != e.src {
		// The URLs changed (config hot reload): the cache belongs to another engine.
		e.src = src
		e.speakers, e.loadedAt, e.lastTry = nil, time.Time{}, time.Time{}
		e.primaryDownUntil.Store(0)
	}
	fresh := !e.loadedAt.IsZero() && time.Since(e.loadedAt) < voicevoxRefreshInterval
	throttled := !e.lastTry.IsZero() && time.Since(e.lastTry) < voicevoxRetryInterval
	if fresh || throttled || e.refreshing {
		defer e.mu.Unlock()
		return e.speakers
	}
	first := e.lastTry.IsZero()
	e.lastTry = time.Now()
	if first {
		// So the voices exist when the bot starts talking (as before).
		defer e.mu.Unlock()
		list, from, err := e.loadSpeakers()
		e.storeSpeakers(src, list, from, err)
		return e.speakers
	}
	e.refreshing = true
	cached := e.speakers
	e.mu.Unlock()

	go func() {
		list, from, err := e.loadSpeakers()
		e.mu.Lock()
		defer e.mu.Unlock()
		e.refreshing = false
		e.storeSpeakers(src, list, from, err)
	}()
	return cached
}

// loadSpeakers fetches /speakers and builds the voice list (no locks held).
func (e *vvEngine) loadSpeakers() ([]model.VoiceVox, string, error) {
	var speakers []voicevoxSpeaker
	from, err := e.with("speakers", func(ctx context.Context, baseURL string) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/speakers", nil)
		if err != nil {
			return err
		}
		resp, err := voicevoxClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return &voicevoxStatusError{"speakers", resp.StatusCode}
		}
		return json.NewDecoder(resp.Body).Decode(&speakers)
	})
	if err != nil {
		return nil, "", err
	}
	built := make([]model.VoiceVox, 0, len(speakers))
	for _, s := range speakers {
		for _, style := range s.Styles {
			name := s.Name
			if style.Name != "ノーマル" {
				name = fmt.Sprintf("%s(%s)", s.Name, style.Name)
			}
			built = append(built, model.VoiceVox{Name: name + e.suffix, Id: style.Id})
		}
	}
	return built, from, nil
}

// storeSpeakers installs a fetch result. The caller holds e.mu. A failed fetch
// keeps the previous list; so does an answer from the fallback URL once a list
// is loaded (the fallback may be an older engine with fewer voices), leaving
// loadedAt stale so the primary is asked again soon.
func (e *vvEngine) storeSpeakers(src string, list []model.VoiceVox, from string, err error) {
	if src != e.src {
		return // the URLs changed while fetching
	}
	if err != nil {
		log.Printf("WARN: %s /speakers fetch failed: %v", e.label, err)
		return
	}
	if primary, _ := e.urls(); from != primary && e.speakers != nil {
		return
	}
	if e.loadedAt.IsZero() || len(list) != len(e.speakers) {
		log.Printf("INFO: %s speakers loaded: %d voices", e.label, len(list))
	}
	e.speakers = list
	e.loadedAt = time.Now()
}

// voices は config の手動設定と API 自動取得をマージして返す
func (e *vvEngine) voices() []model.VoiceVox {
	var pinned []model.VoiceVox
	if e.pinned != nil {
		pinned = e.pinned()
	}
	voices := make([]model.VoiceVox, len(pinned))
	copy(voices, pinned)

	// 自動取得分のうち、config に同名がないものを追加
	configNames := map[string]bool{}
	for _, v := range voices {
		configNames[v.Name] = true
	}
	for _, v := range e.fetchSpeakers() {
		if !configNames[v.Name] {
			voices = append(voices, v)
		}
	}
	return voices
}

// idByName resolves a speaker (style) id by display name.
func (e *vvEngine) idByName(name string) (int, bool) {
	for _, v := range e.voices() {
		if v.Name == name {
			return v.Id, true
		}
	}
	return 0, false
}

// FetchVoicevoxSpeakers returns the speakers auto-discovered from VOICEVOX.
func FetchVoicevoxSpeakers() []model.VoiceVox { return voicevoxEngine.fetchSpeakers() }

// GetVoicevoxVoices returns the configured plus auto-discovered VOICEVOX voices.
func GetVoicevoxVoices() []model.VoiceVox { return voicevoxEngine.voices() }

// GetAivisSpeechVoices returns the voices of the AivisSpeech engine (names
// carry the "@Aivis" suffix); empty when [aivisspeech] is not configured.
func GetAivisSpeechVoices() []model.VoiceVox { return aivisEngine.voices() }

// audioQuery asks the engine at baseURL for the audio query of speech. The
// query is kept as raw JSON and sent back as-is apart from the fields we
// override: decoding it into a fixed struct dropped every field the struct did
// not know (pause_mora, is_interrogative, pauseLength, ...), which silently
// removed comma pauses and question intonation.
func audioQuery(ctx context.Context, baseURL string, speech Speech, speakerID int) (map[string]json.RawMessage, error) {
	endpoint := fmt.Sprintf("%s/audio_query?text=%s&speaker=%d", baseURL, url.QueryEscape(speech.Text), speakerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := voicevoxClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &voicevoxStatusError{"audio_query", response.StatusCode}
	}

	var query map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&query); err != nil {
		return nil, err
	}
	return query, nil
}

// synthesize runs audio_query + synthesis on one URL. Both steps go to the
// same engine so a query is never synthesized by a different version.
func (e *vvEngine) synthesize(ctx context.Context, baseURL string, speech Speech, speakerID int) ([]byte, error) {
	query, err := audioQuery(ctx, baseURL, speech, speakerID)
	if err != nil {
		return nil, err
	}
	intonation := speech.UserInfo.Intone
	if e.maxIntonation > 0 && intonation > e.maxIntonation {
		intonation = e.maxIntonation
	}
	// PitchScale (tone) と VolumeScale は VOICEVOX のレンジが Open JTalk と異なり、
	// 無検証のマッピングは音割れを招くため現状は適用しない（要・実機調整）。
	for key, value := range map[string]float64{
		"speedScale":      speech.UserInfo.Speed,
		"intonationScale": intonation,
	} {
		if query[key], err = json.Marshal(value); err != nil {
			return nil, err
		}
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/synthesis?speaker=%d", baseURL, speakerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")

	response, err := voicevoxClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &voicevoxStatusError{"synthesis", response.StatusCode}
	}
	return io.ReadAll(response.Body)
}

// createWav synthesizes speech with this engine into a temp WAV file.
func (e *vvEngine) createWav(speech Speech) (string, error) {
	speakerID, ok := e.idByName(speech.UserInfo.Voice)
	if !ok {
		return "", fmt.Errorf("%s voice not found: %s", e.label, speech.UserInfo.Voice)
	}

	var wav []byte
	_, err := e.with("synthesis", func(ctx context.Context, baseURL string) (err error) {
		wav, err = e.synthesize(ctx, baseURL, speech, speakerID)
		return err
	})
	if err != nil {
		log.Printf("ERROR: %s synthesis: %v", e.label, err)
		return "", err
	}

	wavFile, err := os.CreateTemp("", "voice-*.wav")
	if err != nil {
		log.Println("ERROR: cannot create wav temp file:", err)
		return "", err
	}
	wavFileName := wavFile.Name()
	if _, err := wavFile.Write(wav); err != nil {
		wavFile.Close()
		os.Remove(wavFileName)
		log.Println("ERROR: wav write:", err)
		return "", err
	}
	if err := wavFile.Close(); err != nil {
		os.Remove(wavFileName)
		return "", err
	}

	return wavFileName, nil
}

// CreateVoiceVoxWav synthesizes speech with whichever VOICEVOX-compatible
// engine owns the selected voice.
func CreateVoiceVoxWav(speech Speech) (string, error) {
	e := vvEngineFor(speech.UserInfo.Voice)
	if e == nil {
		return "", fmt.Errorf("VOICEVOX voice not found: %s", speech.UserInfo.Voice)
	}
	return e.createWav(speech)
}
