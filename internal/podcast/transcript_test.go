package podcast

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestOfficialTranscript(t *testing.T) {
	for _, tc := range []struct {
		name, url              string
		unavailable, wantError bool
		compressed             bool
	}{
		{name: "available", url: "https://transcript.xyzcdn.net/test.json"},
		{name: "gzip without encoding header", url: "https://transcript.xyzcdn.net/test.json", compressed: true},
		{name: "missing", unavailable: true},
		{name: "untrusted CDN", url: "https://example.com/test.json", wantError: true},
		{name: "insecure CDN", url: "http://transcript.xyzcdn.net/test.json", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/episode/" + testID:
					b, _ := json.Marshal(Map{"props": Map{"pageProps": Map{"episode": Map{"transcriptMediaId": "mirrored-media", "media": Map{"id": "external-media"}}}}})
					fmt.Fprintf(w, `<script id="__NEXT_DATA__" type="application/json">%s</script>`, b)
				case "/v1/episode-transcript/get":
					if r.Header.Get("User-Agent") != "Xiaoyuzhou/2.7.0 (build:1234; iOS 17.0.0)" || r.Header.Get("x-jike-app-version") != "2.7.0" {
						t.Fatal("transcript endpoint requires its own app headers; generic playback headers returned unrelated text in live acceptance")
					}
					if r.Header.Get("x-jike-access-token") != "test-token" {
						t.Error("missing API token")
					}
					var body Map
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["mediaId"] != "mirrored-media" {
						t.Error("wrong media ID")
					}
					writeJSON(w, Map{"data": Map{"data": Map{"transcriptUrl": tc.url}}})
				case "/test.json":
					if r.Header.Get("x-jike-access-token") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
						t.Error("credentials leaked to CDN")
					}
					if r.Header.Get("User-Agent") != "Xiaoyuzhou/2.99.1(android 28)" {
						t.Error("wrong CDN UA")
					}
					segments := []Map{{"startMs": 2000, "text": "第二句"}, {"startMs": 0, "text": "第一句"}, {"startMs": -1, "text": "无效"}, {"startMs": 100, "text": " "}}
					if tc.compressed {
						w.Header().Set("Content-Type", "application/json")
						writer := gzip.NewWriter(w)
						_ = json.NewEncoder(writer).Encode(segments)
						_ = writer.Close()
					} else {
						writeJSON(w, segments)
					}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			})
			got, err := c.GetTranscript(context.Background(), "test-token", testID)
			if tc.wantError {
				if err == nil {
					t.Fatal("accepted invalid CDN")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.unavailable {
				if got["status"] != "unavailable" {
					t.Fatal(got)
				}
				return
			}
			segments := got["segments"].([]Map)
			if len(segments) != 2 || segments[0]["text"] != "第一句" {
				t.Fatal(got)
			}
		})
	}
}
