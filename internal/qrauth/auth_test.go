package qrauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type doFunc func(*http.Request) (*http.Response, error)

func (f doFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func newTest(status string, p Persist) *Auth {
	return New(Options{Persist: p, HTTPClient: doFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/create") {
			return response(200, `{"id":"remote"}`), nil
		}
		return response(200, `{"status":"`+status+`"}`), nil
	})})
}
func startID(t *testing.T, a *Auth) string {
	t.Helper()
	r, e := a.Start()
	if e != nil {
		t.Fatal(e)
	}
	return r["sessionId"].(string)
}
func state(t *testing.T, r Result, e error, want string) {
	t.Helper()
	if e != nil || r["ok"] != true || r["status"] != want {
		t.Fatalf("got %v error %v, want %s", r, e, want)
	}
}
func TestStartProtocol(t *testing.T) {
	now := time.UnixMilli(1000)
	a := New(Options{Clock: func() time.Time { return now }, HTTPClient: doFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.String() != origin+"/v1/auth/qrcode/create" {
			t.Fatal("request protocol")
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["clientId"] != "xyz-web" || len(body) != 1 {
			t.Fatal(body)
		}
		for k, v := range map[string]string{"Content-Type": "application/json", "Accept": "application/json", "User-Agent": userAgent, "Origin": "https://accounts.xiaoyuzhoufm.com", "Referer": "https://accounts.xiaoyuzhoufm.com/", "X-Midway-App-Id": defaultAppID} {
			if r.Header.Get(k) != v {
				t.Fatalf("header %s", k)
			}
		}
		d, ok := r.Context().Deadline()
		if !ok || time.Until(d) > 12*time.Second {
			t.Fatal("missing timeout")
		}
		return response(200, `{"id":"a b+&/?'!*()","url":"https://evil.invalid"}`), nil
	})})
	r, e := a.Start()
	state(t, r, e, "waiting")
	if len(r) != 5 || r["expiresAt"] != int64(181000) || r["url"] != "https://h5.xiaoyuzhoufm.com/oauth?qrcode_id=a%20b%2B%26%2F%3F'!*()" {
		t.Fatal(r)
	}
	if len(r["sessionId"].(string)) != 36 {
		t.Fatal("not UUID")
	}
}
func TestStatuses(t *testing.T) {
	for input, want := range map[string]string{"WAITTING": "waiting", "WAITING": "waiting", "waiting": "waiting", "SCANNED": "scanned", "EXPIRED": "expired", "CANCELED": "expired", "CANCELLED": "expired"} {
		t.Run(input, func(t *testing.T) {
			a := newTest(input, nil)
			id := startID(t, a)
			r, e := a.Check(id)
			state(t, r, e, want)
			if len(r) != 2 {
				t.Fatal(r)
			}
		})
	}
}
func TestValidationAndErrors(t *testing.T) {
	cases := []struct {
		code       int
		body, want string
	}{
		{429, `{}`, "操作太频繁，请稍后再试"}, {500, `{}`, "暂时无法创建登录二维码，请重试"}, {200, `bad`, "登录服务返回的二维码无效，请重试"}, {200, `{"id":2}`, "登录服务返回的二维码无效，请重试"}, {200, `{"id":""}`, "登录服务返回的二维码无效，请重试"}, {200, `{"id":"` + strings.Repeat("x", 513) + `"}`, "登录服务返回的二维码无效，请重试"},
	}
	for _, c := range cases {
		a := New(Options{HTTPClient: doFunc(func(*http.Request) (*http.Response, error) { return response(c.code, c.body), nil })})
		_, err := a.Start()
		if err == nil || err.Error() != c.want {
			t.Fatalf("code %d: %v", c.code, err)
		}
	}
	for _, code := range []int{400, 401, 410, 429, 500} {
		a := newTest("WAITING", nil)
		id := startID(t, a)
		a.client = doFunc(func(*http.Request) (*http.Response, error) { return response(code, `{}`), nil })
		r, err := a.Check(id)
		if code == 400 || code == 401 || code == 410 {
			state(t, r, err, "expired")
		} else if err == nil {
			t.Fatal("missing error")
		}
	}
	for _, input := range []string{"BOGUS", "CONFIRMED", "USED"} {
		a := newTest(input, nil)
		id := startID(t, a)
		if _, err := a.Check(id); err == nil {
			t.Fatal("expected missing tokens or unknown status")
		}
	}
	a := New(Options{HTTPClient: doFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("private-credential") })})
	_, err := a.Start()
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("unsanitized network error")
	}
}
func TestExpiryCancelAndReplacement(t *testing.T) {
	now := time.Unix(1000, 0)
	a := newTest("WAITING", nil)
	a.clock = func() time.Time { return now }
	id := startID(t, a)
	a.Cancel("")
	r, e := a.Check(id)
	state(t, r, e, "waiting")
	now = now.Add(180 * time.Second)
	r, e = a.Check(id)
	state(t, r, e, "expired")
	id = startID(t, a)
	next := startID(t, a)
	r, e = a.Check(id)
	state(t, r, e, "expired")
	a.Cancel(id)
	r, e = a.Check(next)
	state(t, r, e, "waiting")
	a.Cancel()
	r, e = a.Check(next)
	state(t, r, e, "expired")
	if len(a.Cancel()) != 1 {
		t.Fatal("cancel JSON")
	}
}
func TestPersistenceRetryCookiesAndCachedSuccess(t *testing.T) {
	var calls, polls int
	a := newTest("CONFIRMED", func(tokens Tokens) (map[string]any, error) {
		calls++
		if tokens.AccessToken != "a+b" || tokens.RefreshToken != "r/=" {
			t.Fatal("cookie decoding")
		}
		if calls == 1 {
			return nil, errors.New("secret a+b")
		}
		return map[string]any{"uid": "u"}, nil
	})
	id := startID(t, a)
	a.client = doFunc(func(r *http.Request) (*http.Response, error) {
		polls++
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["id"] != "remote" {
			t.Fatal(body)
		}
		v := response(200, `{"status":"CONFIRMED"}`)
		v.Header.Add("Set-Cookie", "x-jike-access-token=a%2Bb; Path=/")
		v.Header.Add("Set-Cookie", "x-jike-refresh-token=r%2F%3D; HttpOnly")
		return v, nil
	})
	if _, err := a.Check(id); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("persistence error")
	}
	r, e := a.Check(id)
	state(t, r, e, "success")
	if r["hasRefresh"] != true || len(r) != 4 {
		t.Fatal(r)
	}
	r["user"].(map[string]any)["uid"] = "mutated"
	r, e = a.Check(id)
	state(t, r, e, "success")
	if r["user"].(map[string]any)["uid"] != "u" || calls != 2 || polls != 1 {
		t.Fatal("cache/retry")
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "a+b") || strings.Contains(string(b), "r/=") {
		t.Fatal("token leaked")
	}
}
func TestHeaderTokensNoRefreshAndNilUser(t *testing.T) {
	a := newTest("USED", func(tokens Tokens) (map[string]any, error) {
		if tokens.AccessToken != "header" || tokens.RefreshToken != "" {
			t.Fatal("header priority")
		}
		return nil, nil
	})
	id := startID(t, a)
	a.client = doFunc(func(*http.Request) (*http.Response, error) {
		r := response(200, `{"status":"USED"}`)
		r.Header.Set("x-jike-access-token", "header")
		r.Header.Add("Set-Cookie", "x-jike-access-token=cookie")
		return r, nil
	})
	r, e := a.Check(id)
	state(t, r, e, "success")
	b, _ := json.Marshal(r)
	if string(b) != `{"hasRefresh":false,"ok":true,"status":"success","user":null}` {
		t.Fatal(string(b))
	}
}
func TestCancelInFlight(t *testing.T) {
	for _, start := range []bool{true, false} {
		t.Run(map[bool]string{true: "start", false: "check"}[start], func(t *testing.T) {
			var persisted atomic.Int32
			a := newTest("CONFIRMED", func(Tokens) (map[string]any, error) { persisted.Add(1); return nil, nil })
			id := ""
			if !start {
				id = startID(t, a)
			}
			entered := make(chan struct{})
			done := make(chan struct{})
			a.client = doFunc(func(r *http.Request) (*http.Response, error) {
				close(entered)
				<-r.Context().Done()
				return nil, errors.New("cancelled secret")
			})
			go func() {
				defer close(done)
				var r Result
				var e error
				if start {
					r, e = a.Start()
				} else {
					r, e = a.Check(id)
				}
				state(t, r, e, "expired")
			}()
			<-entered
			a.Cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("cancel did not abort HTTP")
			}
			if persisted.Load() != 0 {
				t.Fatal("persisted cancelled session")
			}
		})
	}
}
func TestConcurrentChecksPersistOnce(t *testing.T) {
	var persisted, polls atomic.Int32
	a := newTest("CONFIRMED", func(Tokens) (map[string]any, error) { persisted.Add(1); return nil, nil })
	id := startID(t, a)
	a.client = doFunc(func(*http.Request) (*http.Response, error) {
		polls.Add(1)
		r := response(200, `{"status":"CONFIRMED"}`)
		r.Header.Set("x-jike-access-token", "test-access")
		return r, nil
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := a.Check(id); state(t, r, e, "success") }()
	}
	wg.Wait()
	if persisted.Load() != 1 || polls.Load() != 1 {
		t.Fatal("duplicate polling or persistence")
	}
}
func TestCancelWaitsForPersistenceCommit(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	a := newTest("USED", func(Tokens) (map[string]any, error) { close(entered); <-release; return nil, nil })
	id := startID(t, a)
	a.client = doFunc(func(*http.Request) (*http.Response, error) {
		r := response(200, `{"status":"USED"}`)
		r.Header.Set("x-jike-access-token", "test")
		return r, nil
	})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = a.Check(id) }()
	<-entered
	cancelled := make(chan struct{})
	go func() { a.Cancel(); close(cancelled) }()
	select {
	case <-cancelled:
		t.Fatal("cancel returned before commit")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-done
	<-cancelled
	r, e := a.Check(id)
	state(t, r, e, "expired")
}
func TestHTTPTimeoutContext(t *testing.T) {
	a := New(Options{HTTPClient: doFunc(func(r *http.Request) (*http.Response, error) {
		d, ok := r.Context().Deadline()
		if !ok || time.Until(d) > 12*time.Second {
			t.Fatal("deadline")
		}
		return nil, context.DeadlineExceeded
	})})
	if _, err := a.Start(); err == nil {
		t.Fatal("timeout accepted")
	}
}

type blockedBody struct {
	ctx     context.Context
	entered chan struct{}
	once    sync.Once
}

func (b *blockedBody) Read([]byte) (int, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (b *blockedBody) Close() error { return nil }
func TestCancelWhileReadingBody(t *testing.T) {
	a := newTest("WAITING", nil)
	id := startID(t, a)
	entered := make(chan struct{})
	a.client = doFunc(func(r *http.Request) (*http.Response, error) {
		v := response(200, "")
		v.Body = &blockedBody{ctx: r.Context(), entered: entered}
		return v, nil
	})
	done := make(chan struct{})
	go func() { defer close(done); r, e := a.Check(id); state(t, r, e, "expired") }()
	<-entered
	cancelled := make(chan struct{})
	go func() { a.Cancel(); close(cancelled) }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("body read blocked cancellation")
	}
	<-done
}
func TestStartReplacesInFlightCheck(t *testing.T) {
	var persisted atomic.Int32
	a := newTest("WAITING", func(Tokens) (map[string]any, error) { persisted.Add(1); return nil, nil })
	id := startID(t, a)
	entered, release := make(chan struct{}), make(chan struct{})
	a.client = doFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/create") {
			return response(200, `{"id":"new"}`), nil
		}
		close(entered)
		<-release
		v := response(200, `{"status":"USED"}`)
		v.Header.Set("x-jike-access-token", "old")
		return v, nil
	})
	done := make(chan struct{})
	go func() { defer close(done); r, e := a.Check(id); state(t, r, e, "expired") }()
	<-entered
	next := startID(t, a)
	close(release)
	<-done
	if next == id || persisted.Load() != 0 {
		t.Fatal("stale result persisted")
	}
}
