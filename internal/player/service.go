package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xiaoyuzhou-menubar/internal/podcast"
	"xiaoyuzhou-menubar/internal/qrauth"
)

type Service struct {
	mu               sync.Mutex
	updateMu         sync.Mutex
	updateEpoch      uint64
	updateLast       time.Time
	updateChecking   bool
	updateAutoCancel context.CancelFunc
	dir              string
	state            object
	cache            object
	client           *podcast.Client
	qr               *qrauth.Auth
}

func New(dir string) (*Service, error) { return NewWithClient(dir, podcast.NewClient()) }
func NewWithClient(dir string, client *podcast.Client) (*Service, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	state, err := readObject(filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	defaults := defaultState()
	for k, v := range state {
		defaults[k] = v
	}
	cache, err := readObject(filepath.Join(dir, "cache.json"))
	if err != nil {
		var syntax *json.SyntaxError
		var kind *json.UnmarshalTypeError
		if !errors.As(err, &syntax) && !errors.As(err, &kind) {
			return nil, err
		}
		broken := filepath.Join(dir, fmt.Sprintf("cache.invalid-%d.json", time.Now().UnixNano()))
		if err = os.Rename(filepath.Join(dir, "cache.json"), broken); err != nil {
			return nil, err
		}
		if err = os.Chmod(broken, 0600); err != nil {
			return nil, err
		}
		cache = object{}
	}
	s := &Service{dir: dir, state: defaults, cache: cache, client: client}
	s.qr = qrauth.New(qrauth.Options{Persist: func(tokens qrauth.Tokens) (map[string]any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		user, err := s.client.GetProfile(ctx, tokens.AccessToken)
		if err != nil && unauthorized(err) {
			return nil, err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err = s.clearAccountCache(); err != nil {
			return nil, err
		}
		if err = s.writeTokens(tokens.AccessToken, tokens.RefreshToken); err != nil {
			return nil, err
		}
		next := clone(s.state)
		next["user"] = user
		if err = s.saveState(next); err != nil {
			return nil, err
		}
		return user, nil
	}})
	return s, nil
}
func (s *Service) tokens() (string, string, error) {
	record, err := readObject(filepath.Join(s.dir, "credentials.json"))
	if err != nil {
		return "", "", err
	}
	if str(record["kind"]) != "grant" {
		return "", "", nil
	}
	payload := obj(record["payload"])
	return str(payload["accessToken"]), str(payload["refreshToken"]), nil
}
func (s *Service) writeTokens(access, refresh string) error {
	return writeJSON(filepath.Join(s.dir, "credentials.json"), object{"kind": "grant", "payload": object{"accessToken": access, "refreshToken": refresh, "savedAt": nowISO()}})
}

var errLogin = errors.New("登录已过期，请到设置重新扫码登录")

func unauthorized(err error) bool { return errors.Is(err, errLogin) || podcast.IsUnauthorized(err) }
func (s *Service) withToken(ctx context.Context, fn func(string) (any, error)) (any, error) {
	access, refresh, err := s.tokens()
	if err != nil {
		return nil, err
	}
	if access == "" {
		return nil, errLogin
	}
	v, err := fn(access)
	if !unauthorized(err) || refresh == "" {
		return v, err
	}
	fresh, err := s.client.RefreshTokens(ctx, access, refresh)
	if err != nil {
		return nil, err
	}
	if err = s.writeTokens(fresh.AccessToken, fresh.RefreshToken); err != nil {
		return nil, err
	}
	return fn(fresh.AccessToken)
}
func merged(v any) object {
	out := object{"ok": true}
	for k, x := range obj(v) {
		out[k] = x
	}
	return out
}
func (s *Service) Request(action string, body any) (any, error) {
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > 256*1024 {
		return nil, errors.New("请求内容过大或格式错误")
	}
	if len(action) > 4096 {
		return nil, errors.New("请求格式错误")
	}
	u, err := url.Parse("/" + action)
	if err != nil || strings.Contains(u.Path[1:], "/") {
		return nil, errors.New("请求格式错误")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var decoded object
	if body != nil {
		if err = json.Unmarshal(raw, &decoded); err != nil {
			return nil, err
		}
	}
	var result any
	switch u.Path[1:] {
	case "checkEpisodeUpdates":
		result, err = s.checkEpisodeUpdates(ctx)
	case "qrStart":
		result, err = s.qr.Start()
	case "qrStatus":
		result, err = s.qr.Check(str(decoded["sessionId"]))
	case "qrCancel":
		result = s.qr.Cancel(str(decoded["sessionId"]))
	default:
		// Never hold the state lock while acquiring QR's commit lock.
		if u.Path[1:] == "logout" {
			s.qr.Cancel()
		}
		s.mu.Lock()
		result, err = s.dispatch(ctx, u.Path[1:], u.Query(), decoded)
		s.mu.Unlock()
	}
	if err != nil {
		return object{"ok": false, "error": err.Error(), "needsLogin": unauthorized(err)}, nil
	}
	return result, nil
}
func (s *Service) dispatch(ctx context.Context, action string, q url.Values, body object) (any, error) {
	force := q.Get("refresh") == "1"
	ok := object{"ok": true}
	switch action {
	case "setEpisodeUpdateAutomatic":
		enabled, valid := body["enabled"].(bool)
		if !valid {
			return nil, errors.New("缺少自动检查开关")
		}
		next := clone(s.state)
		next["episodeUpdatesAutomatic"] = enabled
		if err := s.saveState(next); err != nil {
			return nil, err
		}
		if !enabled && s.updateAutoCancel != nil {
			s.updateAutoCancel()
		}
		return s.episodeUpdateFeed(), nil
	case "episodeUpdates":
		return s.episodeUpdateFeed(), nil
	case "dismissEpisodeUpdates":
		next := clone(s.state)
		feed := obj(next["episodeUpdates"])
		ids := map[string]bool{}
		for _, id := range list(body["eids"]) {
			ids[str(id)] = true
		}
		pending := []any{}
		for _, ep := range list(feed["episodes"]) {
			if !ids[str(obj(ep)["eid"])] {
				pending = append(pending, ep)
			}
		}
		feed["episodes"] = pending
		next["episodeUpdates"] = feed
		if err := s.saveState(next); err != nil {
			return nil, err
		}
		return s.episodeUpdateFeed(), nil
	case "state":
		access, _, err := s.tokens()
		if err != nil {
			return nil, err
		}
		return object{"ok": true, "state": clone(s.state), "loggedIn": access != ""}, nil
	case "saveQueue":
		next := clone(s.state)
		next["queue"] = list(body["queue"])
		next["queueIndex"] = num(body["index"])
		queuedIDs := map[string]bool{}
		for _, v := range list(next["queue"]) {
			queuedIDs[str(obj(v)["eid"])] = true
		}
		if feed, exists := next["episodeUpdates"]; exists {
			f := obj(feed)
			pending := []any{}
			for _, v := range list(f["episodes"]) {
				if !queuedIDs[str(obj(v)["eid"])] {
					pending = append(pending, v)
				}
			}
			f["episodes"] = pending
			next["episodeUpdates"] = f
		}
		return ok, s.saveState(next)
	case "savePrefs":
		next := clone(s.state)
		for _, k := range []string{"volume", "rate", "panelWidth", "panelHeight"} {
			if v, exists := body[k]; exists {
				if _, isNum := v.(float64); isNum {
					next[k] = v
				}
			}
		}
		return ok, s.saveState(next)
	case "progress":
		return s.progress(body)
	case "logout":
		if err := writeJSON(filepath.Join(s.dir, "credentials.json"), nil); err != nil {
			return nil, err
		}
		if err := s.clearAccountCache(); err != nil {
			return nil, err
		}
		next := clone(s.state)
		next["user"] = nil
		return ok, s.saveState(next)
	case "login":
		raw := strings.TrimSpace(str(body["cookie"]))
		if raw == "" {
			return nil, errors.New("请扫码登录或提供有效凭证")
		}
		tokens := podcast.TokensFromCookieString(raw)
		if !strings.Contains(raw, "=") && len(raw) >= 40 {
			tokens.AccessToken = raw
		}
		if tokens.AccessToken == "" {
			var err error
			tokens, err = s.client.ExchangeCookie(ctx, raw)
			if err != nil {
				return nil, err
			}
		}
		user, err := s.client.GetProfile(ctx, tokens.AccessToken)
		if err != nil {
			return nil, err
		}
		if err = s.clearAccountCache(); err != nil {
			return nil, err
		}
		if err = s.writeTokens(tokens.AccessToken, tokens.RefreshToken); err != nil {
			return nil, err
		}
		next := clone(s.state)
		next["user"] = user
		if err = s.saveState(next); err != nil {
			return nil, err
		}
		return object{"ok": true, "user": user, "hasRefresh": tokens.RefreshToken != ""}, nil
	case "clearCache":
		if err := s.resetEpisodeUpdates(); err != nil {
			return nil, err
		}
		s.cache = object{}
		return ok, s.saveCache()
	case "dropCache":
		keys := []any{}
		if eid := q.Get("eid"); eid != "" {
			keys = append(keys, "episode:"+eid, "notes:"+eid)
		}
		if pid := q.Get("pid"); pid != "" {
			used := false
			for _, v := range list(s.state["queue"]) {
				if str(obj(v)["pid"]) == pid {
					used = true
				}
			}
			if !used {
				keys = append(keys, "podcast:"+pid)
			}
		}
		for _, k := range keys {
			delete(s.cache, str(k))
		}
		if err := s.saveCache(); err != nil {
			return nil, err
		}
		return object{"ok": true, "dropped": keys}, nil
	case "subsCache":
		access, _, err := s.tokens()
		if err != nil {
			return nil, err
		}
		entry := object{}
		if access != "" {
			entry = obj(s.cache["subs"])
		}
		return object{"ok": true, "list": list(entry["value"]), "at": num(entry["at"]), "hasCache": entry["value"] != nil, "fresh": entry["value"] != nil && float64(time.Now().UnixMilli())-num(entry["at"]) < 1800000}, nil
	case "subscriptions":
		access, _, err := s.tokens()
		if err != nil {
			return nil, err
		}
		if access == "" {
			return nil, errLogin
		}
		v, err := s.cached("subs", "subs", force, func() (any, error) {
			return s.withToken(ctx, func(token string) (any, error) { return s.client.ListSubscriptions(ctx, token) })
		})
		if err != nil {
			return nil, err
		}
		return object{"ok": true, "list": v, "user": s.state["user"]}, nil
	case "localEpisodes":
		keyword := strings.ToLower(strings.TrimSpace(q.Get("q")))
		hits := []any{}
		if keyword != "" {
			for key, entry := range s.cache {
				if !strings.HasPrefix(key, "podcast:") {
					continue
				}
				for _, v := range list(obj(obj(entry)["value"])["episodes"]) {
					ep := obj(v)
					if strings.Contains(strings.ToLower(str(ep["title"])+" "+str(ep["podcastTitle"])), keyword) {
						hits = append(hits, v)
					}
					if len(hits) >= 60 {
						break
					}
				}
				if len(hits) >= 60 {
					break
				}
			}
		}
		return object{"ok": true, "episodes": hits}, nil
	case "episode", "shownotes", "audioUrl":
		eid := q.Get("eid")
		key := "episode:" + eid
		if action == "shownotes" {
			key = "notes:" + eid
		}
		v, err := s.cached(key, "episode", force, func() (any, error) {
			if action == "shownotes" {
				return s.client.GetEpisodeNotes(ctx, eid)
			}
			return s.client.GetEpisode(ctx, eid)
		})
		if err != nil {
			return nil, err
		}
		if action == "shownotes" {
			return merged(v), nil
		}
		if action == "episode" {
			return object{"ok": true, "episode": v}, nil
		}
		ep := obj(v)
		src := str(ep["audioUrl"])
		if src == "" {
			media, err := s.withToken(ctx, func(token string) (any, error) { return s.client.GetPrivateMedia(ctx, token, eid) })
			if err != nil {
				return nil, err
			}
			src = str(media)
		}
		if src == "" {
			return nil, errors.New("未能获取音频地址")
		}
		if err = validateMedia(src); err != nil {
			return nil, err
		}
		return object{"ok": true, "url": src, "paid": flag(ep["isPaid"])}, nil
	case "open":
		target, err := s.client.ResolveTarget(ctx, str(body["input"]))
		if err != nil {
			return nil, err
		}
		kind, id := str(target["type"]), str(target["id"])
		if kind != "episode" && kind != "podcast" {
			return nil, errors.New("只支持节目链接、单集链接或分享短链")
		}
		v, err := s.cached(kind+":"+id, kind, flag(body["refresh"]), func() (any, error) {
			if kind == "episode" {
				return s.client.GetEpisode(ctx, id)
			}
			return s.client.GetPodcast(ctx, id)
		})
		if err != nil {
			return nil, err
		}
		out := object{"ok": true, "kind": kind, "cached": !flag(body["refresh"])}
		if kind == "episode" {
			out["episode"] = v
		} else {
			for k, x := range obj(v) {
				out[k] = x
			}
		}
		return out, nil
	case "search":
		keyword := strings.TrimSpace(q.Get("q"))
		if keyword == "" {
			return nil, errors.New("缺少关键词")
		}
		v, err := s.cached("search:"+keyword, "search", force, func() (any, error) {
			return s.withToken(ctx, func(token string) (any, error) { return s.client.SearchAll(ctx, token, keyword) })
		})
		return merged(v), err
	case "picks":
		v, err := s.withToken(ctx, func(token string) (any, error) { return s.client.GetEditorPicks(ctx, token, q.Get("cursor")) })
		if err != nil {
			return nil, err
		}
		return object{"ok": true, "days": obj(v)["days"], "cursor": obj(v)["loadMoreKey"]}, nil
	case "discover":
		v, err := s.cached("discover", "discover", force, func() (any, error) {
			return s.withToken(ctx, func(token string) (any, error) {
				tops := []any{}
				for _, category := range []string{"HOT_EPISODES_IN_24_HOURS", "SKYROCKET_EPISODES", "NEW_STAR_EPISODES"} {
					top, e := s.client.GetTopList(ctx, token, category)
					if e == nil {
						tops = append(tops, top)
					} else if unauthorized(e) {
						return nil, e
					}
				}
				picks, e := s.client.GetEditorPicks(ctx, token, "")
				if e != nil {
					return nil, e
				}
				return object{"tops": tops, "picks": picks["days"], "picksCursor": picks["loadMoreKey"]}, nil
			})
		})
		return merged(v), err
	}
	return nil, fmt.Errorf("不支持的操作：%s", action)
}
func validateMedia(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return errors.New("音频地址无效")
	}
	// URLs originate from first-party episode/private-media responses. Publishers
	// may host audio on their own CDN; never attach account credentials here.
	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, ".") || net.ParseIP(host) != nil || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
		return errors.New("音频地址必须为公开 HTTPS 域名")
	}
	return nil
}
