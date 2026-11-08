package podcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const testID = "0123456789abcdef01234567"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Keep the production URL on the request until it reaches the injected transport.
func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	u, _ := url.Parse(s.URL)
	return &Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		q := r.Clone(r.Context())
		v := *r.URL
		q.URL = &v
		q.URL.Scheme = u.Scheme
		q.URL.Host = u.Host
		return http.DefaultTransport.RoundTrip(q)
	})}}
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func rawEpisode() Map {
	return Map{"eid": testID, "title": "单集", "duration": "120", "payType": "PAID", "shownotes": "<p>notes &amp; more</p>", "podcast": Map{"pid": testID, "title": "节目", "image": Map{"middlePicUrl": "cover"}}, "media": Map{"source": Map{"mode": "PUBLIC", "url": "audio"}}}
}
func TestParsing(t *testing.T) {
	for _, input := range []string{testID, "  " + testID + "  "} {
		if p := ParseTarget(input); p["type"] != "unknown" || p["id"] != testID {
			t.Fatal(p)
		}
	}
	for _, kind := range []string{"episode", "podcast"} {
		p := ParseTarget("分享 https://www.xiaoyuzhoufm.com/" + kind + "/" + testID + "?x=1")
		if p["type"] != kind || p["id"] != testID {
			t.Fatal(p)
		}
	}
	if ParseTarget("  ") != nil || IsValidID("bad/") {
		t.Fatal("invalid ID accepted")
	}
	for _, u := range []string{"http://xyzfm.link/a", "https://evil.test/a", "https://www.xiaoyuzhoufm.com.evil.test/a", "https://user@xyzfm.link/a", "https://xyzfm.link:444/a"} {
		if _, e := ValidateContentURL(u); e == nil {
			t.Fatal(u)
		}
	}
	if _, e := ValidateContentURL("https://xyzfm.link:443/a"); e != nil {
		t.Fatal(e)
	}
	for _, html := range []string{`<script id="__NEXT_DATA__" type="application/json">{"props":{}}</script>`, `<script nonce='x' type='application/json' id='__NEXT_DATA__'>{"props":{}}</script>`} {
		m, e := extractNextData(strings.NewReader(html))
		if e != nil || obj(m["props"]) == nil {
			t.Fatal(m, e)
		}
	}
	for _, s := range []string{"<html/>", `<script id="__NEXT_DATA__">bad</script>`} {
		if _, e := extractNextData(strings.NewReader(s)); e == nil {
			t.Fatal("invalid page accepted")
		}
	}
	tokens := TokensFromCookieString("other=1; x-jike-access-token=a%2Bb+c; x-jike-refresh-token=r%3D1")
	if tokens != (Tokens{"a+b+c", "r=1"}) {
		t.Fatal(tokens)
	}
}
func TestPageMethods(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-jike-access-token") != "" || r.Header.Get("Cookie") != "" {
			t.Error("public page credentials")
		}
		p := Map{"episode": rawEpisode(), "podcast": Map{"pid": testID, "episodeCount": "3", "episodes": []any{rawEpisode(), Map{}}}}
		b, _ := json.Marshal(Map{"props": Map{"pageProps": p}})
		fmt.Fprintf(w, `<script id="__NEXT_DATA__" type="application/json">%s</script>`, b)
	})
	ep, e := c.GetEpisode(context.Background(), testID)
	if e != nil || ep["duration"] != float64(120) || ep["audioUrl"] != "audio" || ep["isPaid"] != true || ep["image"] != "cover" {
		t.Fatal(ep, e)
	}
	if _, ok := ep["shownotes"]; ok {
		t.Fatal("untrimmed episode")
	}
	notes, e := c.GetEpisodeNotes(context.Background(), testID)
	if e != nil || notes["html"] != "<p>notes &amp; more</p>" {
		t.Fatal(notes, e)
	}
	pc, e := c.GetPodcast(context.Background(), testID)
	if e != nil || len(pc["episodes"].([]Map)) != 1 {
		t.Fatal(pc, e)
	}
	if _, e = c.GetEpisode(context.Background(), "bad"); e == nil {
		t.Fatal("invalid ID accepted")
	}
	private := normalizeEpisode(Map{"eid": testID, "media": Map{"id": "pid/file", "source": Map{"mode": "PRIVATE", "url": "secret"}}})
	if private["audioUrl"] != "" || private["pid"] != "pid" {
		t.Fatal(private)
	}
}
func TestAuthenticatedReads(t *testing.T) {
	pages := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-jike-access-token") != "fake" {
			t.Error("missing fake token")
		}
		switch r.URL.Path {
		case "/v1/subscription/list":
			var b Map
			_ = json.NewDecoder(r.Body).Decode(&b)
			pages++
			if b["limit"] != "50" || b["sortBy"] != "subscribedAt" {
				t.Error(b)
			}
			if pages == 1 {
				writeJSON(w, Map{"data": []any{Map{"podcast": Map{"pid": testID}}}, "loadMoreKey": Map{"page": 2}})
			} else {
				if obj(b["loadMoreKey"])["page"] != float64(2) {
					t.Error(b)
				}
				writeJSON(w, Map{"data": []any{Map{"pid": "other"}, Map{}}})
			}
		case "/v1/search/create":
			writeJSON(w, Map{"data": []any{rawEpisode(), rawEpisode(), Map{"type": "PODCAST", "pid": testID}, Map{"nested": []any{Map{"type": "PODCAST", "pid": testID}}}}})
		case "/v1/editor-pick/list-history":
			var b Map
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["loadMoreKey"] != "cursor" {
				t.Error(b)
			}
			writeJSON(w, Map{"data": []any{Map{"date": "today", "picks": []any{Map{"episode": rawEpisode()}}}, Map{"date": "empty"}}, "loadMoreKey": "older"})
		case "/v1/top-list/get":
			if r.URL.Query().Get("category") != "HOT_EPISODES_IN_24_HOURS" {
				t.Error(r.URL)
			}
			writeJSON(w, Map{"data": Map{"items": []any{Map{"item": rawEpisode()}, Map{}}}})
		case "/v1/private-media/get":
			if r.URL.Query().Get("eid") != "id & x" {
				t.Error(r.URL)
			}
			writeJSON(w, Map{"data": Map{"url": "private"}})
		case "/web/user/get-me":
			writeJSON(w, Map{"data": Map{"uid": "u", "nickname": "n", "avatarUrl": "a"}})
		default:
			t.Error(r.URL)
			w.WriteHeader(404)
		}
	})
	ctx := context.Background()
	subs, e := c.ListSubscriptions(ctx, "fake")
	if e != nil || len(subs) != 2 || pages != 2 {
		t.Fatal(subs, pages, e)
	}
	search, e := c.SearchAll(ctx, "fake", "kw")
	if e != nil || len(search["episodes"].([]Map)) != 1 || len(search["podcasts"].([]Map)) != 1 {
		t.Fatal(search, e)
	}
	picks, e := c.GetEditorPicks(ctx, "fake", "cursor")
	if e != nil || picks["loadMoreKey"] != "older" || len(picks["days"].([]Map)) != 1 {
		t.Fatal(picks, e)
	}
	top, e := c.GetTopList(ctx, "fake", "")
	if e != nil || top["title"] != "最热榜" || len(top["episodes"].([]Map)) != 1 {
		t.Fatal(top, e)
	}
	media, e := c.GetPrivateMedia(ctx, "fake", "id & x")
	if e != nil || media != "private" {
		t.Fatal(media, e)
	}
	profile, e := c.GetProfile(ctx, "fake")
	if e != nil || !reflect.DeepEqual(profile, Map{"uid": "u", "nickname": "n", "avatar": "a"}) {
		t.Fatal(profile, e)
	}
}
func TestTokensAndErrors(t *testing.T) {
	for _, header := range []bool{false, true} {
		t.Run(fmt.Sprint(header), func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/auth/cookie" {
					if r.Header.Get("Cookie") != "fake=1" {
						t.Error("cookie missing")
					}
					w.Header().Add("Set-Cookie", "x-jike-access-token=a%2Bb; Path=/; HttpOnly")
					w.Header().Add("Set-Cookie", "x-jike-refresh-token=r%3D; Path=/")
					return
				}
				if r.Header.Get("x-jike-refresh-token") != "old" || r.Header.Get("x-jike-access-token") != "access" {
					t.Error("refresh headers")
				}
				if header {
					w.Header().Set("x-jike-access-token", "new")
					return
				}
				writeJSON(w, Map{"x-jike-access-token": "new"})
			})
			v, e := c.ExchangeCookie(context.Background(), "fake=1")
			if e != nil || v != (Tokens{"a+b", "r="}) {
				t.Fatal(v, e)
			}
			v, e = c.RefreshTokens(context.Background(), "access", "old")
			if e != nil || v != (Tokens{"new", "old"}) {
				t.Fatal(v, e)
			}
		})
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) })
	_, e := c.ListSubscriptions(context.Background(), "fake")
	if !IsUnauthorized(fmt.Errorf("wrapped: %w", e)) {
		t.Fatal(e)
	}
	if IsUnauthorized(errors.New("UNAUTHORIZED")) || IsUnauthorized(nil) {
		t.Fatal("incorrect error recognition")
	}
	_, e = c.RefreshTokens(context.Background(), "a", "r")
	if !IsUnauthorized(e) {
		t.Fatal(e)
	}
}
func TestRedirectSecurity(t *testing.T) {
	for _, method := range []string{"profile", "cookie", "refresh", "subscriptions"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "https://evil.test/steal")
				w.WriteHeader(307)
			})
			var e error
			switch method {
			case "profile":
				_, e = c.GetProfile(context.Background(), "secret")
			case "cookie":
				_, e = c.ExchangeCookie(context.Background(), "secret=1")
			case "refresh":
				_, e = c.RefreshTokens(context.Background(), "secret", "refresh")
			default:
				_, e = c.ListSubscriptions(context.Background(), "secret")
			}
			if e == nil || calls != 1 {
				t.Fatal("redirect followed", calls, e)
			}
		})
	}
	for _, target := range []string{"https://evil.test/a", "http://xyzfm.link/a", "https://api.xiaoyuzhoufm.com/a"} {
		calls := 0
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Location", target)
			w.WriteHeader(302)
		})
		if _, e := c.ResolveTarget(context.Background(), "https://xyzfm.link/a"); e == nil || calls != 1 {
			t.Fatal(target, calls, e)
		}
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a" {
			w.Header().Set("Location", "https://www.xiaoyuzhoufm.com/episode/"+testID)
			w.WriteHeader(302)
			return
		}
		_, _ = io.WriteString(w, "ok")
	})
	v, e := c.ResolveTarget(context.Background(), "https://xyzfm.link/a")
	if e != nil || v["type"] != "episode" || v["id"] != testID {
		t.Fatal(v, e)
	}
}
func TestPaginationCapAndCancellation(t *testing.T) {
	n := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		writeJSON(w, Map{"data": []any{}, "loadMoreKey": "more"})
	})
	v, e := c.ListSubscriptions(context.Background(), "fake")
	if e != nil || n != 8 || len(v) != 0 {
		t.Fatal(n, v, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = c.GetEpisode(ctx, testID)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestFallbacksAndEmptyJSON(t *testing.T) {
	ctx := context.Background()
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/user/get-me":
			w.WriteHeader(500)
		case "/v1/subscription/list":
			w.WriteHeader(401)
		default:
			writeJSON(w, Map{})
		}
	})
	if _, err := c.GetProfile(ctx, "fake"); !IsUnauthorized(err) {
		t.Fatal(err)
	}
	for _, call := range []func() (Map, error){func() (Map, error) { return c.GetEditorPicks(ctx, "fake", "") }, func() (Map, error) { return c.GetTopList(ctx, "fake", "") }, func() (Map, error) { return c.SearchAll(ctx, "fake", "") }} {
		m, err := call()
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(m)
		if err != nil || strings.Contains(string(b), "null") {
			t.Fatal(string(b), err)
		}
	}
	n := 0
	c = testClient(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Header().Set("Location", "/loop")
		w.WriteHeader(302)
	})
	if _, err := c.ResolveTarget(ctx, "https://xyzfm.link/loop"); err == nil || n != 6 {
		t.Fatal(n, err)
	}
	c = testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	got, err := c.ResolveTarget(ctx, testID)
	if err != nil || got["type"] != "podcast" {
		t.Fatal(got, err)
	}
	if _, err := c.RefreshTokens(ctx, "", ""); err == nil {
		t.Fatal("empty refresh accepted")
	}
}
