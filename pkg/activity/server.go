package activity

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/OGA45/gomatalk/pkg/config"
)

// Server owns the activity HTTP listener and its shared dependencies (token/
// guild authenticator, rate limiter, Discord HTTP client).
type Server struct {
	httpServer   *http.Server
	auth         *authenticator
	limiter      *rateLimiter
	httpClient   *http.Client
	clientID     string
	clientSecret string
}

// Start builds the activity server from the current config and launches it in a
// background goroutine, returning immediately. Callers are expected to have
// checked config.O().Activity.Enabled. ListenAndServe failures are logged.
func Start() (*Server, error) {
	cfg := config.O().Activity
	listen := cfg.Listen
	if listen == "" {
		listen = ":8080"
	}

	// Shared client for all Discord calls (token exchange + token/guild verify).
	httpClient := &http.Client{Timeout: 10 * time.Second}

	s := &Server{
		auth:         newAuthenticator(cfg.ClientID, httpClient),
		limiter:      newRateLimiter(),
		httpClient:   httpClient,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	}

	webapp, err := webappHandler()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	// Static frontend; also handles "/". More specific /api/* patterns win.
	mux.Handle("/", webapp)

	mux.HandleFunc("GET /api/config", s.api(s.handleConfig))
	mux.HandleFunc("POST /api/token", s.api(s.handleToken))
	mux.HandleFunc("GET /api/me", s.api(s.handleMe))
	mux.HandleFunc("GET /api/voices", s.api(s.handleVoices))
	mux.HandleFunc("GET /api/me/voice", s.api(s.handleGetVoice))
	mux.HandleFunc("PUT /api/me/voice", s.api(s.handlePutVoice))
	mux.HandleFunc("POST /api/me/voice/random", s.api(s.handleRandomVoice))
	mux.HandleFunc("GET /api/guilds/{gid}/words", s.api(s.handleGetWords))
	mux.HandleFunc("POST /api/guilds/{gid}/words", s.api(s.handlePostWord))
	mux.HandleFunc("DELETE /api/guilds/{gid}/words/{word}", s.api(s.handleDeleteWord))

	s.httpServer = &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	go func() {
		log.Println("INFO: activity server listening on", listen)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("ERROR: activity server:", err)
		}
	}()

	return s, nil
}

// api wraps a JSON handler with the shared security and no-cache headers
// required on every /api/* response.
func (s *Server) api(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		h(w, r)
	}
}

// Shutdown gracefully stops the HTTP server. Safe to call on a nil Server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
