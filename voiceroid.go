package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

func CreateVoiceroidWav(speech Speech) (string, error) {
	wavFileName := fmt.Sprintf("/tmp/voice-%d.wav", time.Now().UnixNano())

	response, err := http.Get(fmt.Sprintf("%s/api/v1/audiofile?text=%s&name=%s&speed=%f&pitch=%f&range=%f", 
											vo.baseURL, 
											url.QueryEscape(speech.Text), 
											url.QueryEscape(speech.UserInfo.Voice), 
											speech.UserInfo.Speed,
											speech.UserInfo.Tone,
											speech.UserInfo.Intone))
	if err != nil {
        return "", err
    }
	defer response.Body.Close()

	file, err := os.Create(wavFileName)
	if err != nil {
        return "", err
    }
    defer file.Close()
	io.Copy(file, response.Body)
	return wavFileName, nil
}

