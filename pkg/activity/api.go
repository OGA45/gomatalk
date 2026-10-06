package activity

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OGA45/gomatalk/pkg/db"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/model"
	"github.com/OGA45/gomatalk/pkg/voice"
)

const (
	maxJSONBody      = 4 << 10 // 4 KiB request-body cap
	maxWordsPerGuild = 500
	wordRuneMin      = 1
	wordRuneMax      = 64
)

// --- JSON wire types -------------------------------------------------------

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// userInfoJSON is the wire form of model.UserInfo with the lowercase keys the
// frontend expects (model.UserInfo carries no json tags, so a dedicated DTO
// keeps the persisted struct untouched).
type userInfoJSON struct {
	Voice     string  `json:"voice"`
	Speed     float64 `json:"speed"`
	Tone      float64 `json:"tone"`
	Intone    float64 `json:"intone"`
	Threshold float64 `json:"threshold"`
	AllPass   float64 `json:"allpass"`
	Volume    float64 `json:"volume"`
}

func toUserInfoJSON(ui model.UserInfo) userInfoJSON {
	return userInfoJSON{
		Voice:     ui.Voice,
		Speed:     ui.Speed,
		Tone:      ui.Tone,
		Intone:    ui.Intone,
		Threshold: ui.Threshold,
		AllPass:   ui.AllPass,
		Volume:    ui.Volume,
	}
}

func (u userInfoJSON) toModel() model.UserInfo {
	return model.UserInfo{
		Voice:     u.Voice,
		Speed:     u.Speed,
		Tone:      u.Tone,
		Intone:    u.Intone,
		Threshold: u.Threshold,
		AllPass:   u.AllPass,
		Volume:    u.Volume,
	}
}

type userInfoResponse struct {
	UserInfo userInfoJSON `json:"user_info"`
}

type wordEntry struct {
	Word    string `json:"word"`
	Reading string `json:"reading"`
}

type wordsResponse struct {
	Words []wordEntry `json:"words"`
	Count int         `json:"count"`
}

// --- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// clientIP resolves the caller's address for per-IP rate limiting. The bot sits
// behind the operator's reverse proxy, so X-Forwarded-For / X-Real-IP are the
// real client; RemoteAddr is only the fallback for direct connections.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// authenticate verifies the Bearer token and returns the request context on
// success, writing the appropriate error and returning ok=false otherwise.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (authedRequest, bool) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "認証が必要です。")
		return authedRequest{}, false
	}
	user, err := s.auth.verifyToken(r.Context(), token)
	if err != nil {
		s.writeAuthError(w, err)
		return authedRequest{}, false
	}
	return authedRequest{token: token, user: user}, true
}

type authedRequest struct {
	token string
	user  discordUser
}

func (s *Server) writeAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, errUpstreamRateLimited) {
		log.Println("WARN: activity Discord upstream unavailable or rate limited")
		writeError(w, http.StatusServiceUnavailable, "upstream_rate_limited", "Discord APIが混雑しています。しばらくしてからお試しください。")
		return
	}
	log.Println("WARN: activity authentication failed")
	writeError(w, http.StatusUnauthorized, "unauthorized", "認証に失敗しました。再度お試しください。")
}

// requireMember ensures the authenticated user belongs to gid.
func (s *Server) requireMember(w http.ResponseWriter, r *http.Request, ar authedRequest, gid string) bool {
	member, err := s.auth.guildMembership(r.Context(), ar.token, gid)
	if err != nil {
		s.writeAuthError(w, err)
		return false
	}
	if !member {
		writeError(w, http.StatusForbidden, "forbidden", "このサーバーのメンバーではありません。")
		return false
	}
	return true
}

// allowMutation applies the per-user rate limit for state-changing requests.
func (s *Server) allowMutation(w http.ResponseWriter, userID string) bool {
	if !s.limiter.allow("user:"+userID, 30, 30) {
		log.Println("WARN: activity mutation rate limit exceeded")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "操作が多すぎます。しばらくしてからお試しください。")
		return false
	}
	return true
}

func (s *Server) loadOrCreateUser(userID string) (model.UserInfo, error) {
	u, err := global.DB.GetUser(userID)
	if err != nil {
		created, nerr := global.DB.NewUser(userID)
		if nerr != nil {
			return model.UserInfo{}, nerr
		}
		return created.UserInfo, nil
	}
	return u.UserInfo, nil
}

func wordsResponseFor(gid string) (wordsResponse, error) {
	m, err := global.DB.ListWords(gid)
	if err != nil {
		return wordsResponse{}, err
	}
	words := make([]wordEntry, 0, len(m))
	for k, v := range m {
		words = append(words, wordEntry{Word: k, Reading: v})
	}
	sort.Slice(words, func(i, j int) bool { return words[i].Word < words[j].Word })
	return wordsResponse{Words: words, Count: len(words)}, nil
}

// sanitizeWord trims, length-checks (1..64 runes) and rejects control chars.
func sanitizeWord(s string) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n < wordRuneMin || n > wordRuneMax {
		return "", errors.New("単語・読みは1〜64文字で入力してください。")
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", errors.New("改行や制御文字は使用できません。")
		}
	}
	return s, nil
}

func validationMessage(err error) string {
	if errors.Is(err, voice.ErrVoiceNotFound) {
		return "指定された声が存在しません。"
	}
	return "音声パラメータが許容範囲外です。"
}

// --- handlers --------------------------------------------------------------

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"client_id": s.clientID})
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow("ip:"+clientIP(r), 10, 10) {
		log.Println("WARN: activity /api/token rate limit exceeded")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "リクエストが多すぎます。しばらくしてからお試しください。")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "認証コードが必要です。")
		return
	}

	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", body.Code)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, discordAPIBase+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "upstream_rate_limited", "トークンの取得に失敗しました。")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "upstream_rate_limited", "Discordへの接続に失敗しました。")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		writeError(w, http.StatusServiceUnavailable, "upstream_rate_limited", "Discord APIが混雑しています。")
		return
	}
	if resp.StatusCode != http.StatusOK {
		// Never surface Discord's raw error body — it can reference client
		// credentials in some failure modes.
		writeError(w, http.StatusBadRequest, "invalid_code", "認証コードが無効です。")
		return
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDiscordBody)).Decode(&tok); err != nil || tok.AccessToken == "" {
		writeError(w, http.StatusServiceUnavailable, "upstream_rate_limited", "トークンの取得に失敗しました。")
		return
	}
	// Only the access token is returned; refresh_token stays server-side.
	writeJSON(w, http.StatusOK, map[string]string{"access_token": tok.AccessToken})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": ar.user})
}

func (s *Server) handleVoices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, voicesResponse())
}

func voicesResponse() map[string]any {
	return map[string]any{
		"voices": voice.VoiceCatalog(),
		"engines": map[string]string{
			voice.EngineOpenJTalk:   "Open JTalk",
			voice.EngineLocal:       "ローカル",
			voice.EngineVoicevox:    "VOICEVOX",
			voice.EngineAivisSpeech: "AivisSpeech",
			voice.EngineVoiceRoid:   "VOICEROID",
			voice.EngineAquesTalk:   "AquesTalk",
		},
		"params": map[string]map[string][2]float64{
			"default": {
				"speed":     {0.5, 2},
				"tone":      {-20, 20},
				"intone":    {0, 4},
				"threshold": {0, 1},
				"allpass":   {0, 1},
				"volume":    {-20, 20},
			},
			"voiceroid": {
				"speed":     {0.5, 2},
				"tone":      {0.5, 2},
				"intone":    {0, 2},
				"threshold": {0, 1},
				"allpass":   {0, 1},
				"volume":    {-20, 20},
			},
			// intonationScale is emotion strength (0-2) in AivisSpeech.
			"aivisspeech": {
				"speed":     {0.5, 2},
				"tone":      {-20, 20},
				"intone":    {0, 2},
				"threshold": {0, 1},
				"allpass":   {0, 1},
				"volume":    {-20, 20},
			},
		},
	}
}

func (s *Server) handleGetVoice(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	ui, err := s.loadOrCreateUser(ar.user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "ユーザー情報の取得に失敗しました。")
		return
	}
	writeJSON(w, http.StatusOK, userInfoResponse{UserInfo: toUserInfoJSON(ui)})
}

func (s *Server) handlePutVoice(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if !s.allowMutation(w, ar.user.ID) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var in userInfoJSON
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_info", "音声設定の形式が正しくありません。")
		return
	}
	ui := in.toModel()
	if err := voice.ValidateUserInfo(ui); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_info", validationMessage(err))
		return
	}
	if err := global.DB.AddUser(ar.user.ID, ui); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "保存に失敗しました。")
		return
	}
	writeJSON(w, http.StatusOK, userInfoResponse{UserInfo: toUserInfoJSON(ui)})
}

func (s *Server) handleRandomVoice(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if !s.allowMutation(w, ar.user.ID) {
		return
	}
	ui := db.MakeRandom()
	if err := global.DB.AddUser(ar.user.ID, ui); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "保存に失敗しました。")
		return
	}
	writeJSON(w, http.StatusOK, userInfoResponse{UserInfo: toUserInfoJSON(ui)})
}

func (s *Server) handleGetWords(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	gid := r.PathValue("gid")
	if !s.requireMember(w, r, ar, gid) {
		return
	}
	resp, err := wordsResponseFor(gid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "辞書の取得に失敗しました。")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePostWord(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	gid := r.PathValue("gid")
	if !s.requireMember(w, r, ar, gid) {
		return
	}
	if !s.allowMutation(w, ar.user.ID) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var body struct {
		Word    string `json:"word"`
		Reading string `json:"reading"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_word", "入力の形式が正しくありません。")
		return
	}
	word, err := sanitizeWord(body.Word)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_word", err.Error())
		return
	}
	reading, err := sanitizeWord(body.Reading)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_word", err.Error())
		return
	}

	// Per-guild cap. Overwriting an already-registered word is always allowed.
	existing, err := global.DB.ListWords(gid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "辞書の取得に失敗しました。")
		return
	}
	if _, ok := existing[word]; !ok && len(existing) >= maxWordsPerGuild {
		writeError(w, http.StatusBadRequest, "word_limit", "登録できる単語数の上限に達しています。")
		return
	}
	if err := global.DB.AddWord(gid, word, reading); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "単語の登録に失敗しました。")
		return
	}
	resp, err := wordsResponseFor(gid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "辞書の取得に失敗しました。")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteWord(w http.ResponseWriter, r *http.Request) {
	ar, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	gid := r.PathValue("gid")
	if !s.requireMember(w, r, ar, gid) {
		return
	}
	if !s.allowMutation(w, ar.user.ID) {
		return
	}

	// r.PathValue percent-decodes the {word} segment for us.
	word := r.PathValue("word")
	if err := global.DB.DeleteWord(gid, word); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "単語の削除に失敗しました。")
		return
	}
	resp, err := wordsResponseFor(gid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "辞書の取得に失敗しました。")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
