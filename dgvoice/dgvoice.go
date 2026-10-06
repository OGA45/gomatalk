/*******************************************************************************
 * This is very experimental code and probably a long way from perfect or
 * ideal.  Please provide feed back on areas that would improve performance
 *
 */

// Package dgvoice provides opus encoding and audio file playback for the
// Discordgo package.
package dgvoice

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/Carmen-Shannon/gopus"
	"github.com/bwmarrin/discordgo"
)

// NOTE: This API is not final and these are likely to change.

// Technically the below settings can be adjusted however that poses
// a lot of other problems that are not handled well at this time.
// These below values seem to provide the best overall performance
const (
	channels  int = 2                   // 1 for mono, 2 for stereo
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

// OnError gets called by dgvoice when an error is encountered.
// By default logs to STDERR
var OnError = func(str string, err error) {
	prefix := "dgVoice: " + str

	if err != nil {
		os.Stderr.WriteString(prefix + ": " + err.Error() + "\n")
	} else {
		os.Stderr.WriteString(prefix + "\n")
	}
}

// SendPCM will receive on the provied channel encode
// received PCM data into Opus then send that to Discordgo
func SendPCM(v *discordgo.VoiceConnection, pcm <-chan []int16, channelId string) {
	defer log.Printf("SendPCM finished for %s", channelId)

	if pcm == nil {
		return
	}

	// A per-call encoder; a package-global encoder would race across
	// concurrent playbacks in different guilds.
	opusEncoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	if err != nil {
		OnError("NewEncoder Error", err)
		return
	}

	for {

		// read pcm from chan, exit if channel is closed.
		recv, ok := <-pcm
		if !ok {
			OnError("PCM Channel closed", nil)
			return
		}

		// try encoding pcm frame with Opus
		buf := make([]byte, maxBytes)
		opus, err := opusEncoder.Encode(recv, frameSize, buf)
		if err != nil {
			OnError("Encoding Error", err)
			return
		}
		if !v.Ready || v.OpusSend == nil {
			// Discordgo not ready for opus packets
			return
		}
		// send encoded opus data to the sendOpus channel
		v.OpusSend <- opus
	}
}

// PlayAudioFile will play the given filename to the already connected
// Discord voice server/channel.  voice websocket and udp socket
// must already be setup before this will work.
func PlayAudioFile(v *discordgo.VoiceConnection, filename string, stop chan bool, channelId string) {
	defer log.Printf("PlayAudioFile finished for %s", filename)
	defer func() {
		// Non-blocking: the killer goroutine may have already consumed an
		// external stop signal and exited, so this must not block.
		select {
		case stop <- true:
		default:
		}
	}()

	// Create a shell command "object" to run.
	run := exec.Command("ffmpeg", "-i", filename, "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	ffmpegout, err := run.StdoutPipe()
	if err != nil {
		OnError("StdoutPipe Error", err)
		return
	}

	ffmpegbuf := bufio.NewReaderSize(ffmpegout, 16384)

	// Starts the ffmpeg command
	err = run.Start()
	if err != nil {
		OnError("RunStart Error", err)
		return
	}

	// prevent memory leak from residual ffmpeg streams
	defer run.Process.Kill()

	//when stop is sent, kill ffmpeg
	go func() {
		<-stop
		err = run.Process.Kill()
	}()

	// Send "speaking" packet over the voice websocket
	err = v.Speaking(true)
	if err != nil {
		OnError("Couldn't set speaking", err)
	}

	// Send not "speaking" packet over the websocket when we finish
	defer func() {
		err := v.Speaking(false)
		if err != nil {
			OnError("Couldn't stop speaking", err)
		}
	}()

	send := make(chan []int16, 2)
	defer close(send)

	// Buffered so the SendPCM wrapper goroutine never blocks on this send,
	// which previously leaked one goroutine per playback.
	closeChan := make(chan bool, 1)
	go func() {
		SendPCM(v, send, channelId)
		closeChan <- true
	}()

	for {
		// read data from ffmpeg stdout
		audiobuf := make([]int16, frameSize*channels)
		err = binary.Read(ffmpegbuf, binary.LittleEndian, &audiobuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		if err != nil {
			OnError("error reading from ffmpeg stdout", err)
			return
		}

		// Send received PCM to the sendPCM channel
		select {
		case send <- audiobuf:
		case <-closeChan:
			return
		}
	}
}
