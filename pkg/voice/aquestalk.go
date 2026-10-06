package voice

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/OGA45/gomatalk/pkg/config"
)

func CreateAquestalkWav(speech Speech) (string, error) {
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
		"-o", wavFileName,
		"-f", textFileName,
		"-s", fmt.Sprintf("%g", speech.UserInfo.Speed*100),
		"-g", fmt.Sprintf("%g", (speech.UserInfo.Volume+20)*2.5),
	}

	run := exec.Command(config.Aq().Aquestalk.ExePath, cmd...)

	if err := run.Run(); err != nil {
		log.Println("ERROR: AquesTalk run failed:", err)
		os.Remove(wavFileName)
		return "", err
	}

	return wavFileName, nil
}
