package player

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"xiaoyuzhou-menubar/internal/podcast"
)

const commentTestEID = "6aeb10f797726cd4675cbe17"

func TestSlowCommentsDoNotBlockStateOrProgress(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	client := podcast.NewClient()
	client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		select {
		case <-release:
			return reply(200, `{"data":[]}`), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})}
	s, err := NewWithClient(t.TempDir(), client)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.writeTokens("test-access", ""); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.Request("comments", object{"eid": commentTestEID}) }()
	<-started
	local := make(chan struct{})
	go func() {
		defer close(local)
		_, _ = s.Request("state", nil)
		_, _ = s.Request("progress", object{"eid": commentTestEID, "position": 30.0})
	}()
	select {
	case <-local:
	case <-time.After(time.Second):
		t.Fatal("comment network call held global state lock")
	}
	// Account change must cancel and discard in-flight responses.
	if r := call(t, s, "logout", nil); !flag(r["ok"]) {
		t.Fatal(r)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("logout did not cancel comments")
	}
}

func TestCommentRefreshCannotRestoreLoggedOutAccount(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	client := podcast.NewClient()
	client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v1/comment/list-primary" {
			return reply(401, `{}`), nil
		}
		close(started)
		// Emulate a refresh transport that finishes despite cancellation.
		<-release
		response := reply(200, `{}`)
		response.Header.Set("x-jike-access-token", "refreshed-access")
		response.Header.Set("x-jike-refresh-token", "refreshed-refresh")
		return response, nil
	})}
	s, err := NewWithClient(t.TempDir(), client)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.writeTokens("old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}
	done := make(chan object, 1)
	go func() { v, _ := s.Request("comments", object{"eid": commentTestEID}); done <- obj(v) }()
	<-started
	if r := call(t, s, "logout", nil); !flag(r["ok"]) {
		t.Fatal(r)
	}
	close(release)
	select {
	case r := <-done:
		if flag(r["ok"]) || !strings.Contains(str(r["error"]), "账号状态已变化") {
			t.Fatal("stale response accepted", r)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh did not finish")
	}
	a, r, err := s.tokens()
	if err != nil || a != "" || r != "" {
		t.Fatal("refresh restored logged-out credentials")
	}
}

func TestConcurrentCommentPagesShareRefresh(t *testing.T) {
	var refreshes atomic.Int32
	barrier := make(chan struct{})
	var oldCalls atomic.Int32
	client := podcast.NewClient()
	client.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/app_auth_tokens.refresh" {
			refreshes.Add(1)
			response := reply(200, `{}`)
			response.Header.Set("x-jike-access-token", "new-access")
			response.Header.Set("x-jike-refresh-token", "new-refresh")
			return response, nil
		}
		if r.Header.Get("x-jike-access-token") == "old-access" {
			if oldCalls.Add(1) == 2 {
				close(barrier)
			}
			<-barrier
			return reply(401, `{}`), nil
		}
		return reply(200, `{"data":[]}`), nil
	})}
	s, err := NewWithClient(t.TempDir(), client)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.writeTokens("old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan object, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _ := s.Request("comments", object{"eid": commentTestEID})
			results <- obj(v)
		}()
	}
	wg.Wait()
	close(results)
	for r := range results {
		if !flag(r["ok"]) {
			t.Fatal(r)
		}
	}
	if refreshes.Load() != 1 {
		t.Fatalf("duplicate refreshes: %d", refreshes.Load())
	}
}
