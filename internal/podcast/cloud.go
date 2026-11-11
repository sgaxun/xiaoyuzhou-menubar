package podcast

import (
	"context"
	"fmt"
	"time"
)

// PullPlaylist requests a full snapshot, preserving the server's episode order.
func (c *Client) PullPlaylist(ctx context.Context, token string) ([]string, error) {
	v, err := c.apiWithHeaders(ctx, "/v1/playlist/pull", token, "POST", Map{"base": ""}, cloudHeaders())
	if err != nil {
		return nil, err
	}
	d := obj(obj(v)["data"])
	if d["kind"] != "FULL_LIST" {
		return nil, fmt.Errorf("播放列表响应不是完整列表")
	}
	list, ok := d["list"].([]any)
	if !ok {
		return nil, fmt.Errorf("播放列表响应格式异常")
	}
	ids := make([]string, len(list))
	for i, item := range list {
		id, ok := item.(string)
		if !ok || !IsValidID(id) {
			return nil, fmt.Errorf("播放列表第 %d 项单集 ID 无效", i+1)
		}
		ids[i] = id
	}
	return ids, nil
}

// UploadPlaybackProgress uploads one progress record; playedAt is passed unchanged.
func (c *Client) UploadPlaybackProgress(ctx context.Context, token, eid, pid string, progress int, playedAt string) error {
	if !IsValidID(eid) || !IsValidID(pid) {
		return fmt.Errorf("单集或节目 ID 无效")
	}
	if progress < 0 || playedAt == "" {
		return fmt.Errorf("播放进度或播放时间无效")
	}
	_, err := c.apiWithHeaders(ctx, "/v1/playback-progress/update", token, "POST", Map{"data": []Map{{"eid": eid, "pid": pid, "progress": progress, "playedAt": playedAt}}}, cloudHeaders())
	return err
}

// CreatePlayedHistory records an episode in the user's played history.
func (c *Client) CreatePlayedHistory(ctx context.Context, token, eid string) error {
	if !IsValidID(eid) {
		return fmt.Errorf("单集 ID 无效")
	}
	_, err := c.apiWithHeaders(ctx, "/v1/episode-played/create", token, "POST", Map{"eid": eid}, cloudHeaders())
	return err
}

// Match the timestamp offset to its wall clock (the generic legacy header uses UTC).
func cloudHeaders() map[string]string {
	return map[string]string{"Local-Time": time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02T15:04:05.000-07:00"), "Timezone": "Asia/Shanghai"}
}
