package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestEpisodeShareURL(t *testing.T) {
	id := "0123456789abcdef01234567"
	got, err := episodeShareURL(id)
	if err != nil || got != "https://www.xiaoyuzhoufm.com/episode/"+id {
		t.Fatalf("%q %v", got, err)
	}
	for _, bad := range []string{"", "../recipient", id + "?t=10", id + "\n"} {
		if _, err := episodeShareURL(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
func TestShareTextAndTelegramEscaping(t *testing.T) {
	link, _ := episodeShareURL("0123456789abcdef01234567")
	text, err := episodeShareText("标题 & ? #", "播客", 3661.9, true)
	if err != nil || text != "标题 & ? # — 播客\n从 1:01:01 开始听" {
		t.Fatalf("%q %v", text, err)
	}
	for _, desktop := range []bool{false, true} {
		u, err := url.Parse(telegramShareURL(link, text, desktop))
		if err != nil {
			t.Fatal(err)
		}
		if u.Query().Get("url") != link || u.Query().Get("text") != text || len(u.Query()) != 2 {
			t.Fatal(u)
		}
		if desktop {
			if u.Scheme != "tg" || u.Host != "msg_url" {
				t.Fatal(u)
			}
		} else if u.Scheme != "https" || u.Host != "t.me" || u.Path != "/share/url" {
			t.Fatal(u)
		}
	}
	text, _ = episodeShareText("标题", "", 59.9, false)
	if text != "标题" {
		t.Fatal(text)
	}
	text, _ = episodeShareText("标题", "", 59.9, true)
	if !strings.HasSuffix(text, "00:59 开始听") {
		t.Fatal(text)
	}
	for _, p := range []float64{-1, math.NaN(), math.Inf(1), 604801} {
		if _, err := episodeShareText("a", "b", p, false); err == nil {
			t.Fatalf("accepted %v", p)
		}
	}
	for _, s := range []string{strings.Repeat("中", 501), "x\ny", string([]byte{255})} {
		if _, err := episodeShareText(s, "", 0, false); err == nil {
			t.Fatal("accepted invalid title")
		}
	}
}
func TestDecodeSharePNG(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	got, err := decodeSharePNG(dataURL)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("%v", err)
	}
	corrupt := append([]byte(nil), buf.Bytes()...)
	corrupt[len(corrupt)-5] ^= 255
	for _, s := range []string{"", "data:image/jpeg;base64,AAAA", "data:image/png;base64,???", "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not PNG")), "data:image/png;base64," + base64.StdEncoding.EncodeToString(corrupt), dataURL[:len(dataURL)-8], "data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(maxSharePNG)+4)} {
		if _, err := decodeSharePNG(s); err == nil {
			t.Fatal("accepted invalid PNG")
		}
	}
}
func TestInvalidRequestsDoNotInvokeNativeAPI(t *testing.T) {
	b := &Backend{}
	if err := b.CopyEpisodeLink("bad"); err == nil {
		t.Fatal("invalid ID")
	}
	if err := b.ShareEpisode("recipient", "0123456789abcdef01234567", "a", "b", 0, false); err == nil {
		t.Fatal("invalid destination")
	}
	if saved, err := b.SaveShareCard("bad"); saved || err == nil {
		t.Fatal("invalid card")
	}
	if err := b.ShareCard("bad"); err == nil {
		t.Fatal("invalid card")
	}
}

// A real child process tests the READY protocol without opening any app or
// touching a recipient. Returning must not wait for the child to exit.
func TestShareHelperHandshake(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test helper")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat >/dev/null\necho READY\nsleep 2\n"), 0700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := runShareCommand(path, []byte("{}"), false); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) >= 1500*time.Millisecond {
		t.Fatal("waited for UI child to exit")
	}
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\ncat >/dev/null\necho 'service unavailable' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := runShareCommand(bad, []byte("{}"), false); err == nil || !strings.Contains(err.Error(), "service unavailable") {
		t.Fatalf("%v", err)
	}
}
