// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains code related to Discord voice suppport

package discordgo

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/chacha20poly1305"
)

// ------------------------------------------------------------------------------------------------
// Code related to both VoiceConnection Websocket and UDP connections.
// ------------------------------------------------------------------------------------------------

// A VoiceConnection struct holds all the data and functions related to a Discord Voice Connection.
type VoiceConnection struct {
	sync.RWMutex

	Debug        bool // If true, print extra logging -- DEPRECATED
	LogLevel     int
	Ready        bool // If true, voice is ready to send/receive audio
	UserID       string
	GuildID      string
	ChannelID    string
	deaf         bool
	mute         bool
	speaking     bool
	reconnecting bool // If true, voice connection is trying to reconnect

	OpusSend chan []byte  // Chan for sending opus audio
	OpusRecv chan *Packet // Chan for receiving opus audio

	wsConn  *websocket.Conn
	wsMutex sync.Mutex
	udpConn *net.UDPConn
	session *Session

	sessionID string
	token     string
	endpoint  string

	// Used to send a close signal to goroutines
	close chan struct{}

	aead cipher.AEAD

	dave *daveSession
	// daveUsers is the set of other users libdave must "recognize" for MLS
	// proposal/welcome validation. Populated from CLIENTS_CONNECT (op 11) /
	// CLIENT_DISCONNECT (op 13); replayed onto a freshly created session in the
	// op4 handler in case CLIENTS_CONNECT arrived before the session existed.
	// Only touched from the (now sequential) onEvent goroutine.
	daveUsers map[string]struct{}
	// resumedSignal is closed by the RESUMED (op 9) handler to tell a pending
	// resume() that the voice session was successfully resumed.
	resumedSignal chan struct{}
	// recoverPending latches a recovery request (e.g. a UDP failure) that
	// arrived while another recovery already held the reconnecting flag, so it
	// is not silently dropped when that recovery was only a websocket resume.
	recoverPending bool

	seqAck int

	op4 voiceOP4
	op2 voiceOP2
	op8 voiceOP8

	voiceSpeakingUpdateHandlers []VoiceSpeakingUpdateHandler
}

// VoiceSpeakingUpdateHandler type provides a function definition for the
// VoiceSpeakingUpdate event
type VoiceSpeakingUpdateHandler func(vc *VoiceConnection, vs *VoiceSpeakingUpdate)

// Speaking sends a speaking notification to Discord over the voice websocket.
// This must be sent as true prior to sending audio and should be set to false
// once finished sending audio.
// b : Send true if speaking, false if not.
func (v *VoiceConnection) Speaking(b bool) (err error) {

	v.log(LogDebug, "called (%t)", b)

	type voiceSpeakingData struct {
		Speaking bool `json:"speaking"`
		Delay    int  `json:"delay"`
	}

	type voiceSpeakingOp struct {
		Op   int               `json:"op"` // Always 5
		Data voiceSpeakingData `json:"d"`
	}

	if v.wsConn == nil {
		return fmt.Errorf("no VoiceConnection websocket")
	}

	data := voiceSpeakingOp{5, voiceSpeakingData{b, 0}}
	v.wsMutex.Lock()
	err = v.wsConn.WriteJSON(data)
	v.wsMutex.Unlock()

	v.Lock()
	defer v.Unlock()
	if err != nil {
		v.speaking = false
		v.log(LogError, "Speaking() write json error, %s", err)
		return
	}

	v.speaking = b

	return
}

// ChangeChannel sends Discord a request to change channels within a Guild
// !!! NOTE !!! This function may be removed in favour of just using ChannelVoiceJoin
func (v *VoiceConnection) ChangeChannel(channelID string, mute, deaf bool) (err error) {

	v.log(LogInformational, "called")

	data := voiceChannelJoinOp{4, voiceChannelJoinData{&v.GuildID, &channelID, mute, deaf}}
	v.session.wsMutex.Lock()
	err = v.session.wsConn.WriteJSON(data)
	v.session.wsMutex.Unlock()
	if err != nil {
		return
	}
	v.ChannelID = channelID
	v.deaf = deaf
	v.mute = mute
	v.speaking = false

	return
}

// Disconnect disconnects from this voice channel and closes the websocket
// and udp connections to Discord.
func (v *VoiceConnection) Disconnect() (err error) {

	// Remove the connection from the session FIRST so a concurrent
	// reconnect()'s ownership check observes the teardown and aborts instead
	// of ghost-rejoining the channel.
	v.log(LogInformational, "Deleting VoiceConnection %s", v.GuildID)
	v.session.Lock()
	if v.session.VoiceConnections[v.GuildID] == v {
		delete(v.session.VoiceConnections, v.GuildID)
	}
	v.session.Unlock()

	// Send a OP4 with a nil channel to disconnect
	v.session.RLock()
	gwConn := v.session.wsConn
	v.session.RUnlock()
	v.Lock()
	if v.sessionID != "" {
		if gwConn != nil {
			data := voiceChannelJoinOp{4, voiceChannelJoinData{&v.GuildID, nil, true, true}}
			v.session.wsMutex.Lock()
			err = gwConn.WriteJSON(data)
			v.session.wsMutex.Unlock()
		}
		v.sessionID = ""
	}
	v.Unlock()

	// Close websocket and udp connections
	v.Close()

	return
}

// Close closes the voice ws and udp connections
func (v *VoiceConnection) Close() {

	v.log(LogInformational, "called")

	v.Lock()
	defer v.Unlock()

	v.Ready = false
	v.speaking = false
	v.dave = nil

	if v.close != nil {
		v.log(LogInformational, "closing v.close")
		close(v.close)
		v.close = nil
	}

	if v.udpConn != nil {
		v.log(LogInformational, "closing udp")
		err := v.udpConn.Close()
		if err != nil {
			v.log(LogError, "error closing udp connection, %s", err)
		}
		v.udpConn = nil
	}

	if v.wsConn != nil {
		v.log(LogInformational, "sending close frame")

		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		v.wsMutex.Lock()
		err := v.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		v.wsMutex.Unlock()
		if err != nil {
			v.log(LogError, "error closing websocket, %s", err)
		}

		// TODO: Wait for Discord to actually close the connection.
		time.Sleep(1 * time.Second)

		v.log(LogInformational, "closing websocket")
		err = v.wsConn.Close()
		if err != nil {
			v.log(LogError, "error closing websocket, %s", err)
		}

		v.wsConn = nil
	}
}

// AddHandler adds a Handler for VoiceSpeakingUpdate events.
func (v *VoiceConnection) AddHandler(h VoiceSpeakingUpdateHandler) {
	v.Lock()
	defer v.Unlock()

	v.voiceSpeakingUpdateHandlers = append(v.voiceSpeakingUpdateHandlers, h)
}

// VoiceSpeakingUpdate is a struct for a VoiceSpeakingUpdate event.
type VoiceSpeakingUpdate struct {
	UserID   string `json:"user_id"`
	SSRC     int    `json:"ssrc"`
	Speaking bool   `json:"speaking"`
}

// ------------------------------------------------------------------------------------------------
// Unexported Internal Functions Below.
// ------------------------------------------------------------------------------------------------

type voiceWebsocketMessage struct {
	Operation int             `json:"op"`
	RawData   json.RawMessage `json:"d"`
	Sequence  *int            `json:"seq"`
}

// A voiceOP4 stores the data for the voice operation 4 websocket event
// which provides us with the NaCl SecretBox encryption key
type voiceOP4 struct {
	SecretKey           []byte `json:"secret_key"`
	Mode                string `json:"mode"`
	DAVEProtocolVersion int    `json:"dave_protocol_version"`
}

// A voiceOP2 stores the data for the voice operation 2 websocket event
// which is sort of like the voice READY packet
type voiceOP2 struct {
	SSRC  uint32   `json:"ssrc"`
	Port  int      `json:"port"`
	Modes []string `json:"modes"`
	IP    string   `json:"ip"`
}

// A voiceOP8 stores the data for the voice operation 8 websocket event HELLO
type voiceOP8 struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// WaitUntilConnected waits for the Voice Connection to
// become ready, if it does not become ready it returns an err
func (v *VoiceConnection) waitUntilConnected() error {

	v.log(LogInformational, "called")

	i := 0
	for {
		v.RLock()
		ready := v.Ready
		v.RUnlock()
		if ready {
			return nil
		}

		if i > 10 {
			return fmt.Errorf("timeout waiting for voice")
		}

		time.Sleep(1 * time.Second)
		i++
	}
}

// Open opens a voice connection.  This should be called
// after VoiceChannelJoin is used and the data VOICE websocket events
// are captured.
func (v *VoiceConnection) open() (err error) {

	v.log(LogInformational, "called")

	v.Lock()
	defer v.Unlock()

	// Don't open a websocket if one is already open
	if v.wsConn != nil {
		v.log(LogWarning, "refusing to overwrite non-nil websocket")
		return
	}

	// TODO temp? loop to wait for the SessionID
	i := 0
	for {
		if v.sessionID != "" {
			break
		}

		if i > 20 { // only loop for up to 1 second total
			return fmt.Errorf("did not receive voice Session ID in time")
		}
		// Release the lock, so sessionID can be populated upon receiving a VoiceStateUpdate event.
		v.Unlock()
		time.Sleep(50 * time.Millisecond)
		i++
		v.Lock()
	}

	if v.endpoint == "" {
		return fmt.Errorf("empty voice endpoint (waiting for voice server allocation)")
	}

	// This is a fresh identify: the v8 sequence space is per-session, so a
	// seq_ack left over from a previous session must not leak into the new
	// session's heartbeats or an early RESUME.
	v.seqAck = 0

	// Connect to VoiceConnection Websocket
	vg := "wss://" + strings.TrimSuffix(v.endpoint, ":80") + "?v=8"
	v.log(LogInformational, "connecting to voice endpoint %s", vg)
	v.wsConn, _, err = v.session.Dialer.Dial(vg, nil)
	if err != nil {
		v.log(LogWarning, "error connecting to voice endpoint %s, %s", vg, err)
		v.log(LogDebug, "voice struct: %#v\n", v)
		return
	}

	type voiceHandshakeData struct {
		ServerID               string `json:"server_id"`
		UserID                 string `json:"user_id"`
		SessionID              string `json:"session_id"`
		Token                  string `json:"token"`
		MaxDAVEProtocolVersion int    `json:"max_dave_protocol_version"`
	}
	type voiceHandshakeOp struct {
		Op   int                `json:"op"` // Always 0
		Data voiceHandshakeData `json:"d"`
	}
	// max_dave_protocol_version advertises support for Discord's DAVE E2EE.
	// Setting it to 0 makes Discord reject the connection (DAVE is effectively
	// mandatory on the current rollout), so we must keep 1 here.
	data := voiceHandshakeOp{0, voiceHandshakeData{v.GuildID, v.UserID, v.sessionID, v.token, 1}}

	v.wsMutex.Lock()
	err = v.wsConn.WriteJSON(data)
	v.wsMutex.Unlock()
	if err != nil {
		v.log(LogWarning, "error sending init packet, %s", err)
		return
	}

	v.close = make(chan struct{})
	go v.wsListen(v.wsConn, v.close)

	// add loop/check for Ready bool here?
	// then return false if not ready?
	// but then wsListen will also err.

	return
}

// wsListen listens on the voice websocket for messages and passes them
// to the voice event handler.  This is automatically called by the Open func
func (v *VoiceConnection) wsListen(wsConn *websocket.Conn, close <-chan struct{}) {

	v.log(LogInformational, "called")

	for {
		// Read from the connection this listener was started for (NOT
		// v.wsConn, which resume()/Close() may swap to nil concurrently).
		messageType, message, err := wsConn.ReadMessage()
		if err != nil {
			// 4014 indicates a manual disconnection by someone in the guild;
			// 4017 indicates DAVE protocol required but not supported;
			// we shouldn't reconnect.
			if websocket.IsCloseError(err, 4014, 4017) {
				v.log(LogError, "received 4014 manual disconnection")

				// Abandon the voice WS connection
				v.Lock()
				v.wsConn = nil
				v.Unlock()

				// Wait for VOICE_SERVER_UPDATE.
				// When the bot is moved by the user to another voice channel,
				// VOICE_SERVER_UPDATE is received after the code 4014.
				for i := 0; i < 5; i++ { // TODO: temp, wait for VoiceServerUpdate.
					<-time.After(1 * time.Second)

					v.RLock()
					reconnected := v.wsConn != nil
					v.RUnlock()
					if !reconnected {
						continue
					}
					v.log(LogError, "successfully reconnected after 4014 manual disconnection")
					return
				}

				// When VOICE_SERVER_UPDATE is not received, disconnect as usual.
				v.log(LogError, "disconnect due to 4014 manual disconnection")

				v.session.Lock()
				delete(v.session.VoiceConnections, v.GuildID)
				v.session.Unlock()

				v.Close()

				return
			}

			// Detect if we have been closed manually. If a Close() has already
			// happened, the websocket we are listening on will be different to the
			// current session.
			v.RLock()
			sameConnection := v.wsConn == wsConn
			v.RUnlock()
			if sameConnection {

				v.log(LogError, "voice endpoint %s websocket closed unexpectedly, %s", v.endpoint, err)

				// Recover: RESUME first (invisible — no gateway voice-state
				// change), full reconnect only if that is not possible.
				go v.recoverAfterClose(err)
			}
			return
		}

		// Pass received message to voice event handler. This MUST be processed
		// in-order (not "go v.onEvent(...)"): the DAVE/MLS handshake is a
		// stateful, order-dependent protocol — op4 (OnSelectProtocolAck ->
		// libdave Init) must run before the external-sender/welcome/commit
		// binaries. Concurrent dispatch raced these and made libdave reject the
		// welcome ("no external sender"), triggering an invalid_commit_welcome
		// storm and a rate-limit disconnect on reconnect.
		select {
		case <-close:
			return
		default:
			v.onEvent(messageType == websocket.BinaryMessage, message)
		}
	}
}

// wsEvent handles any voice websocket events. This is only called by the
// wsListen() function.
func (v *VoiceConnection) onEvent(isBinary bool, message []byte) {

	if isBinary {
		if len(message) >= 4 {
			v.log(LogDebug, "received binary: len=%d first_bytes=[%02x %02x %02x %02x]", len(message), message[0], message[1], message[2], message[3])
		}
		// Binary voice-gateway messages carry a big-endian uint16 sequence
		// number in their first two bytes. Track it so heartbeat/RESUME
		// seq_ack reflects binary traffic too — otherwise a resume would
		// replay DAVE messages we already processed.
		if len(message) >= 2 {
			v.Lock()
			v.seqAck = int(binary.BigEndian.Uint16(message[0:2]))
			v.Unlock()
		}
		v.handleDAVEBinary(message)
		return
	}

	v.log(LogDebug, "received: %s", string(message))

	var e voiceWebsocketMessage
	if err := json.Unmarshal(message, &e); err != nil {
		v.log(LogError, "unmarshall error, %s", err)
		return
	}

	if e.Sequence != nil {
		v.Lock()
		v.seqAck = *e.Sequence
		v.Unlock()
	}

	switch e.Operation {

	case 2: // READY

		if err := json.Unmarshal(e.RawData, &v.op2); err != nil {
			v.log(LogError, "OP2 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		// Start the voice websocket heartbeat to keep the connection alive.
		// Capture the fields under the lock — resume()/Close() may swap them.
		v.RLock()
		hbConn := v.wsConn
		hbClose := v.close
		hbInterval := v.op8.HeartbeatInterval
		v.RUnlock()
		go v.wsHeartbeat(hbConn, hbClose, time.Duration(hbInterval))
		// TODO monitor a chan/bool to verify this was successful

		// Start the UDP connection
		err := v.udpOpen()
		if err != nil {
			v.log(LogError, "error opening udp connection, %s", err)
			return
		}

		return

	case 3: // HEARTBEAT response
		// add code to use this to track latency?
		// TODO: maybe actually implement this, seems cool
		return

	case 4: // udp encryption secret key
		v.Lock()

		v.op4 = voiceOP4{}
		if err := json.Unmarshal(e.RawData, &v.op4); err != nil {
			v.Unlock()
			v.log(LogError, "OP4 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		v.log(LogInformational, "OP4 received: mode=%s, dave_version=%d",
			v.op4.Mode, v.op4.DAVEProtocolVersion)

		switch v.op4.Mode {
		case "aead_aes256_gcm_rtpsize":
			block, err := aes.NewCipher(v.op4.SecretKey)
			if err != nil {
				v.Unlock()
				v.log(LogError, "error creating AES cipher, %s", err)
				return
			}
			v.aead, err = cipher.NewGCM(block)
			if err != nil {
				v.Unlock()
				v.log(LogError, "error creating GCM, %s", err)
				return
			}
		case "aead_xchacha20_poly1305_rtpsize":
			var err error
			v.aead, err = chacha20poly1305.NewX(v.op4.SecretKey)
			if err != nil {
				v.Unlock()
				v.log(LogError, "error creating XChaCha20 cipher, %s", err)
				return
			}
		default:
			v.Unlock()
			v.log(LogError, "unknown encryption mode: %s", v.op4.Mode)
			return
		}

		v.log(LogInformational, "DAVE protocol version %d", v.op4.DAVEProtocolVersion)
		if v.op4.DAVEProtocolVersion > 0 {
			v.dave = newDaveSession(v.UserID, daveCallbacks{v: v})
			chID, _ := strconv.ParseUint(v.ChannelID, 10, 64)
			v.dave.setChannelID(chID)
			v.dave.assignSSRC(v.op2.SSRC)
		}

		if v.OpusSend == nil {
			v.OpusSend = make(chan []byte, 16)
		}
		go v.opusSender(v.udpConn, v.close, v.OpusSend, 48000, 960)

		if !v.deaf {
			if v.OpusRecv == nil {
				v.OpusRecv = make(chan *Packet, 2)
			}
			go v.opusReceiver(v.udpConn, v.close, v.OpusRecv)
		}

		v.Ready = true
		v.Unlock()

		if v.dave != nil {
			// Starts the MLS handshake: godave generates and sends our key
			// package via the callbacks and prepares the initial epoch.
			v.dave.selectProtocolAck(uint16(v.op4.DAVEProtocolVersion))
			// Replay any users announced before the session existed so they
			// are recognized when the welcome/proposals arrive.
			for uid := range v.daveUsers {
				v.dave.addUser(uid)
			}
		}

		return

	case 5:
		if len(v.voiceSpeakingUpdateHandlers) == 0 {
			return
		}

		voiceSpeakingUpdate := &VoiceSpeakingUpdate{}
		if err := json.Unmarshal(e.RawData, voiceSpeakingUpdate); err != nil {
			v.log(LogError, "OP5 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		for _, h := range v.voiceSpeakingUpdateHandlers {
			h(v, voiceSpeakingUpdate)
		}

	case 21: // DAVE prepare_transition
		v.handleDAVEPrepareTransition(e.RawData)
		return

	case 22: // DAVE execute_transition
		v.handleDAVEExecuteTransition(e.RawData)
		return

	case 24: // DAVE prepare_epoch
		v.handleDAVEPrepareEpoch(e.RawData)
		return

	case 11: // CLIENTS_CONNECT — users present in / newly joining the channel.
		// libdave must "recognize" every user referenced by MLS proposals and
		// welcomes, otherwise it rejects the handshake ("unrecognized user ID").
		v.handleClientsConnect(e.RawData)
		return

	case 13: // CLIENT_DISCONNECT
		v.handleClientDisconnect(e.RawData)
		return

	case 9: // RESUMED — the voice session continues; media/UDP and the DAVE
		// session stay valid, no gateway voice-state change happened.
		v.log(LogError, "voice session RESUMED")
		v.Lock()
		if v.resumedSignal != nil {
			close(v.resumedSignal)
			v.resumedSignal = nil
		}
		// Force re-sending the speaking packet on the next playback frame.
		v.speaking = false
		wsConn := v.wsConn
		closeChan := v.close
		interval := v.op8.HeartbeatInterval
		v.Unlock()
		// The previous heartbeat goroutine died with the old connection;
		// HELLO (op 8) was re-received on this connection before RESUMED.
		go v.wsHeartbeat(wsConn, closeChan, time.Duration(interval))
		return

	case 8: // HELLO
		if err := json.Unmarshal(e.RawData, &v.op8); err != nil {
			v.log(LogError, "OP8 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}
		return

	default:
		v.log(LogDebug, "unknown voice operation, %d, %s", e.Operation, string(e.RawData))
	}

	return
}

type voiceHeartbeatOp struct {
	Op   int                `json:"op"` // Always 3
	Data voiceHeartbeatData `json:"d"`
}

type voiceHeartbeatData struct {
	T      int64 `json:"t"`
	SeqAck int   `json:"seq_ack"`
}

// NOTE :: When a guild voice server changes how do we shut this down
// properly, so a new connection can be setup without fuss?
//
// wsHeartbeat sends regular heartbeats to voice Discord so it knows the client
// is still connected.  If you do not send these heartbeats Discord will
// disconnect the websocket connection after a few seconds.
func (v *VoiceConnection) wsHeartbeat(wsConn *websocket.Conn, close <-chan struct{}, i time.Duration) {

	if close == nil || wsConn == nil {
		return
	}

	var err error
	ticker := time.NewTicker(i * time.Millisecond)
	defer ticker.Stop()
	for {
		v.log(LogDebug, "sending heartbeat packet")
		v.RLock()
		seqAck := v.seqAck
		v.RUnlock()
		v.wsMutex.Lock()
		err = wsConn.WriteJSON(voiceHeartbeatOp{3, voiceHeartbeatData{time.Now().Unix(), seqAck}})
		v.wsMutex.Unlock()
		if err != nil {
			v.log(LogError, "error sending heartbeat to voice endpoint %s, %s", v.endpoint, err)
			return
		}

		select {
		case <-ticker.C:
			// continue loop and send heartbeat
		case <-close:
			return
		}
	}
}

// ------------------------------------------------------------------------------------------------
// Code related to the VoiceConnection UDP connection
// ------------------------------------------------------------------------------------------------

type voiceUDPData struct {
	Address string `json:"address"` // Public IP of machine running this code
	Port    uint16 `json:"port"`    // UDP Port of machine running this code
	Mode    string `json:"mode"`    // always "xsalsa20_poly1305"
}

type voiceUDPD struct {
	Protocol string       `json:"protocol"` // Always "udp" ?
	Data     voiceUDPData `json:"data"`
}

type voiceUDPOp struct {
	Op   int       `json:"op"` // Always 1
	Data voiceUDPD `json:"d"`
}

// udpOpen opens a UDP connection to the voice server and completes the
// initial required handshake.  This connection is left open in the session
// and can be used to send or receive audio.  This should only be called
// from voice.wsEvent OP2
func (v *VoiceConnection) udpOpen() (err error) {

	v.Lock()
	defer v.Unlock()

	if v.wsConn == nil {
		return fmt.Errorf("nil voice websocket")
	}

	if v.udpConn != nil {
		return fmt.Errorf("udp connection already open")
	}

	if v.close == nil {
		return fmt.Errorf("nil close channel")
	}

	if v.endpoint == "" {
		return fmt.Errorf("empty endpoint")
	}

	host := v.op2.IP + ":" + strconv.Itoa(v.op2.Port)
	addr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		v.log(LogWarning, "error resolving udp host %s, %s", host, err)
		return
	}

	v.log(LogInformational, "connecting to udp addr %s", addr.String())
	v.udpConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		v.log(LogWarning, "error connecting to udp addr %s, %s", addr.String(), err)
		return
	}

	// Create a 74 byte array to store the packet data
	sb := make([]byte, 74)
	binary.BigEndian.PutUint16(sb, 1)              // Packet type (0x1 is request, 0x2 is response)
	binary.BigEndian.PutUint16(sb[2:], 70)         // Packet length (excluding type and length fields)
	binary.BigEndian.PutUint32(sb[4:], v.op2.SSRC) // The SSRC code from the Op 2 VoiceConnection event

	// And send that data over the UDP connection to Discord.
	_, err = v.udpConn.Write(sb)
	if err != nil {
		v.log(LogWarning, "udp write error to %s, %s", addr.String(), err)
		return
	}

	// Create a 74-byte array and listen for the initial handshake response
	// from Discord.  Once we get it parse the IP and PORT information out
	// of the response.  This should be our public IP and PORT as Discord
	// saw us.
	rb := make([]byte, 74)
	rlen, _, err := v.udpConn.ReadFromUDP(rb)
	if err != nil {
		v.log(LogWarning, "udp read error, %s, %s", addr.String(), err)
		return
	}

	if rlen < 74 {
		v.log(LogWarning, "received udp packet too small")
		return fmt.Errorf("received udp packet too small")
	}

	// Loop over position 8 through 71 to grab the IP address.
	var ip string
	for i := 8; i < len(rb)-2; i++ {
		if rb[i] == 0 {
			break
		}
		ip += string(rb[i])
	}

	// Grab port from position 72 and 73
	port := binary.BigEndian.Uint16(rb[len(rb)-2:])

	// Take the data from above and send it back to Discord to finalize
	// the UDP connection handshake.

	encryptionMode := ""
	for _, mode := range v.op2.Modes {
		switch mode {
		case "aead_aes256_gcm_rtpsize":
			encryptionMode = mode
		case "aead_xchacha20_poly1305_rtpsize":
			if encryptionMode == "" {
				encryptionMode = mode
			}
		}
	}
	data := voiceUDPOp{1, voiceUDPD{"udp", voiceUDPData{ip, port, encryptionMode}}}

	v.wsMutex.Lock()
	err = v.wsConn.WriteJSON(data)
	v.wsMutex.Unlock()
	if err != nil {
		v.log(LogWarning, "udp write error, %#v, %s", data, err)
		return
	}

	// start udpKeepAlive
	go v.udpKeepAlive(v.udpConn, v.close, 5*time.Second)
	// TODO: find a way to check that it fired off okay

	return
}

// udpKeepAlive sends a udp packet to keep the udp connection open
// This is still a bit of a "proof of concept"
func (v *VoiceConnection) udpKeepAlive(udpConn *net.UDPConn, close <-chan struct{}, i time.Duration) {

	if udpConn == nil || close == nil {
		return
	}

	var err error
	var sequence uint64

	packet := make([]byte, 8)

	ticker := time.NewTicker(i)
	defer ticker.Stop()
	for {

		binary.LittleEndian.PutUint64(packet, sequence)
		sequence++

		_, err = udpConn.Write(packet)
		if err != nil {
			v.log(LogError, "write error, %s", err)
			return
		}

		select {
		case <-ticker.C:
			// continue loop and send keepalive
		case <-close:
			return
		}
	}
}

// opusSender will listen on the given channel and send any
// pre-encoded opus audio to Discord.  Supposedly.
func (v *VoiceConnection) opusSender(udpConn *net.UDPConn, close <-chan struct{}, opus <-chan []byte, rate, size int) {

	if udpConn == nil || close == nil {
		return
	}

	var sequence uint16
	var timestamp uint32
	var recvbuf []byte
	var ok bool
	udpHeader := make([]byte, 12)
	nonce := make([]byte, v.aead.NonceSize())

	// build the parts that don't change in the udpHeader
	udpHeader[0] = 0x80
	udpHeader[1] = 0x78
	binary.BigEndian.PutUint32(udpHeader[8:], v.op2.SSRC)

	// start a send loop that loops until buf chan is closed
	ticker := time.NewTicker(time.Millisecond * time.Duration(size/(rate/1000)))
	defer ticker.Stop()
	for i := uint32(0); ; i++ {

		// Get data from chan.  If chan is closed, return.
		select {
		case <-close:
			return
		case recvbuf, ok = <-opus:
			if !ok {
				return
			}
			// else, continue loop
		}

		v.RLock()
		speaking := v.speaking
		v.RUnlock()
		if !speaking {
			err := v.Speaking(true)
			if err != nil {
				v.log(LogError, "error sending speaking packet, %s", err)
			}
		}

		// Add sequence and timestamp to udpPacket
		binary.BigEndian.PutUint16(udpHeader[2:], sequence)
		binary.BigEndian.PutUint32(udpHeader[4:], timestamp)

		// DAVE E2EE encryption (before transport encryption). libdave handles
		// passthrough internally, so before an epoch is active the frame is
		// returned unchanged.
		v.RLock()
		dave := v.dave
		v.RUnlock()
		if dave != nil {
			encrypted, err := dave.encrypt(v.op2.SSRC, recvbuf)
			if err != nil {
				if i%200 == 0 {
					v.log(LogError, "DAVE encrypt error: %s", err)
				}
				sequence++
				timestamp += uint32(size)
				continue
			}
			recvbuf = encrypted
		}

		// encrypt the opus data
		// add incrementing nonce counter as per discord's requirements
		binary.LittleEndian.PutUint32(nonce[:4], i)

		sendbuf := v.aead.Seal(nil, nonce, recvbuf, udpHeader)
		sendbuf = append(sendbuf, nonce[:4]...) // 4 byte nonce to ciphertext appended
		sendbuf = append(udpHeader, sendbuf...) // final

		// block here until we're exactly at the right time :)
		// Then send rtp audio packet to Discord over UDP
		select {
		case <-close:
			return
		case <-ticker.C:
			// continue
		}
		n, err := udpConn.Write(sendbuf)

		if i < 3 {
			v.log(LogDebug, "opusSender[%d]: wrote %d bytes, opus=%d, hdr=%x, err=%v", i, n, len(recvbuf), udpHeader, err)
		}

		if err != nil {
			v.log(LogError, "udp write error, %s", err)
			v.log(LogDebug, "voice struct: %#v\n", v)
			return
		}

		if (sequence) == 0xFFFF {
			sequence = 0
		} else {
			sequence++
		}

		if (timestamp + uint32(size)) >= 0xFFFFFFFF {
			timestamp = 0
		} else {
			timestamp += uint32(size)
		}
	}
}

// A Packet contains the headers and content of a received voice packet.
type Packet struct {
	SSRC      uint32
	Sequence  uint16
	Timestamp uint32
	Type      []byte
	Opus      []byte
	PCM       []int16
}

// opusReceiver listens on the UDP socket for incoming packets
// and sends them across the given channel
// NOTE :: This function may change names later.
func (v *VoiceConnection) opusReceiver(udpConn *net.UDPConn, close <-chan struct{}, c chan *Packet) {

	if udpConn == nil || close == nil {
		return
	}

	recvbuf := make([]byte, 2048)
	var nonce [12]byte

	for {
		rlen, err := udpConn.Read(recvbuf)
		if err != nil {
			// Detect if we have been closed manually. If a Close() has already
			// happened, the udp connection we are listening on will be different
			// to the current session.
			v.RLock()
			sameConnection := v.udpConn == udpConn
			v.RUnlock()
			if sameConnection {

				v.log(LogError, "udp read error, %s, %s", v.endpoint, err)
				v.log(LogDebug, "voice struct: %#v\n", v)

				go v.reconnect()
			}
			return
		}

		select {
		case <-close:
			return
		default:
			// continue loop
		}

		// For now, skip anything except RTP v2 packets (audio).
		// RTP v2 => top two bits are 10 (0x80).
		if rlen < 12 || (recvbuf[0]&0xC0) != 0x80 {
			continue
		}

		// build a audio packet struct
		p := Packet{}
		p.Type = recvbuf[0:2]
		p.Sequence = binary.BigEndian.Uint16(recvbuf[2:4])
		p.Timestamp = binary.BigEndian.Uint32(recvbuf[4:8])
		p.SSRC = binary.BigEndian.Uint32(recvbuf[8:12])

		// RTP header parsing for *_rtpsize AEAD modes:
		// - base RTP header is 12 bytes + 4 bytes per CSRC (CC).
		// - if extension bit (X) is set, ONLY the 4-byte extension preamble is unencrypted/AAD;
		//   the extension payload is encrypted and must be stripped after decryption.
		cc := int(recvbuf[0] & 0x0F)
		hasExt := (recvbuf[0] & 0x10) != 0

		baseHeaderLen := 12 + (4 * cc)
		if rlen < baseHeaderLen {
			continue
		}

		aadLen := baseHeaderLen
		extPayloadBytes := 0
		if hasExt {
			if rlen < baseHeaderLen+4 {
				continue
			}
			// Extension length is in 32-bit words at the end of the extension preamble.
			extLenWords := int(binary.BigEndian.Uint16(recvbuf[baseHeaderLen+2 : baseHeaderLen+4]))
			extPayloadBytes = extLenWords * 4
			aadLen = baseHeaderLen + 4
		}

		if rlen < aadLen+4 {
			continue
		}

		// decrypt opus data
		payload := recvbuf[aadLen:rlen]
		if len(payload) < 4 {
			continue
		}
		nonceCounter := payload[len(payload)-4:]
		cipherTextPayload := payload[:len(payload)-4]

		binary.LittleEndian.PutUint32(nonce[:4], binary.LittleEndian.Uint32(nonceCounter))

		if v.aead == nil {
			continue
		}
		// AAD must cover the unencrypted header portion.
		if plain, err := v.aead.Open(nil, nonce[:], cipherTextPayload, recvbuf[:aadLen]); err == nil {
			// If header extensions are present, strip decrypted extension payload to get to Opus.
			if extPayloadBytes > 0 {
				if len(plain) < extPayloadBytes {
					continue
				}
				plain = plain[extPayloadBytes:]
			}
			p.Opus = plain
		} else {
			continue
		}

		if c != nil {
			select {
			case c <- &p:
			case <-close:
				return
			}
		}
	}
}

// Reconnect will close down a voice connection then immediately try to
// reconnect to that session.
// NOTE : This func is messy and a WIP while I find what works.
// It will be cleaned up once a proven stable option is flushed out.
// aka: this is ugly shit code, please don't judge too harshly.
// isResumableVoiceClose reports whether an unexpected voice websocket close can
// be recovered with RESUME (opcode 7) instead of a full re-join. Network blips
// (EOF, 1006) and 4015 (voice server crashed) are resumable; session-level
// close codes are not.
func isResumableVoiceClose(err error) bool {
	return !websocket.IsCloseError(err,
		4004, // authentication failed
		4006, // session no longer valid
		4009, // session timeout
		4011, // server not found
		4012, // unknown protocol
		4014, // disconnected (handled separately in wsListen)
		4016, // unknown encryption mode
		4017, // DAVE required
		4021, // rate limited
		4022, // call terminated
	)
}

// recoverAfterClose recovers from an unexpected voice websocket close. It first
// attempts a RESUME — invisible to users: no gateway voice-state change, and
// the UDP media flow plus the DAVE/MLS session stay valid. Only when that is
// impossible does it fall back to a full reconnect.
func (v *VoiceConnection) recoverAfterClose(closeErr error) {
	v.Lock()
	if v.reconnecting {
		// Another recovery is in flight; latch the request so it is honored
		// if that recovery turns out to be only a websocket resume.
		v.recoverPending = true
		v.Unlock()
		return
	}
	v.reconnecting = true
	v.recoverPending = false
	v.Unlock()

	resumed := false
	if isResumableVoiceClose(closeErr) {
		resumed = v.resume()
	} else {
		v.log(LogError, "voice close not resumable (%s), doing full reconnect", closeErr)
	}

	v.Lock()
	v.reconnecting = false
	pending := v.recoverPending
	v.recoverPending = false
	v.Unlock()

	// A resume only repairs the websocket. If another failure (e.g. the UDP
	// receiver dying) requested recovery while we held the flag, a full
	// reconnect is still required.
	if resumed && !pending {
		return
	}

	// Before rebuilding, give a concurrent VOICE_SERVER_UPDATE re-open a
	// moment to land: a voice-server migration closes the old socket AND
	// ships a new endpoint in parallel, so the resume attempt (against the
	// dead server) fails while the real fix is already being applied by
	// onVoiceServerUpdate. Tearing that fresh session down with a full
	// reconnect is exactly the churn that ends in a visible leave/rejoin.
	if !resumed {
		for i := 0; i < 10; i++ {
			v.RLock()
			ready := v.Ready
			v.RUnlock()
			if ready {
				v.log(LogError, "voice session re-established by server update while recovering; reconnect not needed")
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	v.reconnect()
}

// resume re-opens the voice websocket to the same endpoint and sends RESUME
// (opcode 7). On success Discord replies HELLO + RESUMED (opcode 9) and the
// session continues where it left off. Returns false if a full reconnect is
// needed.
func (v *VoiceConnection) resume() bool {
	v.RLock()
	endpoint := v.endpoint
	sessionID := v.sessionID
	token := v.token
	guildID := v.GuildID
	seqAck := v.seqAck
	closeChan := v.close
	v.RUnlock()

	if endpoint == "" || sessionID == "" || token == "" || closeChan == nil {
		return false
	}

	vg := "wss://" + strings.TrimSuffix(endpoint, ":80") + "?v=8"
	v.log(LogError, "resuming voice connection to %s", vg)
	conn, _, err := v.session.Dialer.Dial(vg, nil)
	if err != nil {
		v.log(LogError, "voice resume dial failed: %s", err)
		return false
	}

	resumed := make(chan struct{})
	v.Lock()
	// Revalidate under the publish lock: if Close()/open() ran while we were
	// dialing (user bye, voice server update), v.close changed — do not
	// resurrect or clobber the connection state.
	if v.close != closeChan {
		v.Unlock()
		v.log(LogError, "voice connection replaced during resume dial, aborting resume")
		conn.Close()
		return false
	}
	v.wsConn = conn
	v.resumedSignal = resumed
	v.Unlock()

	type voiceResumeData struct {
		ServerID  string `json:"server_id"`
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
		SeqAck    int    `json:"seq_ack"`
	}
	type voiceResumeOp struct {
		Op   int             `json:"op"` // Always 7
		Data voiceResumeData `json:"d"`
	}

	v.wsMutex.Lock()
	err = conn.WriteJSON(voiceResumeOp{7, voiceResumeData{guildID, sessionID, token, seqAck}})
	v.wsMutex.Unlock()
	if err != nil {
		v.log(LogError, "voice resume send failed: %s", err)
		v.Lock()
		if v.wsConn == conn {
			v.wsConn = nil
		}
		v.resumedSignal = nil
		v.Unlock()
		conn.Close()
		return false
	}

	go v.wsListen(conn, closeChan)

	select {
	case <-resumed:
		return true
	case <-time.After(6 * time.Second):
		v.log(LogError, "voice resume timed out, falling back to full reconnect")
		v.Lock()
		if v.wsConn == conn {
			v.wsConn = nil
		}
		v.resumedSignal = nil
		v.Unlock()
		conn.Close()
		return false
	}
}

func (v *VoiceConnection) reconnect() {

	v.log(LogInformational, "called")

	v.Lock()
	if v.reconnecting {
		v.log(LogInformational, "already reconnecting to channel %s, exiting", v.ChannelID)
		// Latch the request: if the in-flight recovery is only a websocket
		// resume it must still perform a full reconnect afterwards.
		v.recoverPending = true
		v.Unlock()
		return
	}
	v.reconnecting = true
	v.recoverPending = false
	v.Unlock()

	defer func() {
		v.Lock()
		v.reconnecting = false
		// A full reconnect rebuilds everything, satisfying any request that
		// arrived while it was running.
		v.recoverPending = false
		v.Unlock()
	}()

	// Close any currently open connections
	v.Close()

	wait := time.Duration(1)
	attempts := 0
	openTried := false
	for {

		<-time.After(wait * time.Second)
		wait *= 2
		if wait > 600 {
			wait = 600
		}

		// Abort if this connection was intentionally torn down (e.g. the user
		// ran bye) while we were retrying — never ghost-rejoin.
		v.session.RLock()
		current := v.session.VoiceConnections[v.GuildID]
		v.session.RUnlock()
		if current != v {
			v.log(LogError, "voice connection for %s was removed, aborting reconnect", v.GuildID)
			return
		}

		// The VOICE_SERVER_UPDATE handler may have re-established the session
		// while we were waiting (a migration closes the old socket and ships
		// a new endpoint in parallel). Rejoining on top of a healthy session
		// only tears it down again.
		if v.isReady() {
			v.log(LogError, "voice session for %s already re-established, reconnect done", v.ChannelID)
			return
		}

		if v.session.DataReady == false || v.session.wsConn == nil {
			v.log(LogInformational, "cannot reconnect to channel %s with unready session", v.ChannelID)
			continue
		}

		// First, try re-opening the still-registered voice server session
		// (endpoint/sessionID/token survive Close). This repairs the
		// connection invisibly when the server-side session is still valid —
		// e.g. after a gateway resume, where a same-channel re-join produces
		// no VOICE_SERVER_UPDATE and therefore can never become ready.
		if !openTried {
			openTried = true
			v.RLock()
			canOpen := v.endpoint != "" && v.sessionID != "" && v.token != ""
			stale := v.wsConn != nil
			v.RUnlock()
			if canOpen {
				if stale {
					v.Close()
				}
				v.log(LogError, "trying direct voice re-open for %s", v.ChannelID)
				if err := v.open(); err != nil {
					v.log(LogError, "voice re-open for %s failed: %s", v.ChannelID, err)
				} else {
					ok := false
					for i := 0; i < 8 && !ok; i++ {
						time.Sleep(1 * time.Second)
						ok = v.isReady()
					}
					if ok {
						v.log(LogError, "voice session for %s re-opened without a gateway rejoin", v.ChannelID)
						return
					}
					v.log(LogError, "voice re-open for %s did not become ready, falling back to gateway rejoin", v.ChannelID)
				}
			}
		}

		attempts++
		v.log(LogError, "voice reconnect attempt %d for channel %s", attempts, v.ChannelID)

		// Deliberately NOT ChannelVoiceJoin: its timeout path calls Close(),
		// which destroys the session when the answering VOICE_SERVER_UPDATE
		// lands a moment too late. Send the gateway join (invisible for the
		// same channel) and poll readiness ourselves — a late completion is a
		// success to keep, not a failure to roll back.
		err := v.session.ChannelVoiceJoinManual(v.GuildID, v.ChannelID, v.mute, v.deaf)
		if err != nil {
			v.log(LogError, "voice reconnect join send failed for %s: %s", v.ChannelID, err)
		} else {
			joined := false
			for i := 0; i < 15 && !joined; i++ {
				time.Sleep(1 * time.Second)
				joined = v.isReady()
			}
			if joined {
				// If the connection was intentionally torn down (bye) while
				// the join was in flight, undo the ghost join.
				v.session.RLock()
				owner := v.session.VoiceConnections[v.GuildID]
				v.session.RUnlock()
				if owner != v {
					v.log(LogError, "voice connection for %s was torn down during reconnect, leaving again", v.GuildID)
					if owner != nil {
						owner.Disconnect()
					} else {
						v.Disconnect()
					}
					return
				}
				v.log(LogError, "voice successfully reconnected to channel %s", v.ChannelID)
				return
			}
			v.log(LogError, "voice reconnect join for %s not ready after 15s", v.ChannelID)
		}

		// Re-sending the join for the same channel is invisible to users, but
		// cannot repair a session that is wedged server-side. Sending an
		// explicit gateway disconnect resets it, at the cost of the bot
		// VISIBLY leaving and re-entering the channel — so only do that as a
		// last resort after several failed rejoin attempts, and never when
		// the session recovered in the meantime.
		if attempts < 3 || v.isReady() {
			continue
		}
		v.session.RLock()
		gwConn := v.session.wsConn
		v.session.RUnlock()
		if gwConn == nil {
			// Main gateway dropped mid-loop; retry once it is back.
			continue
		}
		v.log(LogError, "resetting wedged voice session for %s via gateway disconnect (VISIBLE leave/rejoin)", v.ChannelID)
		data := voiceChannelJoinOp{4, voiceChannelJoinData{&v.GuildID, nil, true, true}}
		v.session.wsMutex.Lock()
		err = gwConn.WriteJSON(data)
		v.session.wsMutex.Unlock()
		if err != nil {
			v.log(LogError, "error sending disconnect packet, %s", err)
		}

	}
}

// isReady reports whether the voice session is currently established.
func (v *VoiceConnection) isReady() bool {
	v.RLock()
	defer v.RUnlock()
	return v.Ready
}

// ------------------------------------------------------------------------------------------------
// DAVE E2EE Protocol Handlers
// ------------------------------------------------------------------------------------------------

func (v *VoiceConnection) handleDAVEBinary(message []byte) {
	if len(message) < 3 {
		v.log(LogWarning, "DAVE binary message too short: %d bytes", len(message))
		return
	}

	opcode := message[2]
	payload := message[3:]
	opcodeNames := map[byte]string{
		25: "ExternalSenderPackage", 27: "Proposals",
		29: "Commit", 30: "Welcome",
	}
	name := opcodeNames[opcode]
	if name == "" {
		name = "Unknown"
	}
	v.log(LogDebug, "DAVE binary opcode=%d(%s) len=%d", opcode, name, len(payload))

	v.RLock()
	dave := v.dave
	v.RUnlock()
	if dave == nil {
		v.log(LogWarning, "DAVE binary opcode %d received but no session", opcode)
		return
	}

	switch opcode {
	case 25: // external sender package
		dave.externalSenderPackage(payload)

	case 27: // proposals
		dave.proposals(payload)

	case 29: // announce commit transition — libdave processes the commit in
		// place and re-keys, so member join/leave no longer breaks audio.
		if len(payload) < 2 {
			v.log(LogWarning, "DAVE commit payload too short")
			return
		}
		transitionID := binary.BigEndian.Uint16(payload[0:2])
		v.log(LogInformational, "DAVE commit transition_id=%d", transitionID)
		dave.commit(transitionID, payload[2:])

	case 30: // welcome
		if len(payload) < 2 {
			v.log(LogWarning, "DAVE welcome payload too short")
			return
		}
		transitionID := binary.BigEndian.Uint16(payload[0:2])
		v.log(LogInformational, "DAVE welcome (%d bytes) transition_id=%d", len(payload)-2, transitionID)
		dave.welcome(transitionID, payload[2:])

	default:
		v.log(LogDebug, "DAVE unknown binary opcode %d (%d bytes)", opcode, len(payload))
	}
}

func (v *VoiceConnection) handleDAVEPrepareTransition(data json.RawMessage) {
	var msg struct {
		TransitionID        uint16 `json:"transition_id"`
		DAVEProtocolVersion int    `json:"protocol_version"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		v.log(LogError, "DAVE prepare_transition unmarshal error: %s", err)
		return
	}

	v.log(LogInformational, "DAVE prepare_transition id=%d version=%d", msg.TransitionID, msg.DAVEProtocolVersion)

	v.RLock()
	dave := v.dave
	v.RUnlock()
	if dave != nil {
		dave.prepareTransition(msg.TransitionID, uint16(msg.DAVEProtocolVersion))
	}
}

func (v *VoiceConnection) handleDAVEExecuteTransition(data json.RawMessage) {
	var msg struct {
		TransitionID uint16 `json:"transition_id"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		v.log(LogError, "DAVE execute_transition unmarshal error: %s", err)
		return
	}

	v.log(LogInformational, "DAVE execute_transition id=%d", msg.TransitionID)

	v.RLock()
	dave := v.dave
	v.RUnlock()
	if dave != nil {
		dave.executeTransition(msg.TransitionID)
	}
}

func (v *VoiceConnection) handleDAVEPrepareEpoch(data json.RawMessage) {
	var msg struct {
		Epoch               uint64 `json:"epoch"`
		DAVEProtocolVersion int    `json:"protocol_version"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		v.log(LogError, "DAVE prepare_epoch unmarshal error: %s", err)
		return
	}

	v.log(LogInformational, "DAVE prepare_epoch epoch=%d version=%d", msg.Epoch, msg.DAVEProtocolVersion)

	v.RLock()
	dave := v.dave
	v.RUnlock()
	if dave != nil {
		// godave generates and sends the key package via callbacks when needed.
		dave.prepareEpoch(int(msg.Epoch), uint16(msg.DAVEProtocolVersion))
	}
}

// handleClientsConnect records the users present in / joining the voice channel
// and registers them with the DAVE session so libdave recognizes them during
// MLS proposal/welcome validation.
func (v *VoiceConnection) handleClientsConnect(data json.RawMessage) {
	var msg struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		v.log(LogError, "clients_connect unmarshal error: %s", err)
		return
	}
	if v.daveUsers == nil {
		v.daveUsers = make(map[string]struct{})
	}
	v.RLock()
	dave := v.dave
	v.RUnlock()
	for _, uid := range msg.UserIDs {
		if uid == "" || uid == v.UserID {
			continue
		}
		v.daveUsers[uid] = struct{}{}
		if dave != nil {
			v.log(LogInformational, "DAVE recognize user %s", uid)
			dave.addUser(uid)
		}
	}
}

// handleClientDisconnect drops a user from the recognized set.
func (v *VoiceConnection) handleClientDisconnect(data json.RawMessage) {
	var msg struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		v.log(LogError, "client_disconnect unmarshal error: %s", err)
		return
	}
	if msg.UserID == "" {
		return
	}
	delete(v.daveUsers, msg.UserID)
	v.RLock()
	dave := v.dave
	v.RUnlock()
	if dave != nil {
		v.log(LogInformational, "DAVE remove user %s", msg.UserID)
		dave.removeUser(msg.UserID)
	}
}

// sendDAVEBinary sends a DAVE binary websocket message: a single opcode byte
// followed by the payload.
func (v *VoiceConnection) sendDAVEBinary(opcode byte, data []byte) {
	v.log(LogInformational, "DAVE sending binary opcode=%d (%d bytes)", opcode, len(data))
	binMsg := make([]byte, 1+len(data))
	binMsg[0] = opcode
	copy(binMsg[1:], data)

	v.wsMutex.Lock()
	defer v.wsMutex.Unlock()
	if v.wsConn != nil {
		if err := v.wsConn.WriteMessage(websocket.BinaryMessage, binMsg); err != nil {
			v.log(LogError, "DAVE binary opcode=%d send failed: %s", opcode, err)
		}
	}
}

func (v *VoiceConnection) sendDAVEReadyForTransition(transitionID uint16) {
	v.log(LogDebug, "DAVE sending ready_for_transition id=%d", transitionID)

	type readyData struct {
		TransitionID uint16 `json:"transition_id"`
	}
	type readyOp struct {
		Op   int       `json:"op"`
		Data readyData `json:"d"`
	}

	v.wsMutex.Lock()
	defer v.wsMutex.Unlock()
	if v.wsConn != nil {
		if err := v.wsConn.WriteJSON(readyOp{23, readyData{transitionID}}); err != nil {
			v.log(LogError, "DAVE ready_for_transition send failed: %s", err)
		}
	}
}

func (v *VoiceConnection) sendDAVEInvalidCommitWelcome(transitionID uint16) {
	v.log(LogInformational, "DAVE sending invalid_commit_welcome id=%d", transitionID)

	type invalidData struct {
		TransitionID uint16 `json:"transition_id"`
	}
	type invalidOp struct {
		Op   int         `json:"op"`
		Data invalidData `json:"d"`
	}

	v.wsMutex.Lock()
	defer v.wsMutex.Unlock()
	if v.wsConn != nil {
		if err := v.wsConn.WriteJSON(invalidOp{31, invalidData{transitionID}}); err != nil {
			v.log(LogError, "DAVE invalid_commit_welcome send failed: %s", err)
		}
	}
}
