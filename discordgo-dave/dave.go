package discordgo

import (
	"log/slog"
	"sync"

	"github.com/disgoorg/godave"
	"github.com/disgoorg/godave/golibdave"
)

// daveSession wraps a godave.Session (backed by Discord's official libdave via
// cgo) with a mutex. libdave sessions are not safe for concurrent use: the
// encryptor is mutated by the DAVE transition handlers on the websocket
// goroutine while opusSender encrypts frames on another goroutine, so every
// call is serialized here.
//
// Unlike the previous hand-rolled joiner-only MLS, libdave processes MLS
// Commits in place (OnDaveMLSPrepareCommitTransition -> ProcessCommit), so
// epoch transitions from members joining/leaving re-key seamlessly with no
// voice reconnect and no audible gap.
type daveSession struct {
	mu sync.Mutex
	s  godave.Session
}

func newDaveSession(userID string, cb godave.Callbacks) *daveSession {
	return &daveSession{s: golibdave.NewSession(slog.Default(), godave.UserID(userID), cb)}
}

func (d *daveSession) setChannelID(id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.SetChannelID(godave.ChannelID(id))
}

func (d *daveSession) assignSSRC(ssrc uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.AssignSsrcToCodec(ssrc, godave.CodecOpus)
}

func (d *daveSession) selectProtocolAck(version uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnSelectProtocolAck(version)
}

func (d *daveSession) prepareTransition(transitionID, version uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDavePrepareTransition(transitionID, version)
}

func (d *daveSession) executeTransition(transitionID uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDaveExecuteTransition(transitionID)
}

func (d *daveSession) prepareEpoch(epoch int, version uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDavePrepareEpoch(epoch, version)
}

func (d *daveSession) externalSenderPackage(pkg []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDaveMLSExternalSenderPackage(pkg)
}

func (d *daveSession) proposals(p []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDaveMLSProposals(p)
}

func (d *daveSession) commit(transitionID uint16, msg []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDaveMLSPrepareCommitTransition(transitionID, msg)
}

func (d *daveSession) welcome(transitionID uint16, msg []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.OnDaveMLSWelcome(transitionID, msg)
}

func (d *daveSession) addUser(userID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.AddUser(godave.UserID(userID))
}

func (d *daveSession) removeUser(userID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.s.RemoveUser(godave.UserID(userID))
}

// encrypt returns the DAVE-encrypted frame. While the session is in passthrough
// mode (before an epoch is active), libdave copies the frame through unchanged.
func (d *daveSession) encrypt(ssrc uint32, frame []byte) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]byte, d.s.MaxEncryptedFrameSize(len(frame)))
	n, err := d.s.Encrypt(ssrc, frame, out)
	if err != nil {
		return nil, err
	}
	return out[:n], nil
}

// daveCallbacks routes godave's outgoing MLS messages to the voice websocket.
// godave invokes these synchronously from within the On* handlers (while the
// daveSession mutex is held); they only take v.wsMutex, so the lock order is
// consistently daveSession.mu -> wsMutex and cannot deadlock.
type daveCallbacks struct {
	v *VoiceConnection
}

func (c daveCallbacks) SendMLSKeyPackage(mlsKeyPackage []byte) error {
	c.v.sendDAVEBinary(26, mlsKeyPackage)
	return nil
}

func (c daveCallbacks) SendMLSCommitWelcome(mlsCommitWelcome []byte) error {
	c.v.sendDAVEBinary(28, mlsCommitWelcome)
	return nil
}

func (c daveCallbacks) SendReadyForTransition(transitionID uint16) error {
	c.v.sendDAVEReadyForTransition(transitionID)
	return nil
}

func (c daveCallbacks) SendInvalidCommitWelcome(transitionID uint16) error {
	c.v.sendDAVEInvalidCommitWelcome(transitionID)
	return nil
}
