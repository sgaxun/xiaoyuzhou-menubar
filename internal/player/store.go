// Package player implements the menu-bar application's local backend.
// Files keep the previous on-disk JSON shape so upgrading preserves user data.
package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type object = map[string]any

func obj(v any) object {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return object{}
}
func list(v any) []any {
	switch a := v.(type) {
	case []any:
		if a != nil {
			return a
		}
	case []map[string]any:
		out := make([]any, len(a))
		for i, x := range a {
			out[i] = x
		}
		return out
	}
	return []any{}
}
func str(v any) string { s, _ := v.(string); return s }
func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
func flag(v any) bool { b, _ := v.(bool); return b }
func nowISO() string  { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
func clone(m object) object {
	b, _ := json.Marshal(m)
	var out object
	_ = json.Unmarshal(b, &out)
	return out
}
func readObject(file string) (object, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return object{}, nil
		}
		return nil, err
	}
	var v object
	if err = json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("本地数据无法解析：%w", err)
	}
	if v == nil {
		v = object{}
	}
	return v, nil
}
func writeJSON(file string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(file), ".player-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, file)
}
func defaultState() object {
	return object{"version": 1, "queue": []any{}, "queueIndex": 0, "history": []any{}, "progress": object{}, "volume": 0.8, "rate": 1, "panelWidth": 440, "panelHeight": 600, "user": nil, "syncedAt": ""}
}
func (s *Service) saveState(next object) error {
	if err := writeJSON(filepath.Join(s.dir, "state.json"), next); err != nil {
		return err
	}
	s.state = next
	return nil
}
func (s *Service) saveCache() error { return writeJSON(filepath.Join(s.dir, "cache.json"), s.cache) }
func (s *Service) clearAccountCache() error {
	s.accountEpoch++
	for _, cancel := range s.commentCancels {
		cancel()
	}
	if err := s.resetEpisodeUpdates(); err != nil {
		return err
	}
	delete(s.cache, "subs")
	delete(s.cache, "discover")
	for k := range s.cache {
		if len(k) > 7 && k[:7] == "search:" {
			delete(s.cache, k)
		}
	}
	return s.saveCache()
}
func (s *Service) progress(body object) (object, error) {
	eid := str(body["eid"])
	if eid == "" {
		return nil, errors.New("缺少 eid")
	}
	next := clone(s.state)
	duration, position := num(body["duration"]), num(body["position"])
	if position < 0 {
		position = 0
	}
	progress := obj(next["progress"])
	previous := num(obj(progress[eid])["position"])
	if !flag(body["reset"]) && !(duration > 0 && position >= duration-15) && position < previous {
		position = previous
	}
	at := nowISO()
	progress[eid] = object{"position": position, "updatedAt": at}
	next["progress"] = progress
	history := []any{}
	var previousItem object
	for _, v := range list(next["history"]) {
		if str(obj(v)["eid"]) == eid {
			previousItem = obj(v)
		}
	}
	item := object{"eid": eid, "position": position, "updatedAt": at, "synced": false, "syncedAt": ""}
	for _, k := range []string{"pid", "title", "podcastTitle", "image"} {
		v := str(body[k])
		if v == "" {
			v = str(previousItem[k])
		}
		item[k] = v
	}
	if duration == 0 {
		duration = num(previousItem["duration"])
	}
	item["duration"] = duration
	// Source metadata is one group: do not mix a new source with old labels.
	source := body
	if str(source["subscriptionPid"]) == "" {
		source = previousItem
	}
	if str(source["subscriptionPid"]) != "" {
		for _, k := range []string{"subscriptionPid", "subscriptionTitle", "subscriptionImage", "isCoCreated"} {
			if v, exists := source[k]; exists {
				item[k] = v
			}
		}
	}
	item = s.historySubscriptionSource(item)
	history = append(history, item)
	for _, v := range list(next["history"]) {
		if str(obj(v)["eid"]) != eid && len(history) < 300 {
			history = append(history, v)
		}
	}
	next["history"] = history
	if err := s.saveState(next); err != nil {
		return nil, err
	}
	return object{"ok": true, "position": position}, nil
}
func (s *Service) cached(key, kind string, force bool, load func() (any, error)) (any, error) {
	ttl := 30 * time.Minute
	switch kind {
	case "search":
		ttl = time.Hour
	case "podcast", "episode":
		ttl = 6 * time.Hour
	}
	entry := obj(s.cache[key])
	valid := true
	if key == "discover" {
		v := obj(entry["value"])
		valid = len(list(v["tops"])) > 0 || len(list(v["picks"])) > 0
	}
	if !force && valid && entry["value"] != nil && float64(time.Now().UnixMilli())-num(entry["at"]) < float64(ttl.Milliseconds()) {
		return entry["value"], nil
	}
	value, err := load()
	if err != nil {
		return nil, err
	}
	s.cache[key] = object{"value": value, "at": time.Now().UnixMilli()}
	prefix := ""
	limit := 0
	for p, n := range map[string]int{"podcast:": 20, "episode:": 30, "search:": 10, "notes:": 20, "transcript:": 10, "transcript-v2:": 10} {
		if len(key) >= len(p) && key[:len(p)] == p {
			prefix = p
			limit = n
			break
		}
	}
	if limit > 0 {
		keys := []string{}
		for k := range s.cache {
			if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
				keys = append(keys, k)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return num(obj(s.cache[keys[i]])["at"]) < num(obj(s.cache[keys[j]])["at"]) })
		for len(keys) > limit {
			delete(s.cache, keys[0])
			keys = keys[1:]
		}
	}
	if err = s.saveCache(); err != nil {
		return nil, err
	}
	return value, nil
}
