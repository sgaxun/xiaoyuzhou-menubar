// Package podcast implements the Xiaoyuzhou public page and authenticated API client.
package podcast

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Map = map[string]any
type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}
type Error struct {
	Code    string
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

// IsUnauthorized also recognizes wrapped API errors.
func IsUnauthorized(err error) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr != nil && apiErr.Code == "UNAUTHORIZED"
}

// HTTPClient may supply a custom Transport for tests. The client is copied before
// use so its redirect policy is always enforced without mutating caller state.
// A cookie jar is deliberately not used: credentials come only from arguments.
type Client struct{ HTTPClient *http.Client }

func NewClient() *Client { return &Client{HTTPClient: &http.Client{Timeout: 30 * time.Second}} }

const siteBase = "https://www.xiaoyuzhoufm.com"
const apiBase = "https://api.xiaoyuzhoufm.com"
const webBase = "https://web-api.xiaoyuzhoufm.com"
const mobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
const appUA = "Xiaoyuzhou/2.57.1 (build:1576; iOS 17.4.1)"

var idRE = regexp.MustCompile(`^[0-9a-zA-Z]{20,32}$`)
var episodeRE = regexp.MustCompile(`/episode/([0-9a-zA-Z]{20,32})`)
var podcastRE = regexp.MustCompile(`/podcast/([0-9a-zA-Z]{20,32})`)

func IsValidID(id string) bool { return idRE.MatchString(id) }
func ParseTarget(input string) Map {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil
	}
	if m := episodeRE.FindStringSubmatch(raw); m != nil {
		return Map{"type": "episode", "id": m[1]}
	}
	if m := podcastRE.FindStringSubmatch(raw); m != nil {
		return Map{"type": "podcast", "id": m[1]}
	}
	typ := "url"
	if IsValidID(raw) {
		typ = "unknown"
	}
	return Map{"type": typ, "id": raw}
}
func validURL(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && u.Opaque == ""
}
func contentHost(h string) bool {
	switch h {
	case "xiaoyuzhoufm.com", "www.xiaoyuzhoufm.com", "xyzfm.link", "xiaoyuzhou.com", "www.xiaoyuzhou.com":
		return true
	}
	return false
}
func ValidateContentURL(value string) (string, error) {
	u, e := url.Parse(value)
	if e != nil || !validURL(u) || !contentHost(u.Hostname()) {
		return "", fmt.Errorf("仅支持小宇宙 HTTPS 链接")
	}
	return u.String(), nil
}
func allowedURL(u *url.URL) bool {
	return validURL(u) && (contentHost(u.Hostname()) || u.Hostname() == "api.xiaoyuzhoufm.com" || u.Hostname() == "web-api.xiaoyuzhoufm.com")
}
func (c *Client) do(req *http.Request, manual bool) (*http.Response, error) {
	if !allowedURL(req.URL) {
		return nil, fmt.Errorf("不允许的请求地址")
	}
	hc := http.Client{Timeout: 30 * time.Second}
	if c.HTTPClient != nil {
		hc = *c.HTTPClient
	}
	hc.Jar = nil
	origin := req.URL.Hostname()
	hc.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if manual {
			return http.ErrUseLastResponse
		}
		if len(via) > 5 {
			return fmt.Errorf("链接跳转过多")
		}
		// Reject cross-host redirects entirely, including between official hosts:
		// Go does not automatically strip custom token headers on redirects.
		if !allowedURL(r.URL) || r.URL.Hostname() != origin {
			return fmt.Errorf("不允许的跳转地址")
		}
		return nil
	}
	return hc.Do(req)
}
func obj(v any) Map { m, _ := v.(map[string]any); return m }
func arr(v any) []any {
	switch a := v.(type) {
	case []any:
		return a
	case []Map:
		out := make([]any, len(a))
		for i, m := range a {
			out[i] = m
		}
		return out
	}
	return nil
}
func truth(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	}
	return true
}
func fallback(v any, d any) any {
	if truth(v) {
		return v
	}
	return d
}
func str(v any) string { s, _ := v.(string); return s }
func number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case bool:
		if x {
			return 1
		}
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0
		}
		return n
	}
	return 0
}
func normalizeEpisode(raw Map) Map {
	if !truth(raw["eid"]) {
		return nil
	}
	p := obj(raw["podcast"])
	m := obj(raw["media"])
	im := obj(p["image"])
	source := obj(m["source"])
	pid := fallback(p["pid"], strings.Split(str(m["id"]), "/")[0])
	audio := any("")
	if source["mode"] == "PUBLIC" {
		audio = fallback(source["url"], "")
	}
	return Map{"eid": raw["eid"], "pid": pid, "title": fallback(raw["title"], "未命名单集"), "podcastTitle": fallback(p["title"], ""), "duration": number(raw["duration"]), "pubDate": fallback(raw["pubDate"], ""), "image": fallback(im["picUrl"], fallback(im["middlePicUrl"], "")), "isPaid": truth(raw["payType"]) && raw["payType"] != "FREE", "audioUrl": audio}
}
func normalizePodcast(raw Map) Map {
	if !truth(raw["pid"]) {
		return nil
	}
	im := obj(raw["image"])
	return Map{"pid": raw["pid"], "title": fallback(raw["title"], "未命名节目"), "author": fallback(raw["author"], ""), "brief": fallback(raw["brief"], ""), "image": fallback(im["picUrl"], fallback(im["middlePicUrl"], "")), "episodeCount": number(raw["episodeCount"]), "latestEpisodePubDate": fallback(raw["latestEpisodePubDate"], ""), "subscriptionStatus": fallback(raw["subscriptionStatus"], "OFF")}
}
func extractNextData(r io.Reader) (Map, error) {
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken {
			continue
		}
		t := z.Token()
		if t.Data != "script" {
			continue
		}
		found := false
		for _, a := range t.Attr {
			if a.Key == "id" && a.Val == "__NEXT_DATA__" {
				found = true
			}
		}
		if found && z.Next() == html.TextToken {
			var m Map
			if json.Unmarshal(z.Text(), &m) == nil && m != nil {
				return m, nil
			}
		}
	}
	return nil, fmt.Errorf("页面结构无法解析（未找到 __NEXT_DATA__），小宇宙可能改版了")
}
func (c *Client) page(ctx context.Context, path string) (Map, error) {
	r, e := http.NewRequestWithContext(ctx, "GET", siteBase+path, nil)
	if e != nil {
		return nil, e
	}
	r.Header.Set("User-Agent", mobileUA)
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	r.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	resp, e := c.do(r, false)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("小宇宙页面返回 %d", resp.StatusCode)
	}
	d, e := extractNextData(resp.Body)
	if e != nil {
		return nil, e
	}
	return obj(obj(d["props"])["pageProps"]), nil
}
func (c *Client) GetEpisode(ctx context.Context, id string) (Map, error) {
	if !IsValidID(id) {
		return nil, fmt.Errorf("单集 ID 无效")
	}
	p, e := c.page(ctx, "/episode/"+id)
	if e != nil {
		return nil, e
	}
	v := normalizeEpisode(obj(p["episode"]))
	if v == nil {
		return nil, fmt.Errorf("该单集不存在或已下架")
	}
	return v, nil
}
func (c *Client) GetEpisodeNotes(ctx context.Context, id string) (Map, error) {
	if !IsValidID(id) {
		return nil, fmt.Errorf("单集 ID 无效")
	}
	p, e := c.page(ctx, "/episode/"+id)
	if e != nil {
		return nil, e
	}
	r := obj(p["episode"])
	if !truth(r["eid"]) {
		return nil, fmt.Errorf("该单集不存在或已下架")
	}
	return Map{"eid": r["eid"], "title": fallback(r["title"], ""), "html": str(r["shownotes"])}, nil
}
func (c *Client) GetPodcast(ctx context.Context, pid string) (Map, error) {
	if !IsValidID(pid) {
		return nil, fmt.Errorf("节目 ID 无效")
	}
	p, e := c.page(ctx, "/podcast/"+pid)
	if e != nil {
		return nil, e
	}
	raw := obj(p["podcast"])
	pc := normalizePodcast(raw)
	if pc == nil {
		return nil, fmt.Errorf("该节目不存在或已下架")
	}
	list := fallback(raw["episodes"], p["episodes"])
	eps := []Map{}
	for _, v := range arr(list) {
		if ep := normalizeEpisode(obj(v)); ep != nil {
			eps = append(eps, ep)
		}
	}
	return Map{"podcast": pc, "episodes": eps}, nil
}
func (c *Client) api(ctx context.Context, path, token, method string, body Map) (any, error) {
	if token == "" {
		return nil, fmt.Errorf("未登录")
	}
	var b io.Reader
	if method != "GET" {
		data, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		b = bytes.NewReader(data)
	}
	r, e := http.NewRequestWithContext(ctx, method, apiBase+path, b)
	if e != nil {
		return nil, e
	}
	r.Header.Set("User-Agent", appUA)
	r.Header.Set("Accept", "*/*")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("app-version", "2.57.1")
	r.Header.Set("os", "ios")
	r.Header.Set("x-jike-access-token", token)
	r.Header.Set("Local-Time", time.Now().UTC().Format("2006-01-02T15:04:05.000")+"+08:00")
	r.Header.Set("Timezone", "Asia/Shanghai")
	resp, e := c.do(r, false)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, &Error{Code: "UNAUTHORIZED", Status: 401, Message: "登录已过期，请重新导入 cookie"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 640))
		detail := []rune(string(b))
		if len(detail) > 160 {
			detail = detail[:160]
		}
		msg := fmt.Sprintf("小宇宙接口返回 %d", resp.StatusCode)
		if len(detail) > 0 {
			msg += "：" + string(detail)
		}
		return nil, &Error{Status: resp.StatusCode, Message: msg}
	}
	var v any
	e = json.NewDecoder(resp.Body).Decode(&v)
	return v, e
}
func (c *Client) subscriptions(ctx context.Context, token string, limit int) ([]Map, error) {
	out := []Map{}
	var cursor any
	for page := 0; page < 8; page++ {
		b := Map{"limit": strconv.Itoa(limit), "sortOrder": "desc", "sortBy": "subscribedAt"}
		if truth(cursor) {
			b["loadMoreKey"] = cursor
		}
		v, e := c.api(ctx, "/v1/subscription/list", token, "POST", b)
		if e != nil {
			return nil, e
		}
		d := obj(v)
		items := fallback(d["data"], v)
		for _, it := range arr(items) {
			m := obj(it)
			if pc := normalizePodcast(obj(fallback(m["podcast"], it))); pc != nil {
				out = append(out, pc)
			}
		}
		cursor = d["loadMoreKey"]
		if !truth(cursor) {
			break
		}
	}
	return out, nil
}
func (c *Client) ListSubscriptions(ctx context.Context, token string) ([]Map, error) {
	return c.subscriptions(ctx, token, 50)
}
func (c *Client) GetProfile(ctx context.Context, token string) (Map, error) {
	if token == "" {
		return nil, fmt.Errorf("未登录")
	}
	r, e := http.NewRequestWithContext(ctx, "GET", webBase+"/web/user/get-me", nil)
	if e != nil {
		return nil, e
	}
	r.Header.Set("User-Agent", mobileUA)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("x-jike-access-token", token)
	r.Header.Set("x-jike-allow-app-token-in-cookie", "true")
	resp, e := c.do(r, false)
	if e != nil {
		return nil, e
	}
	var b Map
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = json.NewDecoder(resp.Body).Decode(&b)
	}
	resp.Body.Close()
	u := obj(fallback(b["data"], b))
	if truth(u["uid"]) || truth(u["nickname"]) {
		return Map{"uid": fallback(u["uid"], ""), "nickname": fallback(u["nickname"], ""), "avatar": fallback(u["avatarUrl"], fallback(u["avatar"], ""))}, nil
	}
	_, e = c.subscriptions(ctx, token, 1)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if x, ok := e.(*Error); ok && x.Code == "UNAUTHORIZED" {
		return nil, e
	}
	return nil, nil
}
func (c *Client) GetPrivateMedia(ctx context.Context, token, eid string) (string, error) {
	v, e := c.api(ctx, "/v1/private-media/get?dubbing=false&eid="+url.QueryEscape(eid), token, "GET", nil)
	if e != nil {
		return "", e
	}
	m := obj(v)
	return str(fallback(obj(m["data"])["url"], fallback(m["url"], ""))), nil
}
func TokensFromCookieString(cookie string) Tokens {
	find := func(name string) string {
		for _, part := range strings.Split(cookie, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && k == name {
				decoded, e := url.PathUnescape(v)
				if e == nil {
					return decoded
				}
				return v
			}
		}
		return ""
	}
	return Tokens{find("x-jike-access-token"), find("x-jike-refresh-token")}
}
func (c *Client) ExchangeCookie(ctx context.Context, cookie string) (Tokens, error) {
	if cookie == "" {
		return Tokens{}, fmt.Errorf("cookie 为空")
	}
	r, e := http.NewRequestWithContext(ctx, "GET", webBase+"/v1/auth/cookie", nil)
	if e != nil {
		return Tokens{}, e
	}
	r.Header.Set("Cookie", cookie)
	r.Header.Set("User-Agent", mobileUA)
	r.Header.Set("x-jike-allow-app-token-in-cookie", "true")
	r.Header.Set("Accept", "*/*")
	resp, e := c.do(r, false)
	if e != nil {
		return Tokens{}, e
	}
	defer resp.Body.Close()
	t := Tokens{}
	for _, line := range resp.Header.Values("Set-Cookie") {
		for _, name := range []string{"x-jike-access-token", "x-jike-refresh-token"} {
			re := regexp.MustCompile(`(?:^|,\s*)` + name + `=([^;]+)`)
			m := re.FindStringSubmatch(line)
			if m != nil {
				v, e := url.PathUnescape(m[1])
				if e != nil {
					return Tokens{}, e
				}
				if name == "x-jike-access-token" && t.AccessToken == "" {
					t.AccessToken = v
				}
				if name == "x-jike-refresh-token" && t.RefreshToken == "" {
					t.RefreshToken = v
				}
			}
		}
	}
	if t.AccessToken != "" {
		return t, nil
	}
	if resp.StatusCode == 401 {
		return t, fmt.Errorf("cookie 无效或已过期，请重新在浏览器登录小宇宙")
	}
	return t, fmt.Errorf("未能从响应中找到 token")
}
func (c *Client) RefreshTokens(ctx context.Context, access, refresh string) (Tokens, error) {
	if refresh == "" {
		return Tokens{}, fmt.Errorf("缺少 refresh token，无法自动续期")
	}
	r, e := http.NewRequestWithContext(ctx, "POST", apiBase+"/app_auth_tokens.refresh", nil)
	if e != nil {
		return Tokens{}, e
	}
	for k, v := range map[string]string{"User-Agent": appUA, "Accept": "*/*", "Content-Type": "application/x-www-form-urlencoded; charset=utf-8", "x-jike-access-token": access, "x-jike-refresh-token": refresh, "app-version": "2.57.1", "os": "ios"} {
		r.Header.Set(k, v)
	}
	resp, e := c.do(r, false)
	if e != nil {
		return Tokens{}, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "REFRESH_FAILED"
		if resp.StatusCode == 401 {
			code = "UNAUTHORIZED"
		}
		return Tokens{}, &Error{Code: code, Status: resp.StatusCode, Message: fmt.Sprintf("登录态续期失败（%d）", resp.StatusCode)}
	}
	t := Tokens{resp.Header.Get("x-jike-access-token"), resp.Header.Get("x-jike-refresh-token")}
	if t.AccessToken == "" {
		var b Map
		_ = json.NewDecoder(resp.Body).Decode(&b)
		t.AccessToken = str(b["x-jike-access-token"])
		if t.RefreshToken == "" {
			t.RefreshToken = str(b["x-jike-refresh-token"])
		}
	}
	if t.AccessToken == "" {
		return t, &Error{Code: "REFRESH_FAILED", Message: "登录态续期未返回新 token"}
	}
	if t.RefreshToken == "" {
		t.RefreshToken = refresh
	}
	return t, nil
}
func (c *Client) SearchAll(ctx context.Context, token, keyword string) (Map, error) {
	v, e := c.api(ctx, "/v1/search/create", token, "POST", Map{"keyword": keyword, "limit": 20})
	if e != nil {
		return nil, e
	}
	eps, pcs := []Map{}, []Map{}
	se, sp := map[string]bool{}, map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch n := v.(type) {
		case []any:
			for _, it := range n {
				walk(it)
			}
		case map[string]any:
			if id := str(n["eid"]); id != "" && !se[id] {
				if ep := normalizeEpisode(n); ep != nil {
					se[id] = true
					eps = append(eps, ep)
				}
			}
			if id := str(n["pid"]); id != "" && n["type"] == "PODCAST" && !sp[id] {
				if pc := normalizePodcast(n); pc != nil {
					sp[id] = true
					pcs = append(pcs, pc)
				}
			}
			keys := make([]string, 0, len(n))
			for k := range n {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(n[k])
			}
		}
	}
	walk(v)
	return Map{"episodes": eps, "podcasts": pcs}, nil
}
func (c *Client) GetTopList(ctx context.Context, token, category string) (Map, error) {
	if category == "" {
		category = "HOT_EPISODES_IN_24_HOURS"
	}
	v, e := c.api(ctx, "/v1/top-list/get?category="+url.QueryEscape(category), token, "GET", nil)
	if e != nil {
		return nil, e
	}
	d := obj(v)
	b := obj(fallback(d["data"], v))
	eps := []Map{}
	for _, it := range arr(b["items"]) {
		m := obj(it)
		if ep := normalizeEpisode(obj(fallback(m["item"], it))); ep != nil {
			eps = append(eps, ep)
		}
	}
	return Map{"title": fallback(b["title"], "最热榜"), "information": fallback(b["information"], ""), "publishDate": fallback(b["publishDate"], ""), "episodes": eps}, nil
}
func (c *Client) GetEditorPicks(ctx context.Context, token, cursor string) (Map, error) {
	b := Map{}
	if cursor != "" {
		b["loadMoreKey"] = cursor
	}
	v, e := c.api(ctx, "/v1/editor-pick/list-history", token, "POST", b)
	if e != nil {
		return nil, e
	}
	d := obj(v)
	days := []Map{}
	for _, it := range arr(d["data"]) {
		day := obj(it)
		eps := []Map{}
		for _, p := range arr(day["picks"]) {
			if ep := normalizeEpisode(obj(obj(p)["episode"])); ep != nil {
				eps = append(eps, ep)
			}
		}
		if len(eps) > 0 {
			days = append(days, Map{"date": fallback(day["date"], ""), "episodes": eps})
		}
	}
	return Map{"days": days, "loadMoreKey": str(d["loadMoreKey"])}, nil
}
func (c *Client) ResolveTarget(ctx context.Context, input string) (Map, error) {
	p := ParseTarget(input)
	if p == nil {
		return nil, fmt.Errorf("无法识别的链接")
	}
	typ, id := str(p["type"]), str(p["id"])
	if typ == "episode" || typ == "podcast" {
		return p, nil
	}
	if typ == "unknown" {
		_, e := c.GetEpisode(ctx, id)
		if e == nil {
			return Map{"type": "episode", "id": id}, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return Map{"type": "podcast", "id": id}, nil
	}
	final, e := ValidateContentURL(id)
	if e != nil {
		return nil, e
	}
	for redirects := 0; ; redirects++ {
		r, e := http.NewRequestWithContext(ctx, "GET", final, nil)
		if e != nil {
			return nil, e
		}
		r.Header.Set("User-Agent", mobileUA)
		resp, e := c.do(r, true)
		if e != nil {
			return nil, e
		}
		resp.Body.Close()
		switch resp.StatusCode {
		case 301, 302, 303, 307, 308:
			if redirects >= 5 {
				return nil, fmt.Errorf("链接跳转过多")
			}
			location := resp.Header.Get("Location")
			if location == "" {
				return nil, fmt.Errorf("跳转地址缺失")
			}
			u, _ := url.Parse(final)
			next, e := u.Parse(location)
			if e != nil {
				return nil, e
			}
			final, e = ValidateContentURL(next.String())
			if e != nil {
				return nil, e
			}
		default:
			again := ParseTarget(final)
			if again != nil && (again["type"] == "episode" || again["type"] == "podcast") {
				return again, nil
			}
			return Map{"type": "unknown", "id": id}, nil
		}
	}
}
