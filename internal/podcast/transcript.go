package podcast

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// GetTranscript retrieves only existing official transcripts; it never starts transcription.
func (c *Client) GetTranscript(ctx context.Context, token, eid string) (Map, error) {
	if !IsValidID(eid) {
		return nil, fmt.Errorf("单集 ID 无效")
	}
	page, err := c.page(ctx, "/episode/"+eid)
	if err != nil {
		return nil, err
	}
	episode := obj(page["episode"])
	// Syndicated episodes use a mirrored media ID for transcripts.
	mediaID := str(fallback(episode["transcriptMediaId"], obj(episode["media"])["id"]))
	empty := Map{"status": "unavailable", "segments": []Map{}}
	if mediaID == "" {
		return empty, nil
	}
	response, err := c.apiWithHeaders(ctx, "/v1/episode-transcript/get", token, "POST", Map{"eid": eid, "mediaId": mediaID}, map[string]string{
		"User-Agent":         "Xiaoyuzhou/2.7.0 (build:1234; iOS 17.0.0)",
		"x-jike-app-version": "2.7.0",
		"x-jike-device-id":   "00000000-0000-0000-0000-000000000000",
	})
	if err != nil {
		return nil, err
	}
	data := obj(obj(response)["data"])
	if nested, ok := data["data"].(map[string]any); ok {
		data = nested
	}
	source := str(data["transcriptUrl"])
	if source == "" {
		return empty, nil
	}
	u, err := url.Parse(source)
	if err != nil || !validURL(u) || u.Hostname() != "transcript.xyzcdn.net" {
		return nil, fmt.Errorf("字幕下载地址无效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("字幕下载地址无效")
	}
	req.Header.Set("User-Agent", "Xiaoyuzhou/2.99.1(android 28)")
	hc := http.Client{Timeout: 30 * time.Second}
	if c.HTTPClient != nil {
		hc = *c.HTTPClient
	}
	hc.Jar = nil
	hc.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || !validURL(r.URL) || r.URL.Hostname() != u.Hostname() {
			return fmt.Errorf("不允许的字幕跳转地址")
		}
		return nil
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("字幕下载失败，请重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("字幕下载失败（%d），请重试", resp.StatusCode)
	}
	const maxSize = 16 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil || len(body) > maxSize {
		return nil, fmt.Errorf("字幕内容过大或读取失败")
	}
	// Some transcript responses are gzipped without Content-Encoding.
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		reader, gzipErr := gzip.NewReader(bytes.NewReader(body))
		if gzipErr != nil {
			return nil, fmt.Errorf("字幕压缩格式无法识别")
		}
		body, err = io.ReadAll(io.LimitReader(reader, maxSize+1))
		reader.Close()
		if err != nil || len(body) > maxSize {
			return nil, fmt.Errorf("字幕内容过大或读取失败")
		}
	}
	var raw []Map
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("字幕格式无法识别")
	}
	segments := []Map{}
	for _, segment := range raw {
		text := strings.TrimSpace(str(segment["text"]))
		start, exists := segment["startMs"]
		ms := number(start)
		if !exists || text == "" || ms < 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
			continue
		}
		segments = append(segments, Map{"startMs": ms, "text": text})
	}
	sort.SliceStable(segments, func(i, j int) bool { return number(segments[i]["startMs"]) < number(segments[j]["startMs"]) })
	if len(segments) == 0 {
		return empty, nil
	}
	return Map{"status": "available", "segments": segments}, nil
}
