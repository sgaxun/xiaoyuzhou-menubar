package player

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"xiaoyuzhou-menubar/internal/podcast"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func TestAutomaticRefreshAndAccountSwitch(t *testing.T) {
	refreshes := 0
	client := podcast.NewClient()
	client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/app_auth_tokens.refresh":
			refreshes++
			resp := reply(200, `{}`)
			resp.Header.Set("x-jike-access-token", "new-test-access")
			resp.Header.Set("x-jike-refresh-token", "new-test-refresh")
			return resp, nil
		case "/v1/subscription/list":
			if r.Header.Get("x-jike-access-token") == "old-test-access" {
				return reply(401, `{}`), nil
			}
			return reply(200, `{"data":[{"pid":"0123456789abcdef01234567","title":"test podcast"}]}`), nil
		case "/web/user/get-me":
			return reply(200, `{"data":{"uid":"test-user","nickname":"test user"}}`), nil
		}
		t.Fatalf("unexpected request: %s", r.URL.Path)
		return nil, nil
	})}
	s, err := NewWithClient(t.TempDir(), client)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.writeTokens("old-test-access", "old-test-refresh"); err != nil {
		t.Fatal(err)
	}
	result := call(t, s, "subscriptions?refresh=1", nil)
	if !flag(result["ok"]) || len(list(result["list"])) != 1 || refreshes != 1 {
		t.Fatal("refresh/subscription contract failed")
	}
	a, r, err := s.tokens()
	if err != nil || a != "new-test-access" || r != "new-test-refresh" {
		t.Fatal("refresh not persisted")
	}
	result = call(t, s, "login", object{"cookie": "x-jike-access-token=account-b-test; x-jike-refresh-token=b-refresh"})
	if !flag(result["ok"]) {
		t.Fatal(result)
	}
	if result = call(t, s, "subsCache", nil); flag(result["hasCache"]) {
		t.Fatal("old account subscription cache leaked")
	}
	public, _ := json.Marshal(call(t, s, "state", nil))
	if strings.Contains(string(public), "test-access") {
		t.Fatal("token returned")
	}
}
