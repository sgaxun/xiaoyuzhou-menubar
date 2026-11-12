package player

import (
	"errors"
	"math"
	"time"
)

// Called under mu as part of the queue's atomic commit. Importing a resume point
// is not listening: it creates neither a history entry nor an upload task.
func (s *Service) mergeCloudPlaylistProgress(next, body object) (object, error) {
	preview := s.cloudPlaylistPreview
	created, _ := time.Parse(time.RFC3339Nano, str(preview["createdAt"]))
	epoch, ok := preview["epoch"].(uint64)
	if !ok || epoch != s.accountEpoch || str(preview["id"]) != str(body["cloudImportId"]) || time.Since(created) > 5*time.Minute {
		return nil, errors.New("拉取结果已失效，请重新拉取云端列表")
	}
	queued := map[string]object{}
	for _, v := range list(next["queue"]) {
		ep := obj(v)
		queued[str(ep["eid"])] = ep
	}
	progress := obj(next["progress"])
	applied := object{}
	account := str(obj(s.state["user"])["uid"])
	tasks := obj(obj(s.state["cloudSync"])["tasks"])
	for _, v := range list(preview["records"]) {
		record := obj(v)
		id := str(record["eid"])
		ep := queued[id]
		if ep == nil || id == str(preview["protectedEid"]) || id == str(body["protectedEid"]) || (id == s.cloudPlayback && s.cloudPlaybackEvent == "playing") || len(obj(tasks[account+":"+id])) > 0 {
			continue
		}
		remoteAt, e := time.Parse(time.RFC3339Nano, str(record["playedAt"]))
		if e != nil {
			continue
		}
		localAt, _ := time.Parse(time.RFC3339Nano, str(obj(progress[id])["updatedAt"]))
		if !localAt.IsZero() && !remoteAt.After(localAt) {
			continue
		}
		position := num(record["progress"])
		if math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 7*24*3600 {
			continue
		}
		if duration := num(ep["duration"]); duration > 0 && position > duration {
			position = duration
		}
		entry := object{"position": position, "updatedAt": remoteAt.UTC().Format(time.RFC3339Nano)}
		progress[id] = entry
		applied[id] = entry
	}
	next["progress"] = progress
	return applied, nil
}

// A server-confirmed completion applies to every local queue item, regardless
// of origin or presence in the current remote playlist. Only active playback is protected.
func (s *Service) mergeCloudQueueRemoval(next, body object) {
	removed := map[string]bool{}
	for _, v := range list(s.cloudPlaylistPreview["removeIDs"]) {
		removed[str(v)] = true
	}
	queue := []any{}
	current := str(body["currentEid"])
	for _, v := range list(next["queue"]) {
		id := str(obj(v)["eid"])
		playing := id == str(body["playingEid"]) || (id == s.cloudPlayback && s.cloudPlaybackEvent == "playing")
		if removed[id] && !playing {
			continue
		}
		queue = append(queue, v)
	}
	next["queue"] = queue
	index := -1
	for i, v := range queue {
		if str(obj(v)["eid"]) == current {
			index = i
			break
		}
	}
	next["queueIndex"] = index
}
