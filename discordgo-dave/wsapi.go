// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains low level functions for interacting with the Discord
// data websocket interface.

package discordgo

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ErrWSAlreadyOpen is thrown when you attempt to open
// a websocket that already is open.
var ErrWSAlreadyOpen = errors.New("web socket already opened")

// ErrWSNotFound is thrown when you attempt to use a websocket
// that doesn't exist
var ErrWSNotFound = errors.New("no websocket connection exists")

// ErrWSShardBounds is thrown when you try to use a shard ID that is
// more than the total shard count
var ErrWSShardBounds = errors.New("ShardID must be less than ShardCount")

type resumePacket struct {
	Op   int `json:"op"`
	Data struct {
		Token     string `json:"token"`
		SessionID string `json:"session_id"`
		Sequence  int64  `json:"seq"`
	} `json:"d"`
}

// Open creates a websocket connection to Discord.
// See: https://discord.com/developers/docs/topics/gateway#connecting
func (s *Session) Open() error {
	s.log(LogInformational, "called")

	var err error

	// Prevent Open or other major Session functions from
	// being called while Open is still running.
	s.Lock()
	defer s.Unlock()

	// If the websock is already open, bail out here.
	if s.wsConn != nil {
		return ErrWSAlreadyOpen
	}

	sequence := atomic.LoadInt64(s.sequence)

	var gateway string
	// Get the gateway to use for the Websocket connection
	if sequence != 0 && s.sessionID != "" && s.resumeGatewayURL != "" {
		s.log(LogDebug, "using resume gateway %s", s.resumeGatewayURL)
		gateway = s.resumeGatewayURL
	} else {
		if s.gateway == "" {
			s.gateway, err = s.Gateway()
			if err != nil {
				return err
			}
		}

		gateway = s.gateway
	}

	// Add the version and encoding to the URL
	gateway += "?v=" + APIVersion + "&encoding=json"

	// Connect to the Gateway
	s.log(LogInformational, "connecting to gateway %s", gateway)
	header := http.Header{}
	header.Add("accept-encoding", "zlib")
	s.wsConn, _, err = s.Dialer.Dial(gateway, header)
	if err != nil {
		s.log(LogError, "error connecting to gateway %s, %s", s.gateway, err)
		s.gateway = "" // clear cached gateway
		s.wsConn = nil // Just to be safe.
		return err
	}

	s.wsConn.SetCloseHandler(func(code int, text string) error {
		return nil
	})

	defer func() {
		// because of this, all code below must set err to the error
		// when exiting with an error :)  Maybe someone has a better
		// way :)
		if err != nil {
			s.wsConn.Close()
			s.wsConn = nil
		}
	}()

	// Open() holds s.Lock() for the whole handshake, so its reads must not
	// block forever. listen() runs without a deadline; missing heartbeat ACKs
	// detect a dead connection there.
	if err = s.wsConn.SetReadDeadline(time.Now().Add(handshakeReadTimeout)); err != nil {
		return err
	}

	// The first response from Discord should be an Op 10 (Hello) Packet.
	// When processed by onEvent the heartbeat goroutine will be started.
	mt, m, err := s.wsConn.ReadMessage()
	if err != nil {
		return err
	}
	e, err := s.onEvent(mt, m, true)
	if err != nil {
		return err
	}
	if e.Operation != 10 {
		err = fmt.Errorf("expecting Op 10, got Op %d instead", e.Operation)
		return err
	}
	s.log(LogInformational, "Op 10 Hello Packet received from Discord")
	var h helloOp
	if err = json.Unmarshal(e.RawData, &h); err != nil {
		err = fmt.Errorf("error unmarshalling helloOp, %s", err)
		return err
	}

	// Now we send either an Op 2 Identity if this is a brand new
	// connection or Op 6 Resume if we are resuming an existing connection.
	if s.sessionID == "" && sequence == 0 {

		// Send Op 2 Identity Packet
		err = s.identify()
		if err != nil {
			err = fmt.Errorf("error sending identify packet to gateway, %s, %s", s.gateway, err)
			return err
		}

	} else {

		// Send Op 6 Resume Packet
		p := resumePacket{}
		p.Op = 6
		p.Data.Token = s.Token
		p.Data.SessionID = s.sessionID
		p.Data.Sequence = sequence

		s.log(LogInformational, "sending resume packet to gateway")
		s.wsMutex.Lock()
		err = s.wsConn.WriteJSON(p)
		s.wsMutex.Unlock()
		if err != nil {
			err = fmt.Errorf("error sending gateway resume packet, %s, %s", s.gateway, err)
			return err
		}

	}

	// A basic state is a hard requirement for Voice.
	// We create it here so the below READY/RESUMED packet can populate
	// the state :)
	// XXX: Move to New() func?
	if s.State == nil {
		state := NewState()
		state.TrackChannels = false
		state.TrackEmojis = false
		state.TrackMembers = false
		state.TrackRoles = false
		state.TrackVoice = false
		s.State = state
	}

	// Now Discord should send us a READY or RESUMED packet.
	mt, m, err = s.wsConn.ReadMessage()
	if err != nil {
		// 4007 Invalid seq / 4009 Session timed out: the session can never be
		// resumed, so identify afresh on the next attempt.
		if websocket.IsCloseError(err, 4007, 4009) {
			s.discardSession()
		}
		return err
	}
	e, err = s.onEvent(mt, m, true)
	if err != nil {
		return err
	}
	// Discord answers a RESUME/IDENTIFY it will not accept with Op9 Invalid
	// Session, and may send Op7 Reconnect at any time. Fail this attempt: the
	// deferred cleanup drops the socket and reconnect() retries with backoff
	// (identifying afresh if onEvent discarded a non-resumable session).
	if e.Operation == 7 || e.Operation == 9 {
		err = fmt.Errorf("gateway answered the handshake with Op %d", e.Operation)
		return err
	}
	if e.Type != `READY` && e.Type != `RESUMED` {
		// This is not fatal, but it does not follow their API documentation.
		s.log(LogWarning, "Expected READY/RESUMED, instead got:\n%#v\n", e)
	}
	s.log(LogInformational, "First Packet:\n%#v\n", e)
	if err = s.wsConn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}

	s.log(LogInformational, "We are now connected to Discord, emitting connect event")
	s.handleEvent(connectEventType, &Connect{})

	// A VoiceConnections map is a hard requirement for Voice.
	// XXX: can this be moved to when opening a voice connection?
	if s.VoiceConnections == nil {
		s.log(LogInformational, "creating new VoiceConnections map")
		s.VoiceConnections = make(map[string]*VoiceConnection)
	}

	// Create listening chan outside of listen, as it needs to happen inside the
	// mutex lock and needs to exist before calling heartbeat and listen
	// go rountines.
	s.listening = make(chan interface{})

	// Only a completed handshake or a real Op11 counts as liveness (the app's
	// watchdog relies on this); heartbeat()'s ACK check starts from here.
	s.LastHeartbeatAck = time.Now().UTC()

	// Start sending heartbeats and reading messages from Discord.
	go s.heartbeat(s.wsConn, s.listening, h.HeartbeatInterval)
	go s.listen(s.wsConn, s.listening)

	s.log(LogInformational, "exiting")
	return nil
}

// listen polls the websocket connection for events, it will stop when the
// listening channel is closed, or an error occurs.
func (s *Session) listen(wsConn *websocket.Conn, listening <-chan interface{}) {

	s.log(LogInformational, "called")

	for {

		messageType, message, err := wsConn.ReadMessage()

		if err != nil {

			// Detect if we have been closed manually. If a Close() has already
			// happened, the websocket we are listening on will be different to
			// the current session.
			s.RLock()
			sameConnection := s.wsConn == wsConn
			s.RUnlock()

			if sameConnection {

				s.log(LogError, "error reading from gateway %s websocket, %s", s.gateway, err)
				// There has been an error reading, close the websocket so that
				// OnDisconnect event is emitted. Not Close(): closing with
				// 1000/1001 invalidates the session, so the RESUME that
				// reconnect() sends next would be refused.
				err := s.CloseWithCode(websocket.CloseServiceRestart)
				if err != nil {
					s.log(LogWarning, "error closing session connection, %s", err)
				}

				s.log(LogError, "calling gateway reconnect() now")
				s.reconnect()
			}

			return
		}

		select {

		case <-listening:
			return

		default:
			s.onEvent(messageType, message, false)

		}
	}
}

type heartbeatOp struct {
	Op   int   `json:"op"`
	Data int64 `json:"d"`
}

type helloOp struct {
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
}

// FailedHeartbeatAcks is the Number of heartbeat intervals to wait until forcing a connection restart.
const FailedHeartbeatAcks time.Duration = 5 * time.Millisecond

// handshakeReadTimeout bounds the reads Open() does while holding s.Lock().
// A variable so tests can shorten it.
var handshakeReadTimeout = 30 * time.Second

// discardSession forgets the gateway session so the next Open() identifies
// instead of resuming. The caller must hold s.Lock().
func (s *Session) discardSession() {
	s.resumeGatewayURL = ""
	s.sessionID = ""
	atomic.StoreInt64(s.sequence, 0)
}

// HeartbeatLatency returns the latency between heartbeat acknowledgement and heartbeat send.
func (s *Session) HeartbeatLatency() time.Duration {

	return s.LastHeartbeatAck.Sub(s.LastHeartbeatSent)

}

// heartbeat sends regular heartbeats to Discord so it knows the client
// is still connected.  If you do not send these heartbeats Discord will
// disconnect the websocket connection after a few seconds.
func (s *Session) heartbeat(wsConn *websocket.Conn, listening <-chan interface{}, heartbeatIntervalMsec time.Duration) {

	s.log(LogInformational, "called")

	if listening == nil || wsConn == nil {
		return
	}

	var err error
	ticker := time.NewTicker(heartbeatIntervalMsec * time.Millisecond)
	defer ticker.Stop()

	for {
		s.RLock()
		last := s.LastHeartbeatAck
		s.RUnlock()
		sequence := atomic.LoadInt64(s.sequence)
		s.log(LogDebug, "sending gateway websocket heartbeat seq %d", sequence)
		s.wsMutex.Lock()
		s.LastHeartbeatSent = time.Now().UTC()
		err = wsConn.WriteJSON(heartbeatOp{1, sequence})
		s.wsMutex.Unlock()
		if err != nil || time.Now().UTC().Sub(last) > (heartbeatIntervalMsec*FailedHeartbeatAcks) {
			if err != nil {
				s.log(LogError, "error sending heartbeat to gateway %s, %s", s.gateway, err)
			} else {
				s.log(LogError, "haven't gotten a heartbeat ACK in %v, triggering a reconnection", time.Now().UTC().Sub(last))
			}
			// Keep the session resumable (see listen).
			s.CloseWithCode(websocket.CloseServiceRestart)
			s.reconnect()
			return
		}
		s.Lock()
		s.DataReady = true
		s.Unlock()

		select {
		case <-ticker.C:
			// continue loop and send heartbeat
		case <-listening:
			return
		}
	}
}

// UpdateStatusData is provided to UpdateStatusComplex()
type UpdateStatusData struct {
	IdleSince  *int        `json:"since"`
	Activities []*Activity `json:"activities"`
	AFK        bool        `json:"afk"`
	Status     string      `json:"status"`
}

type updateStatusOp struct {
	Op   int              `json:"op"`
	Data UpdateStatusData `json:"d"`
}

func newUpdateStatusData(idle int, activityType ActivityType, name, url string) *UpdateStatusData {
	usd := &UpdateStatusData{
		Status: "online",
	}

	if idle > 0 {
		usd.IdleSince = &idle
	}

	if name != "" {
		usd.Activities = []*Activity{{
			Name: name,
			Type: activityType,
			URL:  url,
		}}
	}

	return usd
}

// UpdateGameStatus is used to update the user's status.
// If idle>0 then set status to idle.
// If name!="" then set game.
// if otherwise, set status to active, and no activity.
func (s *Session) UpdateGameStatus(idle int, name string) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(idle, ActivityTypeGame, name, ""))
}

// UpdateWatchStatus is used to update the user's watch status.
// If idle>0 then set status to idle.
// If name!="" then set movie/stream.
// if otherwise, set status to active, and no activity.
func (s *Session) UpdateWatchStatus(idle int, name string) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(idle, ActivityTypeWatching, name, ""))
}

// UpdateStreamingStatus is used to update the user's streaming status.
// If idle>0 then set status to idle.
// If name!="" then set game.
// If name!="" and url!="" then set the status type to streaming with the URL set.
// if otherwise, set status to active, and no game.
func (s *Session) UpdateStreamingStatus(idle int, name string, url string) (err error) {
	gameType := ActivityTypeGame
	if url != "" {
		gameType = ActivityTypeStreaming
	}
	return s.UpdateStatusComplex(*newUpdateStatusData(idle, gameType, name, url))
}

// UpdateListeningStatus is used to set the user to "Listening to..."
// If name!="" then set to what user is listening to
// Else, set user to active and no activity.
func (s *Session) UpdateListeningStatus(name string) (err error) {
	return s.UpdateStatusComplex(*newUpdateStatusData(0, ActivityTypeListening, name, ""))
}

// UpdateCustomStatus is used to update the user's custom status.
// If state!="" then set the custom status.
// Else, set user to active and remove the custom status.
func (s *Session) UpdateCustomStatus(state string) (err error) {
	data := UpdateStatusData{
		Status: "online",
	}

	if state != "" {
		// Discord requires a non-empty activity name, therefore we provide "Custom Status" as a placeholder.
		data.Activities = []*Activity{{
			Name:  "Custom Status",
			Type:  ActivityTypeCustom,
			State: state,
		}}
	}

	return s.UpdateStatusComplex(data)
}

// UpdateStatusComplex allows for sending the raw status update data untouched by discordgo.
func (s *Session) UpdateStatusComplex(usd UpdateStatusData) (err error) {
	// The comment does say "untouched by discordgo", but we might need to lie a bit here.
	// The Discord documentation lists `activities` as being nullable, but in practice this
	// doesn't seem to be the case. I had filed an issue about this at
	// https://github.com/discord/discord-api-docs/issues/2559, but as of writing this
	// haven't had any movement on it, so at this point I'm assuming this is an error,
	// and am fixing this bug accordingly. Because sending `null` for `activities` instantly
	// disconnects us, I think that disallowing it from being sent in `UpdateStatusComplex`
	// isn't that big of an issue.
	if usd.Activities == nil {
		usd.Activities = make([]*Activity, 0)
	}

	s.RLock()
	defer s.RUnlock()
	if s.wsConn == nil {
		return ErrWSNotFound
	}

	s.wsMutex.Lock()
	err = s.wsConn.WriteJSON(updateStatusOp{3, usd})
	s.wsMutex.Unlock()

	return
}

type requestGuildMembersData struct {
	// TODO: Deprecated. Use string instead of []string
	GuildIDs  []string  `json:"guild_id"`
	Query     *string   `json:"query,omitempty"`
	UserIDs   *[]string `json:"user_ids,omitempty"`
	Limit     int       `json:"limit"`
	Nonce     string    `json:"nonce,omitempty"`
	Presences bool      `json:"presences"`
}

type requestGuildMembersOp struct {
	Op   int                     `json:"op"`
	Data requestGuildMembersData `json:"d"`
}

// RequestGuildMembers requests guild members from the gateway
// The gateway responds with GuildMembersChunk events
// guildID   : Single Guild ID to request members of
// query     : String that username starts with, leave empty to return all members
// limit     : Max number of items to return, or 0 to request all members matched
// nonce     : Nonce to identify the Guild Members Chunk response
// presences : Whether to request presences of guild members
func (s *Session) RequestGuildMembers(guildID, query string, limit int, nonce string, presences bool) error {
	return s.RequestGuildMembersBatch([]string{guildID}, query, limit, nonce, presences)
}

// RequestGuildMembersList requests guild members from the gateway
// The gateway responds with GuildMembersChunk events
// guildID   : Single Guild ID to request members of
// userIDs   : IDs of users to fetch
// limit     : Max number of items to return, or 0 to request all members matched
// nonce     : Nonce to identify the Guild Members Chunk response
// presences : Whether to request presences of guild members
func (s *Session) RequestGuildMembersList(guildID string, userIDs []string, limit int, nonce string, presences bool) error {
	return s.RequestGuildMembersBatchList([]string{guildID}, userIDs, limit, nonce, presences)
}

// RequestGuildMembersBatch requests guild members from the gateway
// The gateway responds with GuildMembersChunk events
// guildID   : Slice of guild IDs to request members of
// query     : String that username starts with, leave empty to return all members
// limit     : Max number of items to return, or 0 to request all members matched
// nonce     : Nonce to identify the Guild Members Chunk response
// presences : Whether to request presences of guild members
//
// NOTE: this function is deprecated, please use RequestGuildMembers instead
func (s *Session) RequestGuildMembersBatch(guildIDs []string, query string, limit int, nonce string, presences bool) (err error) {
	data := requestGuildMembersData{
		GuildIDs:  guildIDs,
		Query:     &query,
		Limit:     limit,
		Nonce:     nonce,
		Presences: presences,
	}
	err = s.requestGuildMembers(data)
	return
}

// RequestGuildMembersBatchList requests guild members from the gateway
// The gateway responds with GuildMembersChunk events
// guildID   : Slice of guild IDs to request members of
// userIDs   : IDs of users to fetch
// limit     : Max number of items to return, or 0 to request all members matched
// nonce     : Nonce to identify the Guild Members Chunk response
// presences : Whether to request presences of guild members
//
// NOTE: this function is deprecated, please use RequestGuildMembersList instead
func (s *Session) RequestGuildMembersBatchList(guildIDs []string, userIDs []string, limit int, nonce string, presences bool) (err error) {
	data := requestGuildMembersData{
		GuildIDs:  guildIDs,
		UserIDs:   &userIDs,
		Limit:     limit,
		Nonce:     nonce,
		Presences: presences,
	}
	err = s.requestGuildMembers(data)
	return
}

// GatewayWriteStruct allows for sending raw gateway structs over the gateway.
func (s *Session) GatewayWriteStruct(data interface{}) (err error) {
	s.RLock()
	defer s.RUnlock()
	if s.wsConn == nil {
		return ErrWSNotFound
	}

	s.wsMutex.Lock()
	err = s.wsConn.WriteJSON(data)
	s.wsMutex.Unlock()

	return err
}

func (s *Session) requestGuildMembers(data requestGuildMembersData) (err error) {
	s.log(LogInformational, "called")

	s.RLock()
	defer s.RUnlock()
	if s.wsConn == nil {
		return ErrWSNotFound
	}

	s.wsMutex.Lock()
	err = s.wsConn.WriteJSON(requestGuildMembersOp{8, data})
	s.wsMutex.Unlock()

	return
}

// onEvent is the "event handler" for all messages received on the
// Discord Gateway API websocket connection.
//
// If you use the AddHandler() function to register a handler for a
// specific event this function will pass the event along to that handler.
//
// If you use the AddHandler() function to register a handler for the
// "OnEvent" event then all events will be passed to that handler.
//
// handshake is true when called from Open(), which holds s.Lock() for the
// whole handshake: nothing on that path may take the Session lock or close
// and reconnect (sync.RWMutex is not reentrant).
func (s *Session) onEvent(messageType int, message []byte, handshake bool) (*Event, error) {

	var err error
	var reader io.Reader
	reader = bytes.NewBuffer(message)

	// If this is a compressed message, uncompress it.
	if messageType == websocket.BinaryMessage {

		z, err2 := zlib.NewReader(reader)
		if err2 != nil {
			s.log(LogError, "error uncompressing websocket message, %s", err)
			return nil, err2
		}

		defer func() {
			err3 := z.Close()
			if err3 != nil {
				s.log(LogWarning, "error closing zlib, %s", err)
			}
		}()

		reader = z
	}

	// Decode the event into an Event struct.
	var e *Event
	decoder := json.NewDecoder(reader)
	if err = decoder.Decode(&e); err != nil {
		s.log(LogError, "error decoding websocket message, %s", err)
		return e, err
	}

	s.log(LogDebug, "Op: %d, Seq: %d, Type: %s, Data: %s\n\n", e.Operation, e.Sequence, e.Type, string(e.RawData))

	// Ping request.
	// Must respond with a heartbeat packet within 5 seconds
	if e.Operation == 1 {
		s.log(LogInformational, "sending heartbeat in response to Op1")
		s.wsMutex.Lock()
		err = s.wsConn.WriteJSON(heartbeatOp{1, atomic.LoadInt64(s.sequence)})
		s.wsMutex.Unlock()
		if err != nil {
			s.log(LogError, "error sending heartbeat in response to Op1")
			return e, err
		}

		return e, nil
	}

	// Reconnect
	// Must immediately disconnect from gateway and reconnect to new gateway.
	if e.Operation == 7 {
		if handshake {
			// Open() fails the attempt; reconnect() retries.
			s.log(LogError, "gateway sent Op7 during the handshake")
			return e, nil
		}
		s.log(LogError, "Closing and reconnecting in response to Op7")
		s.CloseWithCode(websocket.CloseServiceRestart)
		s.reconnect()
		return e, nil
	}

	// Invalid Session
	// Must respond with a Identify packet.
	if e.Operation == 9 {
		var resumable bool
		if err := json.Unmarshal(e.RawData, &resumable); err != nil {
			// Treat as non-resumable rather than bailing out: returning here
			// used to leave a closed socket that nobody reconnected.
			s.log(LogError, "error unmarshalling invalid session event, %s", err)
		}
		s.log(LogError, "gateway sent Op9 Invalid Session (resumable=%v, handshake=%v)", resumable, handshake)

		if handshake {
			// Open() holds s.Lock(), so the session can be reset directly; it
			// then fails the attempt and reconnect() retries.
			if !resumable {
				s.discardSession()
			}
			return e, nil
		}

		s.CloseWithCode(websocket.CloseServiceRestart)
		if !resumable {
			s.log(LogInformational, "Gateway session is not resumable, discarding its information")
			s.Lock()
			s.discardSession()
			s.Unlock()
		}

		// Discord asks clients to wait a random 1-5 s before re-identifying.
		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Second)
		s.reconnect()
		return e, nil
	}

	if e.Operation == 10 {
		// Op10 is handled by Open()
		return e, nil
	}

	if e.Operation == 11 {
		if handshake {
			s.LastHeartbeatAck = time.Now().UTC() // Open() holds s.Lock()
		} else {
			s.Lock()
			s.LastHeartbeatAck = time.Now().UTC()
			s.Unlock()
		}
		s.log(LogDebug, "got heartbeat ACK")
		return e, nil
	}

	// Do not try to Dispatch a non-Dispatch Message
	if e.Operation != 0 {
		// But we probably should be doing something with them.
		// TEMP
		s.log(LogWarning, "unknown Op: %d, Seq: %d, Type: %s, Data: %s, message: %s", e.Operation, e.Sequence, e.Type, string(e.RawData), string(message))
		return e, nil
	}

	// Store the message sequence
	atomic.StoreInt64(s.sequence, e.Sequence)

	// Map event to registered event handlers and pass it along to any registered handlers.
	if eh, ok := registeredInterfaceProviders[e.Type]; ok {
		e.Struct = eh.New()

		// Attempt to unmarshal our event.
		if err = json.Unmarshal(e.RawData, e.Struct); err != nil {
			s.log(LogError, "error unmarshalling %s event, %s", e.Type, err)
		}

		// Send event to any registered event handlers for it's type.
		// Because the above doesn't cancel this, in case of an error
		// the struct could be partially populated or at default values.
		// However, most errors are due to a single field and I feel
		// it's better to pass along what we received than nothing at all.
		// TODO: Think about that decision :)
		// Either way, READY events must fire, even with errors.
		s.handleEvent(e.Type, e.Struct)
	} else {
		s.log(LogWarning, "unknown event: Op: %d, Seq: %d, Type: %s, Data: %s", e.Operation, e.Sequence, e.Type, string(e.RawData))
	}

	// For legacy reasons, we send the raw event also, this could be useful for handling unknown events.
	s.handleEvent(eventEventType, e)

	return e, nil
}

// ------------------------------------------------------------------------------------------------
// Code related to voice connections that initiate over the data websocket
// ------------------------------------------------------------------------------------------------

type voiceChannelJoinData struct {
	GuildID   *string `json:"guild_id"`
	ChannelID *string `json:"channel_id"`
	SelfMute  bool    `json:"self_mute"`
	SelfDeaf  bool    `json:"self_deaf"`
}

type voiceChannelJoinOp struct {
	Op   int                  `json:"op"`
	Data voiceChannelJoinData `json:"d"`
}

// ChannelVoiceJoin joins the session user to a voice channel.
//
//	gID     : Guild ID of the channel to join.
//	cID     : Channel ID of the channel to join.
//	mute    : If true, you will be set to muted upon joining.
//	deaf    : If true, you will be set to deafened upon joining.
func (s *Session) ChannelVoiceJoin(gID, cID string, mute, deaf bool) (voice *VoiceConnection, err error) {

	s.log(LogInformational, "called")

	s.RLock()
	voice, _ = s.VoiceConnections[gID]
	s.RUnlock()

	created := false
	if voice == nil {
		voice = &VoiceConnection{}
		s.Lock()
		s.VoiceConnections[gID] = voice
		s.Unlock()
		created = true
	}

	voice.Lock()
	voice.GuildID = gID
	voice.ChannelID = cID
	voice.deaf = deaf
	voice.mute = mute
	voice.session = s
	voice.Unlock()

	err = s.ChannelVoiceJoinManual(gID, cID, mute, deaf)
	if err != nil {
		// No OP4 went out, so just unregister the entry this call created;
		// left behind, the next gateway reconnect would "restore" it and join
		// the channel with no app-side instance (a ghost bot).
		if created {
			s.Lock()
			if s.VoiceConnections[gID] == voice {
				delete(s.VoiceConnections, gID)
			}
			s.Unlock()
		}
		return
	}

	// doesn't exactly work perfect yet.. TODO
	err = voice.waitUntilConnected()
	if err != nil {
		s.log(LogWarning, "error waiting for voice to connect, %s", err)
		// Disconnect (not just Close): the OP4 join was already sent, so the
		// gateway shows the bot inside the channel. Close() alone would leave
		// a silent ghost there; Disconnect() also clears the gateway voice
		// state and unregisters the connection so a late VOICE_SERVER_UPDATE
		// cannot half-revive it.
		voice.Disconnect()
		return
	}

	return
}

// ChannelVoiceJoinManual initiates a voice session to a voice channel, but does not complete it.
//
// This should only be used when the VoiceServerUpdate will be intercepted and used elsewhere.
//
//	gID     : Guild ID of the channel to join.
//	cID     : Channel ID of the channel to join, leave empty to disconnect.
//	mute    : If true, you will be set to muted upon joining.
//	deaf    : If true, you will be set to deafened upon joining.
func (s *Session) ChannelVoiceJoinManual(gID, cID string, mute, deaf bool) (err error) {

	s.log(LogInformational, "called")

	var channelID *string
	if cID == "" {
		channelID = nil
	} else {
		channelID = &cID
	}

	// Send the request to Discord that we want to join the voice channel.
	// wsConn is nil while the gateway reconnects; writing to it would panic
	// with wsMutex held and wedge every later gateway write.
	s.RLock()
	gwConn := s.wsConn
	s.RUnlock()
	if gwConn == nil {
		return ErrWSNotFound
	}
	data := voiceChannelJoinOp{4, voiceChannelJoinData{&gID, channelID, mute, deaf}}
	s.wsMutex.Lock()
	err = gwConn.WriteJSON(data)
	s.wsMutex.Unlock()
	return
}

// onVoiceStateUpdate handles Voice State Update events on the data websocket.
func (s *Session) onVoiceStateUpdate(st *VoiceStateUpdate) {

	// If we don't have a connection for the channel, don't bother
	if st.ChannelID == "" {
		return
	}

	// Check if we have a voice connection to update
	s.RLock()
	voice, exists := s.VoiceConnections[st.GuildID]
	s.RUnlock()
	if !exists {
		return
	}

	// We only care about events that are about us.
	if s.State.User.ID != st.UserID {
		return
	}

	// Store the SessionID for later use.
	voice.Lock()
	voice.UserID = st.UserID
	voice.sessionID = st.SessionID
	voice.ChannelID = st.ChannelID
	voice.Unlock()
}

// onVoiceServerUpdate handles the Voice Server Update data websocket event.
//
// This is also fired if the Guild's voice region changes while connected
// to a voice channel.  In that case, need to re-establish connection to
// the new region endpoint.
func (s *Session) onVoiceServerUpdate(st *VoiceServerUpdate) {

	s.log(LogInformational, "called")

	s.RLock()
	voice, exists := s.VoiceConnections[st.GuildID]
	s.RUnlock()

	// If no VoiceConnection exists, just skip this
	if !exists {
		return
	}

	// If currently connected to voice ws/udp, then disconnect.
	// Has no effect if not connected.
	s.log(LogError, "voice server update for guild %s: reopening (endpoint %s)", st.GuildID, st.Endpoint)
	voice.Close()

	// Store values for later use
	voice.Lock()
	voice.token = st.Token
	voice.endpoint = st.Endpoint
	voice.GuildID = st.GuildID
	voice.Unlock()

	// A null endpoint means the voice server is being reallocated; per the
	// docs we must wait for the follow-up VOICE_SERVER_UPDATE that carries
	// the new endpoint (it re-enters this handler) instead of dialing.
	if st.Endpoint == "" {
		s.log(LogError, "onVoiceServerUpdate: null endpoint, waiting for voice server reallocation")
		return
	}

	// Open a connection to the voice server
	err := voice.open()
	if err != nil {
		s.log(LogError, "onVoiceServerUpdate voice.open, %s", err)
		// Don't leave a zombie voice state behind (the bot would appear to be
		// in the channel with no audio until Discord drops it — a visible
		// leave). Retry via reconnect: it re-joins invisibly and only resets
		// with a visible disconnect as a last resort.
		go voice.reconnect()
	}
}

type identifyOp struct {
	Op   int      `json:"op"`
	Data Identify `json:"d"`
}

// identify sends the identify packet to the gateway
func (s *Session) identify() error {
	s.log(LogDebug, "called")

	// TODO: This is a temporary block of code to help
	// maintain backwards compatibility
	if s.Compress == false {
		s.Identify.Compress = false
	}

	// TODO: This is a temporary block of code to help
	// maintain backwards compatibility
	if s.Token != "" && s.Identify.Token == "" {
		s.Identify.Token = s.Token
	}

	// TODO: Below block should be refactored so ShardID and ShardCount
	// can be deprecated and their usage moved to the Session.Identify
	// struct
	if s.ShardCount > 1 {

		if s.ShardID >= s.ShardCount {
			return ErrWSShardBounds
		}

		s.Identify.Shard = &[2]int{s.ShardID, s.ShardCount}
	}

	// Send Identify packet to Discord
	op := identifyOp{2, s.Identify}
	s.log(LogDebug, "Identify Packet: \n%#v", op)
	s.wsMutex.Lock()
	err := s.wsConn.WriteJSON(op)
	s.wsMutex.Unlock()

	return err
}

func (s *Session) reconnect() {

	s.log(LogInformational, "called")

	var err error

	if s.ShouldReconnectOnError {

		wait := time.Duration(1)

		for {
			s.log(LogError, "trying to reconnect to gateway")

			s.RLock()
			prevSessionID := s.sessionID
			s.RUnlock()

			err = s.Open()
			if err == nil {
				s.RLock()
				resumed := prevSessionID != "" && s.sessionID == prevSessionID
				s.RUnlock()
				s.log(LogError, "successfully reconnected to gateway (session resumed=%v)", resumed)

				if s.ShouldReconnectVoiceOnSessionError {
					// Snapshot, then release the lock before touching the
					// connections: holding s.RLock across v.isReady() and the
					// sleeps below lets one wedged VoiceConnection block every
					// Session writer (heartbeat, Op11 ACK) forever.
					s.RLock()
					voices := make([]*VoiceConnection, 0, len(s.VoiceConnections))
					for _, v := range s.VoiceConnections {
						voices = append(voices, v)
					}
					s.RUnlock()
					for _, v := range voices {

						// After a successful gateway RESUME, the server-side
						// voice state and the (independent) voice websocket
						// are both still intact. Reconnecting would tear down
						// a healthy session — and the same-channel re-join
						// elicits no VOICE_SERVER_UPDATE (no state change), so
						// that path can only recover via a VISIBLE
						// leave/rejoin reset. Leave healthy connections alone;
						// a genuinely broken voice socket recovers through its
						// own error path (resume/recoverAfterClose).
						if resumed && v.isReady() {
							s.log(LogError, "gateway resumed; voice connection for guild %s is healthy, leaving it alone", v.GuildID)
							continue
						}

						s.log(LogError, "reconnecting voice connection to guild %s", v.GuildID)
						go v.reconnect()

						// This is here just to prevent violently spamming the
						// voice reconnects
						time.Sleep(1 * time.Second)
					}
				}
				return
			}

			// Certain race conditions can call reconnect() twice. If this happens, we
			// just break out of the reconnect loop
			if err == ErrWSAlreadyOpen {
				s.log(LogInformational, "Websocket already exists, no need to reconnect")
				return
			}

			s.log(LogError, "error reconnecting to gateway, %s", err)

			// Jitter keeps the first retry after an Op9 within Discord's
			// 1-5 s guidance and spreads out retry bursts.
			<-time.After(wait*time.Second + time.Duration(rand.Int63n(int64(4*time.Second))))
			wait *= 2
			if wait > 600 {
				wait = 600
			}
		}
	}
}

// Close closes a websocket and stops all listening/heartbeat goroutines.
// TODO: Add support for Voice WS/UDP
func (s *Session) Close() error {
	return s.CloseWithCode(websocket.CloseNormalClosure)
}

// CloseWithCode closes a websocket using the provided closeCode and stops all
// listening/heartbeat goroutines.
// TODO: Add support for Voice WS/UDP connections
func (s *Session) CloseWithCode(closeCode int) (err error) {

	s.log(LogInformational, "called")
	s.Lock()

	s.DataReady = false

	if s.listening != nil {
		s.log(LogInformational, "closing listening channel")
		close(s.listening)
		s.listening = nil
	}

	// TODO: Close all active Voice Connections too
	// this should force stop any reconnecting voice channels too

	if s.wsConn != nil {

		s.log(LogInformational, "sending close frame")
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		s.wsMutex.Lock()
		err := s.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, ""))
		s.wsMutex.Unlock()
		if err != nil {
			s.log(LogInformational, "error closing websocket, %s", err)
		}

		// TODO: Wait for Discord to actually close the connection.
		time.Sleep(1 * time.Second)

		s.log(LogInformational, "closing gateway websocket")
		err = s.wsConn.Close()
		if err != nil {
			s.log(LogInformational, "error closing websocket, %s", err)
		}

		s.wsConn = nil
	}

	s.Unlock()

	s.log(LogInformational, "emit disconnect event")
	s.handleEvent(disconnectEventType, &Disconnect{})

	return
}
