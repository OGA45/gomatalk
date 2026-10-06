package voice

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
)

func CreateVoiceroidWav(speech Speech) (string, error) {
	// TTS synthesis routinely takes more than a second; a 1s whole-exchange
	// timeout caused almost every non-trivial request to fail.
	client := http.Client{
		Timeout: 15 * time.Second,
	}

	response, err := client.Get(fmt.Sprintf("%s/api/v1/audiofile?text=%s&name=%s&speed=%f&pitch=%f&range=%f",
		config.Vo().Voiceroid.BaseURL,
		url.QueryEscape(speech.Text),
		url.QueryEscape(speech.UserInfo.Voice),
		speech.UserInfo.Speed,
		speech.UserInfo.Tone,
		speech.UserInfo.Intone))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("VOICEROID audiofile status %d", response.StatusCode)
	}

	wavFile, err := os.CreateTemp("", "voice-*.wav")
	if err != nil {
		return "", err
	}
	wavFileName := wavFile.Name()
	if _, err := io.Copy(wavFile, response.Body); err != nil {
		wavFile.Close()
		os.Remove(wavFileName)
		return "", err
	}
	if err := wavFile.Close(); err != nil {
		os.Remove(wavFileName)
		return "", err
	}
	return wavFileName, nil
}
