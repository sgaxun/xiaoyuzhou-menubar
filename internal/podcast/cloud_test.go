package podcast

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestPullPlaylist(t *testing.T) {
	for _, tc := range []struct {
		name string
		data any
		want []string
		bad  bool
	}{
		{"ordered", Map{"kind": "FULL_LIST", "list": []string{testID, "abcdef0123456789abcdef01"}}, []string{testID, "abcdef0123456789abcdef01"}, false},
		{"empty", Map{"kind": "FULL_LIST", "list": []string{}}, []string{}, false},
		{"delta", Map{"kind": "DIFF", "list": []string{}}, nil, true},
		{"missing", Map{"kind": "FULL_LIST"}, nil, true},
		{"null", Map{"kind": "FULL_LIST", "list": nil}, nil, true},
		{"object", Map{"kind": "FULL_LIST", "list": Map{}}, nil, true},
		{"nonstring", Map{"kind": "FULL_LIST", "list": []any{testID, 1}}, nil, true},
		{"invalidID", Map{"kind": "FULL_LIST", "list": []string{""}}, nil, true},
		{"missingData", nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				var body Map
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if r.Method != "POST" || r.URL.Path != "/v1/playlist/pull" || !reflect.DeepEqual(body, Map{"base": ""}) || r.Header.Get("x-jike-access-token") != "test-token" {
					t.Error("incorrect playlist request")
				}
				writeJSON(w, Map{"data": tc.data})
			})
			got, err := c.PullPlaylist(context.Background(), "test-token")
			if (err != nil) != tc.bad || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, %v; want %v, bad=%v", got, err, tc.want, tc.bad)
			}
		})
	}
}

func TestCloudWrites(t *testing.T) {
	playedAt := "2026-11-11T12:34:56.789+08:00"
	for _, history := range []bool{false, true} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			var body Map
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			path := "/v1/playback-progress/update"
			want := Map{"data": []any{Map{"eid": testID, "pid": testID, "progress": float64(123), "playedAt": playedAt}}}
			if history {
				path = "/v1/episode-played/create"
				want = Map{"eid": testID}
			}
			if r.Method != "POST" || r.URL.Path != path || !reflect.DeepEqual(body, want) {
				t.Errorf("incorrect write: %s %#v", r.URL.Path, body)
			}
			if r.Header.Get("x-jike-access-token") != "test-token" || r.Header.Get("Local-Time") == "" || r.Header.Get("Timezone") != "Asia/Shanghai" {
				t.Error("missing write headers")
			}
			writeJSON(w, Map{})
		})
		var err error
		if history {
			err = c.CreatePlayedHistory(context.Background(), "test-token", testID)
		} else {
			err = c.UploadPlaybackProgress(context.Background(), "test-token", testID, testID, 123, playedAt)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestCloudErrors(t *testing.T) {
	operations := []func(*Client, context.Context, string) error{
		func(c *Client, ctx context.Context, token string) error {
			_, err := c.PullPlaylist(ctx, token)
			return err
		},
		func(c *Client, ctx context.Context, token string) error {
			return c.UploadPlaybackProgress(ctx, token, testID, testID, 0, "2026-11-11T00:00:00Z")
		},
		func(c *Client, ctx context.Context, token string) error {
			return c.CreatePlayedHistory(ctx, token, testID)
		},
	}
	for _, op := range operations {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
		if err := op(c, context.Background(), "test-token"); !IsUnauthorized(err) {
			t.Fatalf("expected unauthorized, got %v", err)
		}
		if err := op(c, context.Background(), ""); err == nil {
			t.Fatal("accepted missing token")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := op(c, ctx, "test-token"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	}
}

func TestCloudInvalidWrites(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid write reached server") })
	for _, args := range []struct {
		eid, pid string
		progress int
		at       string
	}{
		{"bad", testID, 0, "now"}, {testID, "bad", 0, "now"}, {testID, testID, -1, "now"}, {testID, testID, 0, ""},
	} {
		if err := c.UploadPlaybackProgress(context.Background(), "test-token", args.eid, args.pid, args.progress, args.at); err == nil {
			t.Fatal("accepted invalid progress")
		}
	}
	if err := c.CreatePlayedHistory(context.Background(), "test-token", "bad"); err == nil {
		t.Fatal("accepted invalid episode")
	}
}
