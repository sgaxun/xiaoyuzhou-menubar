package podcast

import (
	"context"
	"fmt"
	"math"
	"time"
)

// playbackProgressBatchSize is a conservative local bound, not an API limit.
const playbackProgressBatchSize = 50

// FinishedHistoryEpisodes reads only the first history page. Only an explicit
// isFinished=true marks completion; missing markers and progress imply nothing.
func (c *Client) FinishedHistoryEpisodes(ctx context.Context, token string) (map[string]bool, error) {
	v, err := c.apiWithHeaders(ctx, "/v1/episode-played/list-history", token, "POST", Map{}, cloudHeaders())
	if err != nil {
		return nil, err
	}
	data, ok := obj(v)["data"].([]any)
	if !ok {
		return nil, fmt.Errorf("播放历史响应不是数组")
	}
	finished := make(map[string]bool)
	for i, item := range data {
		record, ok := item.(Map)
		if !ok {
			return nil, fmt.Errorf("播放历史第 %d 项格式异常", i+1)
		}
		episode, ok := record["episode"].(Map)
		if !ok {
			return nil, fmt.Errorf("播放历史第 %d 项单集格式异常", i+1)
		}
		eid, ok := episode["eid"].(string)
		if !ok || !IsValidID(eid) {
			return nil, fmt.Errorf("播放历史第 %d 项单集 ID 无效", i+1)
		}
		if marker, exists := episode["isFinished"]; exists {
			complete, ok := marker.(bool)
			if !ok {
				return nil, fmt.Errorf("播放历史第 %d 项完成标记无效", i+1)
			}
			if complete {
				finished[eid] = true
			}
		}
	}
	return finished, nil
}

// ListPlaybackProgress reads cloud progress in seconds without inventing missing records.
func (c *Client) ListPlaybackProgress(ctx context.Context, token string, eids []string) ([]Map, error) {
	ids := make([]string, 0, len(eids))
	seen := make(map[string]bool, len(eids))
	for _, id := range eids {
		if !IsValidID(id) {
			return nil, fmt.Errorf("单集 ID 无效")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	result := make([]Map, 0)
	for start := 0; start < len(ids); start += playbackProgressBatchSize {
		end := min(start+playbackProgressBatchSize, len(ids))
		batch := ids[start:end]
		requested := make(map[string]bool, len(batch))
		for _, id := range batch {
			requested[id] = true
		}
		v, err := c.apiWithHeaders(ctx, "/v1/playback-progress/list", token, "POST", Map{"eids": batch}, cloudHeaders())
		if err != nil {
			return nil, err
		}
		data, ok := obj(v)["data"].([]any)
		if !ok {
			return nil, fmt.Errorf("播放进度响应不是数组")
		}
		returned := make(map[string]bool, len(data))
		for i, item := range data {
			record, ok := item.(Map)
			if !ok {
				return nil, fmt.Errorf("播放进度第 %d 项格式异常", i+1)
			}
			eid, eidOK := record["eid"].(string)
			pid, pidOK := record["pid"].(string)
			progress, progressOK := record["progress"].(float64)
			playedAt, atOK := record["playedAt"].(string)
			_, timeErr := time.Parse(time.RFC3339Nano, playedAt)
			if !eidOK || !requested[eid] || returned[eid] || !pidOK || !IsValidID(pid) ||
				!progressOK || math.IsNaN(progress) || math.IsInf(progress, 0) || progress < 0 || progress > 7*24*60*60 || !atOK || timeErr != nil {
				return nil, fmt.Errorf("播放进度第 %d 项字段无效", i+1)
			}
			returned[eid] = true
			result = append(result, Map{"eid": eid, "pid": pid, "progress": progress, "playedAt": playedAt})
		}
	}
	return result, nil
}

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
