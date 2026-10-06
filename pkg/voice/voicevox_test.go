package voice

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
	"github.com/OGA45/gomatalk/pkg/model"
)

// fakeEngine is a minimal VOICEVOX ENGINE that answers with the given status
// and counts the requests it served.
type fakeEngine struct {
	*httptest.Server
	status    atomic.Int64
	hits      atomic.Int64
	synthBody atomic.Value // []byte: last /synthesis request body
	speakers  string       // /speakers answer
	delay     atomic.Int64 // ms to stall /speakers (simulates a hung engine)
}

// realQuery is an audio_query answer as VOICEVOX 0.25 returns it for
// "はい、そうですか？" (trimmed), including fields gomatalk must pass through.
const realQuery = `{"accent_phrases":[{"moras":[{"text":"ハ","consonant":"h","consonant_length":0.07,"vowel":"a","vowel_length":0.1,"pitch":5.8},{"text":"イ","consonant":null,"consonant_length":null,"vowel":"i","vowel_length":0.0,"pitch":5.6}],"accent":1,"pause_mora":{"text":"、","consonant":null,"consonant_length":null,"vowel":"pau","vowel_length":0.32,"pitch":0.0},"is_interrogative":false},{"moras":[{"text":"カ","consonant":"k","consonant_length":0.06,"vowel":"a","vowel_length":0.12,"pitch":5.9}],"accent":1,"pause_mora":null,"is_interrogative":true}],"speedScale":1.0,"pitchScale":0.0,"intonationScale":1.0,"volumeScale":1.0,"prePhonemeLength":0.1,"postPhonemeLength":0.1,"pauseLength":null,"pauseLengthScale":1.0,"outputSamplingRate":24000,"outputStereo":false,"kana":"ハ'イ、カ'？"}`

func newFakeEngine(t *testing.T, status int) *fakeEngine {
	return newFakeEngineWith(t, status, `[{"name":"テスト","styles":[{"name":"ノーマル","id":3}]}]`)
}

func newFakeEngineWith(t *testing.T, status int, speakers string) *fakeEngine {
	e := &fakeEngine{speakers: speakers}
	e.status.Store(int64(status))
	e.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.hits.Add(1)
		if st := int(e.status.Load()); st != http.StatusOK {
			w.WriteHeader(st)
			return
		}
		switch r.URL.Path {
		case "/speakers":
			time.Sleep(time.Duration(e.delay.Load()) * time.Millisecond)
			w.Write([]byte(e.speakers))
		case "/audio_query":
			w.Write([]byte(realQuery))
		case "/synthesis":
			b, _ := io.ReadAll(r.Body)
			e.synthBody.Store(b)
			w.Write([]byte("RIFF-fake-wav"))
		}
	}))
	t.Cleanup(e.Server.Close)
	return e
}

// deadURL returns a URL nothing listens on (connection refused).
func deadURL(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return "http://" + addr
}

func loadVoicevoxConfig(t *testing.T, baseURL, fallbackURL string) {
	t.Helper()
	loadConfig(t, baseURL, fallbackURL, "")
}

// reloadConfig writes and loads a config with the given engine URLs ("" =
// omitted), like a hot reload: engine caches are left alone.
func reloadConfig(t *testing.T, vvBase, vvFallback, aivisBase string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	toml := "[discord]\ntoken = \"x\"\nstatus = \"x\"\nprefix = \"!\"\n\n[voicevox]\nbaseURL = \"" + vvBase + "\"\n"
	if vvFallback != "" {
		toml += "fallbackURL = \"" + vvFallback + "\"\n"
	}
	if aivisBase != "" {
		toml += "\n[aivisspeech]\nbaseURL = \"" + aivisBase + "\"\n"
	}
	if err := os.WriteFile(path, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.Load(path); err != nil {
		t.Fatal(err)
	}
}

// loadConfig loads a config with the given engine URLs ("" = omitted) and
// resets every engine's cached speakers and health.
func loadConfig(t *testing.T, vvBase, vvFallback, aivisBase string) {
	t.Helper()
	reloadConfig(t, vvBase, vvFallback, aivisBase)
	for _, e := range vvEngines {
		e.primaryDownUntil.Store(0)
		e.mu.Lock()
		e.speakers, e.loadedAt, e.lastTry, e.src, e.refreshing = nil, time.Time{}, time.Time{}, "", false
		e.mu.Unlock()
	}
}

func testSpeech() Speech {
	return Speech{Text: "テスト", UserInfo: model.UserInfo{Voice: "テスト", Speed: 1, Intone: 1}}
}

func synth(t *testing.T) error {
	t.Helper()
	f, err := CreateVoiceVoxWav(testSpeech())
	if err == nil {
		b, _ := os.ReadFile(f)
		os.Remove(f)
		if string(b) != "RIFF-fake-wav" {
			t.Fatalf("wav = %q", b)
		}
	}
	return err
}

func TestVoicevoxUsesPrimaryWhenHealthy(t *testing.T) {
	gpu, cpu := newFakeEngine(t, 200), newFakeEngine(t, 200)
	loadVoicevoxConfig(t, gpu.URL, cpu.URL)
	if err := synth(t); err != nil {
		t.Fatal(err)
	}
	if cpu.hits.Load() != 0 {
		t.Fatalf("fallback used %d times while primary was healthy", cpu.hits.Load())
	}
}

func TestVoicevoxFallsBackWhenPrimaryDown(t *testing.T) {
	cpu := newFakeEngine(t, 200)
	loadVoicevoxConfig(t, deadURL(t), cpu.URL)
	if err := synth(t); err != nil {
		t.Fatalf("no fallback: %v", err)
	}
	if voicevoxEngine.primaryDownUntil.Load() == 0 {
		t.Fatal("primary not marked down")
	}
	// While cooling down the fallback answers first: no extra primary attempt.
	if got := voicevoxEngine.order()[0]; got != cpu.URL {
		t.Fatalf("first engine during cooldown = %s, want fallback", got)
	}
}

func TestVoicevoxFallsBackOn5xx(t *testing.T) {
	gpu, cpu := newFakeEngine(t, 500), newFakeEngine(t, 200)
	loadVoicevoxConfig(t, gpu.URL, cpu.URL)
	if err := synth(t); err != nil {
		t.Fatalf("no fallback on 500: %v", err)
	}
	if cpu.hits.Load() == 0 {
		t.Fatal("fallback not used")
	}
}

func TestVoicevoxDoesNotFallBackOn4xx(t *testing.T) {
	gpu, cpu := newFakeEngine(t, 200), newFakeEngine(t, 200)
	loadVoicevoxConfig(t, gpu.URL, cpu.URL)
	FetchVoicevoxSpeakers() // resolve the voice name first
	gpu.status.Store(http.StatusUnprocessableEntity)
	if err := synth(t); err == nil {
		t.Fatal("want the 422 to surface")
	}
	if cpu.hits.Load() != 0 {
		t.Fatal("4xx should not be retried on the fallback")
	}
	if voicevoxEngine.primaryDownUntil.Load() != 0 {
		t.Fatal("4xx should not mark the primary down")
	}
}

func TestVoicevoxPrimaryRecovers(t *testing.T) {
	gpu, cpu := newFakeEngine(t, 200), newFakeEngine(t, 200)
	loadVoicevoxConfig(t, gpu.URL, cpu.URL)
	FetchVoicevoxSpeakers()
	gpu.status.Store(http.StatusServiceUnavailable)
	if err := synth(t); err != nil {
		t.Fatal(err)
	}
	// Cooldown over and the primary is healthy again: it is used and cleared.
	gpu.status.Store(http.StatusOK)
	voicevoxEngine.primaryDownUntil.Store(1)
	before := cpu.hits.Load()
	if err := synth(t); err != nil {
		t.Fatal(err)
	}
	if cpu.hits.Load() != before {
		t.Fatal("fallback used although the primary recovered")
	}
	if voicevoxEngine.primaryDownUntil.Load() != 0 {
		t.Fatal("primary still marked down after a success")
	}
}

func TestVoicevoxWithoutFallbackKeepsOldBehaviour(t *testing.T) {
	loadVoicevoxConfig(t, deadURL(t), "")
	if err := synth(t); err == nil {
		t.Fatal("want an error with no engine reachable")
	}
}

// Regression: the query used to be decoded into a fixed struct and re-encoded,
// dropping pause_mora / is_interrogative / pauseLength etc., so commas lost
// their pause and questions their rising intonation.
func TestVoicevoxQueryPassesThroughUnknownFields(t *testing.T) {
	e := newFakeEngine(t, 200)
	loadVoicevoxConfig(t, e.URL, "")
	speech := testSpeech()
	speech.UserInfo.Speed, speech.UserInfo.Intone = 1.3, 0.8
	f, err := CreateVoiceVoxWav(speech)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(f)

	var sent, orig map[string]any
	if err := json.Unmarshal(e.synthBody.Load().([]byte), &sent); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(realQuery), &orig)

	if sent["speedScale"] != 1.3 || sent["intonationScale"] != 0.8 {
		t.Fatalf("overrides not applied: speed=%v intonation=%v", sent["speedScale"], sent["intonationScale"])
	}
	delete(sent, "speedScale")
	delete(sent, "intonationScale")
	delete(orig, "speedScale")
	delete(orig, "intonationScale")
	got, _ := json.Marshal(sent)
	want, _ := json.Marshal(orig)
	if string(got) != string(want) {
		t.Fatalf("query altered in transit:\n got %s\nwant %s", got, want)
	}
}

const aivisSpeakers = `[{"name":"まお","styles":[{"name":"ノーマル","id":888753760},{"name":"あまあま","id":888753762}]}]`

func TestAivisVoicesAreSuffixedAndDispatched(t *testing.T) {
	vv := newFakeEngine(t, 200)
	av := newFakeEngineWith(t, 200, aivisSpeakers)
	loadConfig(t, vv.URL, "", av.URL)

	names := map[string]bool{}
	for _, v := range GetAivisSpeechVoices() {
		names[v.Name] = true
	}
	if !names["まお@Aivis"] || !names["まお(あまあま)@Aivis"] || len(names) != 2 {
		t.Fatalf("AivisSpeech voices = %v", names)
	}
	if !IsAivisSpeech("まお@Aivis") || IsAivisSpeech("テスト") || !IsVoiceVox("テスト") || !IsVoiceVox("まお@Aivis") {
		t.Fatal("engine classification is wrong")
	}

	speech := testSpeech()
	speech.UserInfo.Voice, speech.UserInfo.Intone = "まお(あまあま)@Aivis", 3.5
	before := vv.hits.Load()
	f, err := CreateVoiceVoxWav(speech)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(f)
	if vv.hits.Load() != before {
		t.Fatal("AivisSpeech voice was sent to VOICEVOX")
	}
	var sent map[string]any
	json.Unmarshal(av.synthBody.Load().([]byte), &sent)
	if sent["intonationScale"] != 2.0 {
		t.Fatalf("intonationScale = %v, want clamped to 2", sent["intonationScale"])
	}
}

func TestAivisDisabledWhenNotConfigured(t *testing.T) {
	vv := newFakeEngine(t, 200)
	loadVoicevoxConfig(t, vv.URL, "")
	if len(GetAivisSpeechVoices()) != 0 || IsAivisSpeech("まお@Aivis") {
		t.Fatal("AivisSpeech voices present without [aivisspeech]")
	}
}

// Other voices must not wait on (or even ask) the AivisSpeech engine.
func TestAivisNotConsultedForOtherVoices(t *testing.T) {
	vv := newFakeEngine(t, 200)
	av := newFakeEngineWith(t, 200, aivisSpeakers)
	loadConfig(t, vv.URL, "", av.URL)
	IsVoiceVox("テスト")
	IsVoiceVox("normal")
	if av.hits.Load() != 0 {
		t.Fatalf("AivisSpeech contacted %d times for non-Aivis voices", av.hits.Load())
	}
}

func TestAivisIntonationRangeValidated(t *testing.T) {
	vv := newFakeEngine(t, 200)
	av := newFakeEngineWith(t, 200, aivisSpeakers)
	loadConfig(t, vv.URL, "", av.URL)
	ui := model.UserInfo{Voice: "まお@Aivis", Speed: 1, Tone: 0, Intone: 1.5, Threshold: 0.5, Volume: 1}
	if err := ValidateUserInfo(ui); err != nil {
		t.Fatalf("valid AivisSpeech settings rejected: %v", err)
	}
	ui.Intone = 3
	if ValidateUserInfo(ui) == nil {
		t.Fatal("intonation 3 accepted for AivisSpeech (range is 0-2)")
	}
}

// A due refresh must not make callers wait on the network.
func TestSpeakerRefreshDoesNotBlock(t *testing.T) {
	vv := newFakeEngine(t, 200)
	loadVoicevoxConfig(t, vv.URL, "")
	if len(FetchVoicevoxSpeakers()) != 1 {
		t.Fatal("initial load failed")
	}
	vv.delay.Store(3000)
	voicevoxEngine.mu.Lock()
	voicevoxEngine.loadedAt = time.Now().Add(-voicevoxRefreshInterval - time.Second)
	voicevoxEngine.mu.Unlock()

	start := time.Now()
	got := FetchVoicevoxSpeakers()
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("refresh blocked the caller for %v", d)
	}
	if len(got) != 1 {
		t.Fatalf("stale list not served during refresh: %v", got)
	}
	vv.delay.Store(0)
}

// An engine with a single URL must stop being contacted for a while after it
// fails, instead of costing every line a connect/request timeout.
func TestSingleURLEngineFailsFast(t *testing.T) {
	vv := newFakeEngine(t, 200)
	av := newFakeEngineWith(t, 200, aivisSpeakers)
	loadConfig(t, vv.URL, "", av.URL)
	GetAivisSpeechVoices()
	av.status.Store(http.StatusServiceUnavailable)

	speech := testSpeech()
	speech.UserInfo.Voice = "まお@Aivis"
	if _, err := CreateVoiceVoxWav(speech); err == nil {
		t.Fatal("want an error from the failing engine")
	}
	hits := av.hits.Load()
	_, err := CreateVoiceVoxWav(speech)
	if !errors.Is(err, errEngineDown) {
		t.Fatalf("second attempt err = %v, want errEngineDown", err)
	}
	if av.hits.Load() != hits {
		t.Fatal("engine contacted during its cooldown")
	}
}

// Changing an engine URL by hot reload must replace its cached voices.
func TestHotReloadResetsSpeakerCache(t *testing.T) {
	vv := newFakeEngine(t, 200)
	a := newFakeEngineWith(t, 200, aivisSpeakers)
	b := newFakeEngineWith(t, 200, `[{"name":"コハク","styles":[{"name":"ノーマル","id":1878365376}]}]`)
	loadConfig(t, vv.URL, "", a.URL)
	if !IsAivisSpeech("まお@Aivis") {
		t.Fatal("initial AivisSpeech list missing")
	}
	reloadConfig(t, vv.URL, "", b.URL)
	if IsAivisSpeech("まお@Aivis") || !IsAivisSpeech("コハク@Aivis") {
		t.Fatalf("voices after reload = %v", GetAivisSpeechVoices())
	}
	reloadConfig(t, vv.URL, "", "")
	if len(GetAivisSpeechVoices()) != 0 {
		t.Fatal("AivisSpeech voices kept after removing [aivisspeech]")
	}
}

// An answer from the fallback must not shrink a list loaded from the primary.
func TestFallbackAnswerDoesNotShrinkList(t *testing.T) {
	gpu := newFakeEngineWith(t, 200, `[{"name":"テスト","styles":[{"name":"ノーマル","id":3}]},{"name":"新キャラ","styles":[{"name":"ノーマル","id":99}]}]`)
	cpu := newFakeEngine(t, 200)
	loadVoicevoxConfig(t, gpu.URL, cpu.URL)
	if len(FetchVoicevoxSpeakers()) != 2 {
		t.Fatal("initial load failed")
	}
	gpu.status.Store(http.StatusServiceUnavailable)
	voicevoxEngine.mu.Lock()
	voicevoxEngine.loadedAt, voicevoxEngine.lastTry = time.Time{}, time.Time{} // force a synchronous re-fetch
	voicevoxEngine.mu.Unlock()
	if got := FetchVoicevoxSpeakers(); len(got) != 2 {
		t.Fatalf("list shrank to %v after the fallback answered", got)
	}
}
