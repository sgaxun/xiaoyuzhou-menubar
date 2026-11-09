package player

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxLocalBackupBytes = 8 << 20
const maxLibraryEntries = 10000

func emptyLibrary() object {
	return object{"bookmarks": object{}, "completed": object{}, "podcastRates": object{}}
}
func localString(v any, limit int, required bool) bool {
	s, ok := v.(string)
	return ok && utf8.ValidString(s) && len(s) <= limit && !strings.ContainsRune(s, 0) && (!required || strings.TrimSpace(s) != "")
}
func localID(s string) bool { return localString(s, 128, true) }
func localNumber(v any, low, high float64) bool {
	n, ok := v.(float64)
	return ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= low && n <= high
}
func localFields(m object, fields ...string) bool {
	allowed := map[string]bool{}
	for _, k := range fields {
		allowed[k] = true
	}
	for k := range m {
		if !allowed[k] {
			return false
		}
	}
	return true
}
func localTime(v any) bool {
	if s, ok := v.(string); ok {
		_, err := time.Parse(time.RFC3339Nano, s)
		return len(s) <= 64 && err == nil
	}
	return localNumber(v, 0, 8640000000000000)
}

// localEpisode deliberately copies only display metadata, never media URLs or credentials.
func localEpisode(v any) (object, error) {
	m, ok := v.(map[string]any)
	if !ok || !localID(str(m["eid"])) {
		return nil, fmt.Errorf("单集信息无效")
	}
	out := object{"eid": m["eid"]}
	for _, k := range []string{"pid", "title", "podcastTitle", "image", "pubDate", "subscriptionPid", "subscriptionTitle", "subscriptionImage"} {
		if x, exists := m[k]; exists {
			limit := 2048
			if k == "pid" || k == "subscriptionPid" {
				limit = 128
			}
			if !localString(x, limit, false) {
				return nil, fmt.Errorf("单集字段 %s 无效", k)
			}
			out[k] = x
		}
	}
	if x, exists := m["duration"]; exists {
		if !localNumber(x, 0, 7*24*3600) {
			return nil, fmt.Errorf("单集时长无效")
		}
		out["duration"] = x
	}
	for _, k := range []string{"isPaid", "isCoCreated"} {
		if x, exists := m[k]; exists {
			if _, ok := x.(bool); !ok {
				return nil, fmt.Errorf("单集付费标记无效")
			}
			out[k] = x
		}
	}
	return out, nil
}
func validateLibrary(v any) (object, error) {
	m, ok := v.(map[string]any)
	if !ok || !localFields(m, "bookmarks", "completed", "podcastRates") {
		return nil, fmt.Errorf("本地资料格式无效")
	}
	out := emptyLibrary()
	total := 0
	for _, k := range []string{"bookmarks", "completed", "podcastRates"} {
		entries, ok := m[k].(map[string]any)
		if !ok || len(entries) > maxLibraryEntries {
			return nil, fmt.Errorf("%s 必须为对象且不超过 %d 项", k, maxLibraryEntries)
		}
		target := obj(out[k])
		for id, v := range entries {
			if !localID(id) {
				return nil, fmt.Errorf("%s ID 无效", k)
			}
			switch k {
			case "completed":
				if _, ok := v.(bool); !ok {
					return nil, fmt.Errorf("完成状态必须为布尔值")
				}
				target[id] = v
			case "podcastRates":
				if !localNumber(v, 0.25, 4) {
					return nil, fmt.Errorf("节目倍速必须在 0.25 到 4 之间")
				}
				target[id] = v
			case "bookmarks":
				rows, ok := v.([]any)
				if !ok || len(rows) > 1000 {
					return nil, fmt.Errorf("书签列表无效或过长")
				}
				total += len(rows)
				if total > maxLibraryEntries {
					return nil, fmt.Errorf("书签总数过多")
				}
				clean := []any{}
				seen := map[string]bool{}
				for _, row := range rows {
					b, ok := row.(map[string]any)
					if !ok || !localFields(b, "id", "position", "note", "createdAt", "episode") || !localID(str(b["id"])) || seen[str(b["id"])] || !localNumber(b["position"], 0, 7*24*3600) || !localString(b["note"], 8192, false) || !localTime(b["createdAt"]) {
						return nil, fmt.Errorf("书签格式无效")
					}
					ep, err := localEpisode(b["episode"])
					if err != nil || str(ep["eid"]) != id {
						return nil, fmt.Errorf("书签单集不匹配")
					}
					seen[str(b["id"])] = true
					clean = append(clean, object{"id": b["id"], "position": b["position"], "note": b["note"], "createdAt": b["createdAt"], "episode": ep})
				}
				target[id] = clean
			}
		}
	}
	return out, nil
}

// localBackup is a whitelist, not a serialization of the service store.
type localBackup struct {
	SchemaVersion int     `json:"schemaVersion"`
	Queue         []any   `json:"queue"`
	QueueIndex    float64 `json:"queueIndex"`
	Progress      object  `json:"progress"`
	Prefs         object  `json:"prefs"`
	Library       object  `json:"library"`
}

func validateLocalBackup(b localBackup) (localBackup, error) {
	fail := func() (localBackup, error) {
		return localBackup{}, fmt.Errorf("本地备份格式、版本或数据范围无效")
	}
	if b.SchemaVersion != 1 || b.Queue == nil || len(b.Queue) > 1000 || b.Progress == nil || len(b.Progress) > maxLibraryEntries || b.Prefs == nil {
		return fail()
	}
	maxIndex := max(0, len(b.Queue)-1)
	if !localNumber(b.QueueIndex, -1, float64(maxIndex)) || math.Trunc(b.QueueIndex) != b.QueueIndex {
		return fail()
	}
	queue := []any{}
	for _, v := range b.Queue {
		ep, err := localEpisode(v)
		if err != nil {
			return fail()
		}
		queue = append(queue, ep)
	}
	b.Queue = queue
	for id, v := range b.Progress {
		p, ok := v.(map[string]any)
		if !localID(id) || !ok || !localFields(p, "position", "updatedAt") || !localNumber(p["position"], 0, 7*24*3600) {
			return fail()
		}
		if at, exists := p["updatedAt"]; exists && !localTime(at) {
			return fail()
		}
	}
	if !localFields(b.Prefs, "volume", "rate", "panelWidth", "panelHeight") || len(b.Prefs) != 4 || !localNumber(b.Prefs["volume"], 0, 1) || !localNumber(b.Prefs["rate"], 0.25, 4) || !localNumber(b.Prefs["panelWidth"], 200, 4096) || !localNumber(b.Prefs["panelHeight"], 200, 4096) {
		return fail()
	}
	library, err := validateLibrary(b.Library)
	if err != nil {
		return localBackup{}, err
	}
	b.Library = library
	return b, nil
}

func (s *Service) ExportLocalBackup() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := clone(s.state)
	library := state["library"]
	if library == nil {
		library = emptyLibrary()
	}
	prefs := object{}
	for _, k := range []string{"volume", "rate", "panelWidth", "panelHeight"} {
		prefs[k] = state[k]
	}
	b, err := validateLocalBackup(localBackup{1, list(state["queue"]), num(state["queueIndex"]), obj(state["progress"]), prefs, obj(library)})
	if err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if len(raw) > MaxLocalBackupBytes {
		return nil, fmt.Errorf("备份超过 8MB")
	}
	return raw, err
}

// ImportLocalBackup validates everything before a single atomic store replacement.
// Other state keys, the account, credentials and cache are preserved.
func (s *Service) ImportLocalBackup(raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxLocalBackupBytes || !utf8.Valid(raw) {
		return fmt.Errorf("备份为空、超过 8MB 或编码无效")
	}
	// Require every schema field explicitly, including a zero queue index.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("备份 JSON 无效: %w", err)
	}
	for _, k := range []string{"schemaVersion", "queue", "queueIndex", "progress", "prefs", "library"} {
		value, exists := fields[k]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("备份缺少字段 %s", k)
		}
	}
	var b localBackup
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&b); err != nil {
		return fmt.Errorf("备份 JSON 无效: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("备份包含多余数据")
	}
	valid, err := validateLocalBackup(b)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	next["queue"], next["queueIndex"], next["progress"], next["library"] = valid.Queue, valid.QueueIndex, valid.Progress, valid.Library
	for k, v := range valid.Prefs {
		next[k] = v
	}
	return s.saveState(next)
}
