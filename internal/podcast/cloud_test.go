package podcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"
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
			_, err := c.FinishedHistoryEpisodes(ctx, token)
			return err
		},
		func(c *Client, ctx context.Context, token string) error {
			_, err := c.ListPlaybackProgress(ctx, token, []string{testID})
			return err
		},
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

func TestFinishedHistoryEpisodes(t *testing.T) {
	otherID := "abcdef0123456789abcdef01"
	for _, tc := range []struct {
		name string
		data any
		want map[string]bool
	}{
		{"true", []Map{{"episode": Map{"eid": testID, "isFinished": true, "isPlayed": true, "status": "NORMAL"}, "recentAudiences": []any{}}}, map[string]bool{testID: true}},
		{"false", []Map{{"episode": Map{"eid": testID, "isFinished": false, "isPlayed": true, "progress": 0}}}, map[string]bool{}},
		{"missingMarker", []Map{{"episode": Map{"eid": testID, "isPlayed": true, "progress": 0}}}, map[string]bool{}},
		{"mixed", []Map{{"episode": Map{"eid": testID, "isFinished": true}}, {"episode": Map{"eid": otherID, "isFinished": false}}}, map[string]bool{testID: true}},
		{"empty", []any{}, map[string]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body Map
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" || r.URL.Path != "/v1/episode-played/list-history" || !reflect.DeepEqual(body, Map{}) || r.URL.RawQuery != "" {
					t.Errorf("incorrect history request: %s %s %#v", r.Method, r.URL, body)
				}
				if r.Header.Get("x-jike-access-token") != "test-token" || r.Header.Get("Timezone") != "Asia/Shanghai" {
					t.Error("missing history headers")
				}
				writeJSON(w, Map{"data": tc.data, "loadMoreKey": Map{"id": "next-page"}})
			})
			got, err := c.FinishedHistoryEpisodes(context.Background(), "test-token")
			if err != nil || !reflect.DeepEqual(got, tc.want) || calls != 1 {
				t.Fatalf("got %#v, %v (%d calls); want %#v", got, err, calls, tc.want)
			}
		})
	}
}

func TestFinishedHistoryEpisodesMalformed(t *testing.T) {
	valid := Map{"episode": Map{"eid": testID, "isFinished": true}}
	cases := map[string]any{
		"nullData": nil, "objectData": Map{}, "stringData": "bad",
	}
	for name, item := range map[string]any{
		"nullItem": nil, "arrayItem": []any{}, "stringItem": "bad",
		"missingEpisode": Map{}, "nullEpisode": Map{"episode": nil},
		"arrayEpisode": Map{"episode": []any{}}, "stringEpisode": Map{"episode": "bad"},
		"missingID": Map{"episode": Map{"isFinished": true}},
	} {
		cases[name] = []any{valid, item}
	}
	for field, values := range map[string][]any{
		"eid":        {nil, 1, "", "bad", " " + testID},
		"isFinished": {nil, "true", 1, Map{}, []any{}},
	} {
		for i, value := range values {
			episode := Map{"eid": testID, "isFinished": true}
			episode[field] = value
			cases[fmt.Sprintf("%s%d", field, i)] = []any{valid, Map{"episode": episode}}
		}
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, Map{"data": data}) })
			got, err := c.FinishedHistoryEpisodes(context.Background(), "test-token")
			if err == nil || got != nil {
				t.Fatalf("accepted malformed or partial response: %#v, %v", got, err)
			}
		})
	}
	for _, raw := range []string{`{}`, `{`, `{"data":[NaN]}`} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(raw)) })
		got, err := c.FinishedHistoryEpisodes(context.Background(), "test-token")
		if err == nil || got != nil {
			t.Fatalf("accepted %s: %#v, %v", raw, got, err)
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

func TestListPlaybackProgressBatches(t *testing.T) {
	ids := make([]string, 103)
	for i := range ids {
		ids[i] = fmt.Sprintf("%024x", i)
	}
	input := append(append([]string{}, ids...), ids[0], ids[50])
	calls := 0
	at := "2026-11-11T12:34:56.789+08:00"
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Eids []string `json:"eids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		start := calls * 50
		end := min(start+50, len(ids))
		if r.Method != "POST" || r.URL.Path != "/v1/playback-progress/list" || !reflect.DeepEqual(body.Eids, ids[start:end]) {
			t.Errorf("incorrect batch: %#v", body)
		}
		if r.Header.Get("x-jike-access-token") != "test-token" || r.Header.Get("Timezone") != "Asia/Shanghai" {
			t.Error("missing headers")
		}
		if _, err := time.Parse(time.RFC3339Nano, r.Header.Get("Local-Time")); err != nil {
			t.Error(err)
		}
		calls++
		data := []Map{}
		for _, id := range body.Eids {
			data = append(data, Map{"eid": id, "pid": testID, "progress": 0, "playedAt": at, "extra": true})
		}
		writeJSON(w, Map{"data": data})
	})
	got, err := c.ListPlaybackProgress(context.Background(), "test-token", input)
	if err != nil || calls != 3 || len(got) != len(ids) {
		t.Fatalf("got %d records, %d calls, %v", len(got), calls, err)
	}
	for i, record := range got {
		want := Map{"eid": ids[i], "pid": testID, "progress": float64(0), "playedAt": at}
		if !reflect.DeepEqual(record, want) {
			t.Fatalf("record %d: %#v", i, record)
		}
	}
}

func TestListPlaybackProgressNoNetwork(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected network request") })
	for _, ids := range [][]string{nil, {}} {
		got, err := c.ListPlaybackProgress(context.Background(), "", ids)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("empty: %#v %v", got, err)
		}
	}
	for _, ids := range [][]string{{"bad"}, {testID, ""}, {testID, " " + testID}} {
		if _, err := c.ListPlaybackProgress(context.Background(), "test-token", ids); err == nil {
			t.Fatal("accepted invalid ID")
		}
	}
}

func TestListPlaybackProgressValidation(t *testing.T) {
	valid := Map{"eid": testID, "pid": testID, "progress": 12.5, "playedAt": "2026-11-11T00:00:00Z"}
	cases := map[string]any{"null": nil, "object": Map{}, "string": "bad", "recordNull": []any{nil}, "recordArray": []any{[]any{}}, "duplicate": []any{valid, valid}}
	for field, values := range map[string][]any{
		"eid":      {nil, 1, "bad", "abcdef0123456789abcdef01"},
		"pid":      {nil, 1, "bad"},
		"progress": {nil, "0", true, -1, 604801},
		"playedAt": {nil, 1, "", "now", "2026-11-11", "2026-02-30T00:00:00Z"},
	} {
		for i, value := range values {
			record := Map{}
			for k, v := range valid {
				record[k] = v
			}
			record[field] = value
			cases[fmt.Sprintf("%s%d", field, i)] = []Map{record}
		}
		record := Map{}
		for k, v := range valid {
			if k != field {
				record[k] = v
			}
		}
		cases[field+"Missing"] = []Map{record}
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, Map{"data": data}) })
			got, err := c.ListPlaybackProgress(context.Background(), "test-token", []string{testID})
			if err == nil || got != nil {
				t.Fatalf("accepted malformed response: %#v %v", got, err)
			}
		})
	}
	for _, progress := range []float64{0, 12.5, 604800} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			record := Map{}
			for k, v := range valid {
				record[k] = v
			}
			record["progress"] = progress
			writeJSON(w, Map{"data": []Map{record}})
		})
		got, err := c.ListPlaybackProgress(context.Background(), "test-token", []string{testID})
		if err != nil || len(got) != 1 || got[0]["progress"] != progress {
			t.Fatalf("valid progress: %#v %v", got, err)
		}
	}
}

func TestListPlaybackProgressResponseEdges(t *testing.T) {
	for _, raw := range []string{`{}`, `{"data":null}`, `{"data":[{"eid":"0123456789abcdef01234567","pid":"0123456789abcdef01234567","progress":1e999,"playedAt":"2026-11-11T00:00:00Z"}]}`, `{"data":[NaN]}`, `{`} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(raw)) })
		got, err := c.ListPlaybackProgress(context.Background(), "test-token", []string{testID})
		if err == nil || got != nil {
			t.Fatalf("accepted %s: %#v %v", raw, got, err)
		}
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, Map{"data": []any{}}) })
	got, err := c.ListPlaybackProgress(context.Background(), "test-token", []string{testID})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty server array: %#v %v", got, err)
	}
	for _, failure := range []string{"malformed", "http", "otherBatchID"} {
		t.Run(failure, func(t *testing.T) {
			ids := make([]string, 51)
			for i := range ids {
				ids[i] = fmt.Sprintf("%024x", i)
			}
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				record := Map{"eid": ids[0], "pid": testID, "progress": 1, "playedAt": "2026-11-11T00:00:00Z"}
				if calls == 1 || failure == "otherBatchID" {
					writeJSON(w, Map{"data": []Map{record}})
					return
				}
				if failure == "http" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				writeJSON(w, Map{"data": Map{}})
			})
			got, err := c.ListPlaybackProgress(context.Background(), "test-token", ids)
			if err == nil || got != nil || calls != 2 {
				t.Fatalf("partial result: %#v %v (%d calls)", got, err, calls)
			}
		})
	}
}
