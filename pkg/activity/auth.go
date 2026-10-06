package activity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

const discordAPIBase = "https://discord.com/api/v10"

// maxDiscordBody caps how much of an upstream Discord response we read, so a
// hostile or malfunctioning upstream cannot exhaust memory.
const maxDiscordBody = 1 << 20 // 1 MiB

var (
	// errUnauthorized -> 401 unauthorized (bad/expired token, wrong app).
	errUnauthorized = errors.New("unauthorized")
	// errUpstreamRateLimited -> 503 upstream_rate_limited (Discord 429 or unreachable).
	errUpstreamRateLimited = errors.New("upstream rate limited")
)

// discordUser is the subset of the Discord user object exposed to the frontend.
type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
}

type tokenCacheEntry struct {
	user      discordUser
	expiresAt time.Time
	// invalid marks a negative entry: the token failed verification recently.
	// Without it, a client looping with a garbage token would relay every
	// request to Discord's API (unauthenticated GETs have no rate limit).
	invalid bool
}

type guildCacheEntry struct {
	guilds    map[string]bool
	expiresAt time.Time
}

// authenticator verifies Discord user tokens against oauth2/@me and resolves
// guild membership, caching both keyed by sha256(token) so the raw token is
// never used as a map key or logged.
type authenticator struct {
	clientID string
	client   *http.Client

	mu         sync.Mutex
	tokenCache map[string]tokenCacheEntry
	guildCache map[string]guildCacheEntry
	writes     int
}

func newAuthenticator(clientID string, client *http.Client) *authenticator {
	return &authenticator{
		clientID:   clientID,
		client:     client,
		tokenCache: map[string]tokenCacheEntry{},
		guildCache: map[string]guildCacheEntry{},
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// verifyToken confirms the token was issued for this application and returns
// the associated user. Results are cached until min(token expiry, +10m).
func (a *authenticator) verifyToken(ctx context.Context, token string) (discordUser, error) {
	key := hashToken(token)
	now := time.Now()

	a.mu.Lock()
	if e, ok := a.tokenCache[key]; ok && now.Before(e.expiresAt) {
		a.mu.Unlock()
		if e.invalid {
			return discordUser{}, errUnauthorized
		}
		return e.user, nil
	}
	a.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordAPIBase+"/oauth2/@me", nil)
	if err != nil {
		return discordUser{}, errUnauthorized
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.client.Do(req)
	if err != nil {
		return discordUser{}, errUpstreamRateLimited
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return discordUser{}, errUpstreamRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return discordUser{}, a.rejectToken(key, now)
	}

	var body struct {
		Application struct {
			ID string `json:"id"`
		} `json:"application"`
		Expires string      `json:"expires"`
		User    discordUser `json:"user"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDiscordBody)).Decode(&body); err != nil {
		return discordUser{}, a.rejectToken(key, now)
	}
	// Reject tokens minted for any other application.
	if body.Application.ID != a.clientID || body.User.ID == "" {
		return discordUser{}, a.rejectToken(key, now)
	}

	ttlUntil := now.Add(10 * time.Minute)
	if exp, perr := time.Parse(time.RFC3339, body.Expires); perr == nil {
		if !now.Before(exp) {
			return discordUser{}, errUnauthorized // already expired
		}
		if exp.Before(ttlUntil) {
			ttlUntil = exp
		}
	}

	a.mu.Lock()
	a.pruneLocked(now)
	a.tokenCache[key] = tokenCacheEntry{user: body.User, expiresAt: ttlUntil}
	a.mu.Unlock()
	return body.User, nil
}

// rejectToken records a negative cache entry for a token that failed
// verification (not for transient upstream failures) and returns
// errUnauthorized. Keeps repeat garbage tokens from hammering Discord.
func (a *authenticator) rejectToken(key string, now time.Time) error {
	a.mu.Lock()
	a.pruneLocked(now)
	a.tokenCache[key] = tokenCacheEntry{invalid: true, expiresAt: now.Add(60 * time.Second)}
	a.mu.Unlock()
	return errUnauthorized
}

// guildMembership reports whether the token's user is a member of guildID.
// The user's full guild-id set is cached for 5 minutes.
func (a *authenticator) guildMembership(ctx context.Context, token, guildID string) (bool, error) {
	key := hashToken(token)
	now := time.Now()

	a.mu.Lock()
	if e, ok := a.guildCache[key]; ok && now.Before(e.expiresAt) {
		member := e.guilds[guildID]
		a.mu.Unlock()
		return member, nil
	}
	a.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordAPIBase+"/users/@me/guilds", nil)
	if err != nil {
		return false, errUnauthorized
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.client.Do(req)
	if err != nil {
		return false, errUpstreamRateLimited
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return false, errUpstreamRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return false, errUnauthorized
	}

	var guilds []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDiscordBody)).Decode(&guilds); err != nil {
		return false, errUnauthorized
	}
	set := make(map[string]bool, len(guilds))
	for _, g := range guilds {
		set[g.ID] = true
	}

	a.mu.Lock()
	a.pruneLocked(now)
	a.guildCache[key] = guildCacheEntry{guilds: set, expiresAt: now.Add(5 * time.Minute)}
	a.mu.Unlock()
	return set[guildID], nil
}

// pruneLocked drops expired cache entries. Called under a.mu, gated by a write
// counter so it stays amortized O(1). Lazy eviction keeps both maps bounded by
// the number of live tokens.
func (a *authenticator) pruneLocked(now time.Time) {
	a.writes++
	if a.writes%128 != 0 {
		return
	}
	for k, e := range a.tokenCache {
		if !now.Before(e.expiresAt) {
			delete(a.tokenCache, k)
		}
	}
	for k, e := range a.guildCache {
		if !now.Before(e.expiresAt) {
			delete(a.guildCache, k)
		}
	}
}
