package voice

import (
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
)

const playTimeout = 30 * time.Second

type VoiceInstance struct {
	sync.Mutex // guards Voice
	Voice      *discordgo.VoiceConnection
	Session    *discordgo.Session
	QueueMutex sync.Mutex // guards Queue, Speaking, NowTalking
	VoiceMutex sync.Mutex // serializes playback within this instance
	NowTalking Speech
	Queue      []Speech
	GuildID    string
	ChannelID  string
	Speaking   bool
	Stop       chan bool
}

type SpeechSignal struct {
	Data Speech
	V    *VoiceInstance
}

type Speech struct {
	Text     string
	UserInfo model.UserInfo
	WavFile  string
}

// SetVoice stores the active voice connection under the instance lock.
func (v *VoiceInstance) SetVoice(vc *discordgo.VoiceConnection) {
	v.Lock()
	v.Voice = vc
	v.Unlock()
}

// GetVoice returns the active voice connection (nil if none) under the lock.
func (v *VoiceInstance) GetVoice() *discordgo.VoiceConnection {
	v.Lock()
	defer v.Unlock()
	return v.Voice
}

// Close disconnects the voice connection if present. It is idempotent and
// safe to call from concurrent teardown paths.
func (v *VoiceInstance) Close() {
	v.Lock()
	defer v.Unlock()
	if v.Voice != nil {
		v.Voice.Disconnect()
		v.Voice = nil
	}
}

func (v *VoiceInstance) PlayQueue(speech Speech) {
	// キューに追加し、Speaking フラグを原子的にチェック
	v.QueueMutex.Lock()
	v.Queue = append(v.Queue, speech)
	if v.Speaking {
		v.QueueMutex.Unlock()
		return
	}
	v.Speaking = true
	v.QueueMutex.Unlock()

	go func() {
		// 同一チャンネルで同時に読み上げるのを防ぐ。別のサーバーには影響しないようにしたい。
		v.VoiceMutex.Lock()
		defer v.VoiceMutex.Unlock()

		for {
			// キューの空チェックと Speaking=false を原子的に行う
			v.QueueMutex.Lock()
			if len(v.Queue) == 0 {
				v.Speaking = false
				v.QueueMutex.Unlock()
				return
			}
			v.NowTalking = v.Queue[0]
			v.QueueMutex.Unlock()

			if err := v.Talk(v.NowTalking); err != nil {
				log.Println("ERROR: Talk failed:", err)
			}

			v.QueueMutex.Lock()
			if len(v.Queue) > 0 {
				v.Queue = v.Queue[1:]
			}
			v.QueueMutex.Unlock()
		}
	}()
}

func (v *VoiceInstance) Talk(speech Speech) error {
	var fileName string
	var err error
	cleanup := false
	if speech.WavFile != "" {
		fileName = "wav/" + speech.WavFile
	} else {
		if IsVoiceRoid(speech.UserInfo.Voice) {
			fileName, err = CreateVoiceroidWav(speech)
		} else if strings.HasSuffix(speech.UserInfo.Voice, AivisSpeechSuffix) {
			fileName, err = createAivisSpeechWav(speech)
		} else if IsVoiceVox(speech.UserInfo.Voice) {
			fileName, err = CreateVoiceVoxWav(speech)
		} else if IsAquesTalk(speech.UserInfo.Voice) {
			fileName, err = CreateAquestalkWav(speech)
		} else {
			fileName, err = CreateWav(speech)
		}
		if err != nil {
			return err
		}
		cleanup = true
	}
	if cleanup {
		defer os.Remove(fileName)
	}

	vc := v.GetVoice()
	if vc == nil {
		return nil
	}

	// Drain any stale stop token left by a previous playback; otherwise the
	// next playback's killer goroutine would consume it and truncate this
	// utterance immediately.
	select {
	case <-v.Stop:
	default:
	}

	done := make(chan struct{})
	go func() {
		dgvoice.PlayAudioFile(vc, fileName, v.Stop, v.ChannelID)
		close(done)
	}()

	t := time.NewTimer(playTimeout)
	defer t.Stop()
	select {
	case <-done:
		return nil
	case <-t.C:
		v.StopTalking()
		<-done // wait for the playback goroutine to fully unwind
		return nil
	}
}

// StopTalking signals the current playback to stop. The send is non-blocking
// so a caller (interaction/command handler) can never block on it, and the
// Speaking flag is read under QueueMutex to avoid a data race.
func (v *VoiceInstance) StopTalking() {
	v.QueueMutex.Lock()
	speaking := v.Speaking
	v.QueueMutex.Unlock()
	if !speaking {
		return
	}
	select {
	case v.Stop <- true:
	default:
	}
}

// aivisFallbackLoggedAt throttles the fallback warning to once a minute.
var aivisFallbackLoggedAt atomic.Int64

// createAivisSpeechWav synthesizes with AivisSpeech. That engine runs on
// another host and has no fallback engine, so when it is unavailable (host
// down, model removed) the line is read with the default Open JTalk voice
// instead of being dropped; the user's saved voice is left untouched.
func createAivisSpeechWav(speech Speech) (string, error) {
	fileName, err := CreateVoiceVoxWav(speech)
	if err == nil {
		return fileName, nil
	}
	if now := time.Now().UnixNano(); now-aivisFallbackLoggedAt.Load() > int64(time.Minute) {
		aivisFallbackLoggedAt.Store(now)
		log.Printf("WARN: AivisSpeech unavailable for %s (%v); reading with Open JTalk instead", speech.UserInfo.Voice, err)
	}
	fallback := speech
	fallback.UserInfo.Voice = "normal"
	return CreateWav(fallback)
}
