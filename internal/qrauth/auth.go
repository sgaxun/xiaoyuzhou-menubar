// Package qrauth implements the accounts site's QR authentication protocol.
// Pending sessions and credentials are kept in memory only.
package qrauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const origin = "https://web-api.xiaoyuzhoufm.com"
const userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
const defaultAppID = "v6worU4NnWyL"

// Tokens are passed only to Persist; they never appear in public responses.
type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// Persist must complete its writes before returning. Errors are sanitized.
// It must not call back into Auth. On failure Check retries these same tokens.
type Persist func(Tokens) (map[string]any, error)
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}
type Options struct {
	Persist    Persist
	HTTPClient HTTPClient
	Clock      func() time.Time
	// AppID optionally overrides the public accounts application identifier.
	AppID string
}

// Result is the exact start/check/cancel JSON object. Success includes user:null
// and hasRefresh:false when applicable; other states omit both fields.
type Result map[string]any

type Auth struct {
	mu      sync.Mutex
	active  *session
	persist Persist
	client  HTTPClient
	clock   func() time.Time
	appID   string
}
type session struct {
	op           sync.Mutex
	id, remoteID string
	expiresAt    time.Time
	ctx          context.Context
	cancel       context.CancelFunc
	pending      *Tokens
	result       Result
}

func New(o Options) *Auth {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if o.Clock == nil {
		o.Clock = time.Now
	}
	if o.AppID == "" {
		o.AppID = defaultAppID
	}
	return &Auth{persist: o.Persist, client: o.HTTPClient, clock: o.Clock, appID: o.AppID}
}
func expired() Result { return Result{"ok": true, "status": "expired"} }
func copyResult(r Result) Result {
	b, _ := json.Marshal(r)
	var c Result
	_ = json.Unmarshal(b, &c)
	return c
}
func (a *Auth) invalidate() {
	if a.active != nil {
		a.active.cancel()
		a.active.pending = nil
		a.active = nil
	}
}

// Start replaces any previous session. All HTTP requests have a 12 second limit.
func (a *Auth) Start() (Result, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, errors.New("暂时无法创建登录二维码，请重试")
	}
	id[6] = (id[6] & 15) | 64
	id[8] = (id[8] & 63) | 128
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{id: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), ctx: ctx, cancel: cancel}
	a.mu.Lock()
	a.invalidate()
	a.active = s
	a.mu.Unlock()
	response, err := a.post(s, "/v1/auth/qrcode/create", map[string]string{"clientId": "xyz-web"})
	if err != nil {
		a.mu.Lock()
		stale := a.active != s
		a.mu.Unlock()
		if stale {
			return expired(), nil
		}
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == 429 {
			return nil, errors.New("操作太频繁，请稍后再试")
		}
		return nil, errors.New("暂时无法创建登录二维码，请重试")
	}
	var payload struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload)
	if err != nil || payload.ID == "" || len(utf16.Encode([]rune(payload.ID))) > 512 {
		return nil, errors.New("登录服务返回的二维码无效，请重试")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active != s {
		return expired(), nil
	}
	s.remoteID = payload.ID
	s.expiresAt = a.clock().Add(180 * time.Second)
	return Result{"ok": true, "sessionId": s.id, "url": "https://h5.xiaoyuzhoufm.com/oauth?qrcode_id=" + encodeURIComponent(payload.ID), "expiresAt": s.expiresAt.UnixMilli(), "status": "waiting"}, nil
}

// encodeURIComponent matches JavaScript, including its unescaped punctuation.
func encodeURIComponent(s string) string {
	v := url.QueryEscape(s)
	v = strings.ReplaceAll(v, "+", "%20")
	for _, p := range [][2]string{{"%21", "!"}, {"%27", "'"}, {"%28", "("}, {"%29", ")"}, {"%2A", "*"}} {
		v = strings.ReplaceAll(v, p[0], p[1])
	}
	return v
}

// Check serializes polling and persistence for a session. Cancel or Start can
// invalidate in-flight HTTP work. Persistence is committed under the state lock:
// cancellation cannot return while an earlier persistence callback is running.
func (a *Auth) Check(id string) (Result, error) {
	a.mu.Lock()
	s := a.active
	a.mu.Unlock()
	if s == nil || s.id != id {
		return expired(), nil
	}
	s.op.Lock()
	defer s.op.Unlock()
	a.mu.Lock()
	if a.active != s || s.remoteID == "" {
		a.mu.Unlock()
		return expired(), nil
	}
	if s.result != nil {
		r := copyResult(s.result)
		a.mu.Unlock()
		return r, nil
	}
	if s.pending != nil {
		defer a.mu.Unlock()
		return a.finish(s)
	}
	if !a.clock().Before(s.expiresAt) {
		a.mu.Unlock()
		return expired(), nil
	}
	a.mu.Unlock()
	response, err := a.post(s, "/v1/auth/qrcode/login", map[string]string{"id": s.remoteID})
	var payload struct {
		Status string `json:"status"`
	}
	if response != nil {
		defer response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			// Do not hold the state lock while reading a potentially slow body:
			// Cancel must be able to abort the request context.
			_ = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload)
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active != s {
		return expired(), nil
	}
	if err != nil {
		return nil, err
	}
	switch response.StatusCode {
	case 400, 401, 410:
		return expired(), nil
	case 429:
		return nil, errors.New("查询过于频繁，请稍后刷新二维码")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("暂时无法查询扫码状态，请稍后重试")
	}
	switch strings.ToUpper(payload.Status) {
	case "WAITTING", "WAITING":
		return Result{"ok": true, "status": "waiting"}, nil
	case "SCANNED":
		return Result{"ok": true, "status": "scanned"}, nil
	case "EXPIRED", "CANCELED", "CANCELLED":
		return expired(), nil
	case "CONFIRMED", "USED":
	default:
		return nil, errors.New("登录服务返回未知状态，请刷新二维码")
	}
	value := func(name string) string {
		if v := response.Header.Get(name); v != "" {
			return v
		}
		for _, line := range response.Header.Values("Set-Cookie") {
			pair := strings.SplitN(line, ";", 2)[0]
			key, v, ok := strings.Cut(pair, "=")
			if ok && key == name {
				decoded, err := url.PathUnescape(v)
				if err == nil {
					return decoded
				}
				return ""
			}
		}
		return ""
	}
	tokens := Tokens{AccessToken: value("x-jike-access-token"), RefreshToken: value("x-jike-refresh-token")}
	if tokens.AccessToken == "" {
		return nil, errors.New("未收到完整登录结果，请刷新二维码重新确认")
	}
	s.pending = &tokens
	return a.finish(s)
}

// finish runs with mu held to linearize persistence against cancellation.
func (a *Auth) finish(s *session) (Result, error) {
	if a.persist == nil {
		return nil, errors.New("保存登录状态失败，请重试")
	}
	user, err := a.persist(*s.pending)
	if err != nil {
		return nil, errors.New("保存登录状态失败，请重试")
	}
	// A persistence callback must return a JSON-compatible public user map.
	var publicUser any
	if user != nil {
		publicUser = user
	}
	r := Result{"ok": true, "status": "success", "user": publicUser, "hasRefresh": s.pending.RefreshToken != ""}
	if _, err := json.Marshal(r); err != nil {
		return nil, errors.New("保存登录状态失败，请重试")
	}
	s.result = copyResult(r)
	s.pending = nil
	return copyResult(s.result), nil
}

// Cancel() cancels all; Cancel(id) cancels only the matching session. An explicit
// empty ID does not cancel an active session (matching JS undefined vs "").
func (a *Auth) Cancel(ids ...string) Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(ids) == 0 || (a.active != nil && a.active.id == ids[0]) {
		a.invalidate()
	}
	return Result{"ok": true}
}
func (a *Auth) post(s *session, path string, body any) (*http.Response, error) {
	data, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(s.ctx, 12*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+path, bytes.NewReader(data))
	if err != nil {
		cancel()
		return nil, errors.New("连接小宇宙登录服务失败，请检查网络后重试")
	}
	for k, v := range map[string]string{"content-type": "application/json", "accept": "application/json", "user-agent": userAgent, "origin": "https://accounts.xiaoyuzhoufm.com", "referer": "https://accounts.xiaoyuzhoufm.com/", "x-midway-app-id": a.appID} {
		req.Header.Set(k, v)
	}
	response, err := a.client.Do(req)
	if err != nil {
		cancel()
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
		return nil, errors.New("连接小宇宙登录服务失败，请检查网络后重试")
	}
	if response == nil || response.Body == nil {
		cancel()
		return nil, errors.New("连接小宇宙登录服务失败，请检查网络后重试")
	}
	response.Body = &cancelBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }
