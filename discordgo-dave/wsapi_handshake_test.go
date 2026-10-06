package discordgo

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakeGateway is a minimal Discord gateway. Each accepted connection is
// handed to the next handler in order; extra connections are closed.
type fakeGateway struct {
	*httptest.Server
	url      string
	handlers chan func(*websocket.Conn)
}

func newFakeGateway(t *testing.T, handlers ...func(*websocket.Conn)) *fakeGateway {
	g := &fakeGateway{handlers: make(chan func(*websocket.Conn), len(handlers))}
	for _, h := range handlers {
		g.handlers <- h
	}
	up := websocket.Upgrader{}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		select {
		case h := <-g.handlers:
			h(c)
		default:
		}
	}))
	g.url = "ws" + strings.TrimPrefix(g.Server.URL, "http")
	t.Cleanup(g.Server.Close)
	return g
}

func sendHello(c *websocket.Conn, intervalMs int) {
	c.WriteMessage(websocket.TextMessage, []byte(`{"op":10,"d":{"heartbeat_interval":`+itoa(intervalMs)+`}}`))
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

// readOp returns the op code of the next client frame, skipping heartbeats.
func readOp(c *websocket.Conn) (int, error) {
	for {
		_, m, err := c.ReadMessage()
		if err != nil {
			return 0, err
		}
		var f struct {
			Op int `json:"op"`
		}
		if err := json.Unmarshal(m, &f); err != nil {
			return 0, err
		}
		if f.Op != 1 {
			return f.Op, nil
		}
	}
}

// closeCode reads until the client closes and returns its close code (0 if
// the connection dropped without a close frame).
func closeCode(c *websocket.Conn) int {
	_, err := readOp(c)
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return 0
}

// drain keeps the connection open until the client goes away, so the client
// never sees a server-side drop it would reconnect from.
func drain(c *websocket.Conn) {
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			return
		}
	}
}

func sendReady(g *fakeGateway, c *websocket.Conn, sessionID string) {
	c.WriteMessage(websocket.TextMessage, []byte(`{"op":0,"s":1,"t":"READY","d":{"v":10,"session_id":"`+
		sessionID+`","resume_gateway_url":"`+g.url+`","user":{"id":"1","username":"bot"},"guilds":[]}}`))
}

func sendResumed(c *websocket.Conn) {
	c.WriteMessage(websocket.TextMessage, []byte(`{"op":0,"s":43,"t":"RESUMED","d":{}}`))
}

func newTestSession(g *fakeGateway) *Session {
	s, _ := New("Bot test")
	s.gateway = g.url
	return s
}

// resumable puts s in the state left by an earlier session, so Open() RESUMEs.
func resumable(s *Session, g *fakeGateway) {
	s.sessionID = "old-session"
	atomic.StoreInt64(s.sequence, 42)
	s.resumeGatewayURL = g.url
}

func openWithTimeout(t *testing.T, s *Session, d time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.Open() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		t.Fatalf("Open() did not return within %v (deadlock)", d)
		return nil
	}
}

func stop(s *Session) {
	s.ShouldReconnectOnError = false
	s.Close()
}

// Regression: Open() holds s.Lock() while processing handshake frames, and
// an Op9 reply to RESUME used to call CloseWithCode() -> s.Lock() from there,
// deadlocking the session forever with no log output.
func TestOpenOp9AfterResumeFailsWithoutDeadlock(t *testing.T) {
	var gotOp int64
	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		op, _ := readOp(c)
		atomic.StoreInt64(&gotOp, int64(op))
		c.WriteMessage(websocket.TextMessage, []byte(`{"op":9,"d":false}`))
		drain(c)
	})
	s := newTestSession(g)
	resumable(s, g)

	if err := openWithTimeout(t, s, 5*time.Second); err == nil {
		t.Fatal("Open() succeeded, want an error for Op9")
	}
	if op := atomic.LoadInt64(&gotOp); op != 6 {
		t.Fatalf("client sent op %d, want 6 (RESUME)", op)
	}
	if s.sessionID != "" || s.resumeGatewayURL != "" || atomic.LoadInt64(s.sequence) != 0 {
		t.Fatalf("non-resumable session not discarded: id=%q url=%q seq=%d",
			s.sessionID, s.resumeGatewayURL, atomic.LoadInt64(s.sequence))
	}
	if s.wsConn != nil {
		t.Fatal("wsConn left open after a failed handshake")
	}
}

func TestOpenOp9ResumableKeepsSession(t *testing.T) {
	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		readOp(c)
		c.WriteMessage(websocket.TextMessage, []byte(`{"op":9,"d":true}`))
		drain(c)
	})
	s := newTestSession(g)
	resumable(s, g)

	if err := openWithTimeout(t, s, 5*time.Second); err == nil {
		t.Fatal("Open() succeeded, want an error for Op9")
	}
	if s.sessionID != "old-session" {
		t.Fatalf("resumable session discarded: id=%q", s.sessionID)
	}
}

func TestOpenOp9AfterIdentifyFailsWithoutDeadlock(t *testing.T) {
	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		readOp(c)
		c.WriteMessage(websocket.TextMessage, []byte(`{"op":9,"d":false}`))
		drain(c)
	})
	if err := openWithTimeout(t, newTestSession(g), 5*time.Second); err == nil {
		t.Fatal("Open() succeeded, want an error for Op9")
	}
}

func TestOpenOp7FailsWithoutDeadlock(t *testing.T) {
	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		readOp(c)
		c.WriteMessage(websocket.TextMessage, []byte(`{"op":7,"d":null}`))
		drain(c)
	})
	s := newTestSession(g)
	resumable(s, g)
	if err := openWithTimeout(t, s, 5*time.Second); err == nil {
		t.Fatal("Open() succeeded, want an error for Op7")
	}
}

// 4009 Session timed out answers a RESUME that can never succeed.
func TestOpenClose4009DiscardsSession(t *testing.T) {
	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		readOp(c)
		c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4009, "session timed out"))
		drain(c)
	})
	s := newTestSession(g)
	resumable(s, g)
	if err := openWithTimeout(t, s, 5*time.Second); err == nil {
		t.Fatal("Open() succeeded, want an error for close 4009")
	}
	if s.sessionID != "" {
		t.Fatalf("timed-out session not discarded: id=%q", s.sessionID)
	}
}

// A gateway that never answers must not hold s.Lock() forever.
func TestOpenHandshakeReadTimesOut(t *testing.T) {
	old := handshakeReadTimeout
	handshakeReadTimeout = 300 * time.Millisecond
	defer func() { handshakeReadTimeout = old }()

	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		drain(c) // swallow IDENTIFY, never send READY
	})
	if err := openWithTimeout(t, newTestSession(g), 5*time.Second); err == nil {
		t.Fatal("Open() succeeded against a silent gateway")
	}
}

func TestOpenResumed(t *testing.T) {
	g := newFakeGateway(t, func(c *websocket.Conn) {
		sendHello(c, 41250)
		readOp(c)
		sendResumed(c)
		drain(c)
	})
	s := newTestSession(g)
	resumable(s, g)
	if err := openWithTimeout(t, s, 5*time.Second); err != nil {
		t.Fatalf("Open() = %v, want nil", err)
	}
	stop(s)
}

// A live session that gets Op9 must close with 1012, wait, and identify afresh.
func TestOp9WhileConnectedReidentifies(t *testing.T) {
	var code int64
	identified := make(chan struct{})
	var g *fakeGateway
	g = newFakeGateway(t,
		func(c *websocket.Conn) {
			sendHello(c, 41250)
			readOp(c) // IDENTIFY
			sendReady(g, c, "sess-1")
			c.WriteMessage(websocket.TextMessage, []byte(`{"op":9,"d":false}`))
			atomic.StoreInt64(&code, int64(closeCode(c)))
		},
		func(c *websocket.Conn) {
			sendHello(c, 41250)
			if op, _ := readOp(c); op == 2 {
				close(identified)
			}
			sendReady(g, c, "sess-2")
			drain(c)
		},
	)
	s := newTestSession(g)
	if err := openWithTimeout(t, s, 5*time.Second); err != nil {
		t.Fatalf("Open() = %v", err)
	}
	defer stop(s)

	select {
	case <-identified:
	case <-time.After(15 * time.Second):
		t.Fatal("no fresh IDENTIFY after Op9 (d=false)")
	}
	if c := atomic.LoadInt64(&code); c != websocket.CloseServiceRestart {
		t.Fatalf("client close code = %d, want %d", c, websocket.CloseServiceRestart)
	}
}

// A server-side close must not be answered with 1000 (which invalidates the
// session); the reconnect must RESUME.
func TestServerCloseKeepsSessionResumable(t *testing.T) {
	var code int64
	resumed := make(chan struct{})
	var g *fakeGateway
	g = newFakeGateway(t,
		func(c *websocket.Conn) {
			sendHello(c, 41250)
			readOp(c) // IDENTIFY
			sendReady(g, c, "sess-1")
			c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4000, "unknown error"))
			atomic.StoreInt64(&code, int64(closeCode(c)))
		},
		func(c *websocket.Conn) {
			sendHello(c, 41250)
			if op, _ := readOp(c); op == 6 {
				close(resumed)
			}
			sendResumed(c)
			drain(c)
		},
	)
	s := newTestSession(g)
	if err := openWithTimeout(t, s, 5*time.Second); err != nil {
		t.Fatalf("Open() = %v", err)
	}
	defer stop(s)

	select {
	case <-resumed:
	case <-time.After(10 * time.Second):
		t.Fatal("no RESUME after a server-side close")
	}
	if c := atomic.LoadInt64(&code); c != websocket.CloseServiceRestart {
		t.Fatalf("client close code = %d, want %d", c, websocket.CloseServiceRestart)
	}
}

// Missing heartbeat ACKs must also close with 1012 and RESUME.
func TestHeartbeatTimeoutKeepsSessionResumable(t *testing.T) {
	var code int64
	resumed := make(chan struct{})
	var g *fakeGateway
	g = newFakeGateway(t,
		func(c *websocket.Conn) {
			sendHello(c, 20) // ACK deadline = 20 x FailedHeartbeatAcks = 100 ms
			readOp(c)        // IDENTIFY
			sendReady(g, c, "sess-1")
			atomic.StoreInt64(&code, int64(closeCode(c))) // never ACK
		},
		func(c *websocket.Conn) {
			sendHello(c, 41250)
			if op, _ := readOp(c); op == 6 {
				close(resumed)
			}
			sendResumed(c)
			drain(c)
		},
	)
	s := newTestSession(g)
	if err := openWithTimeout(t, s, 5*time.Second); err != nil {
		t.Fatalf("Open() = %v", err)
	}
	defer stop(s)

	select {
	case <-resumed:
	case <-time.After(10 * time.Second):
		t.Fatal("no RESUME after heartbeat ACK timeout")
	}
	if c := atomic.LoadInt64(&code); c != websocket.CloseServiceRestart {
		t.Fatalf("client close code = %d, want %d", c, websocket.CloseServiceRestart)
	}
}

// A join attempted while the gateway is down must fail cleanly and leave no
// VoiceConnection behind for the next reconnect to "restore".
func TestChannelVoiceJoinWithoutGatewayLeavesNoGhost(t *testing.T) {
	s, _ := New("Bot test")
	s.VoiceConnections = map[string]*VoiceConnection{}
	if _, err := s.ChannelVoiceJoin("g1", "c1", false, false); err != ErrWSNotFound {
		t.Fatalf("ChannelVoiceJoin() = %v, want ErrWSNotFound", err)
	}
	if _, ok := s.VoiceConnections["g1"]; ok {
		t.Fatal("failed join left a VoiceConnection registered")
	}
}
