package player

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

func (s *Service) cloudStore() object {
	c := obj(s.state["cloudSync"])
	if c["tasks"] == nil {
		c["tasks"] = object{}
	}
	s.state["cloudSync"] = c
	return c
}

// Account metadata survives restarts and is never shared between users.
func cloudAccount(c object, account string) object {
	accounts := obj(c["accounts"])
	a := obj(accounts[account])
	accounts[account] = a
	c["accounts"] = accounts
	return a
}
func cloudDue(t, a object) time.Time {
	due, _ := time.Parse(time.RFC3339Nano, str(t["dueAt"]))
	allowed, _ := time.Parse(time.RFC3339Nano, str(a["nextAllowedAt"]))
	if allowed.After(due) {
		return allowed
	}
	return due
}
func cloudBlocked(c object, account string) bool {
	if flag(cloudAccount(c, account)["blocked"]) {
		return true
	}
	for _, v := range obj(c["tasks"]) {
		t := obj(v)
		if str(t["account"]) == account && str(t["error"]) != "" {
			return true
		}
	}
	return false
}
func (s *Service) cloudSyncView() object {
	c := s.cloudStore()
	account := str(obj(s.state["user"])["uid"])
	a := cloudAccount(c, account)
	waiting, uploading, failed, pending := 0, 0, 0, 0
	lastError := str(a["error"])
	var next time.Time
	for _, v := range obj(c["tasks"]) {
		t := obj(v)
		if str(t["account"]) != account {
			continue
		}
		pending++
		if str(t["error"]) != "" {
			failed++
			if lastError == "" {
				lastError = str(t["error"])
			}
		} else if flag(t["inFlight"]) {
			uploading++
		} else if !flag(t["suspended"]) {
			waiting++
		}
		if !flag(t["suspended"]) {
			due := cloudDue(t, a)
			if next.IsZero() || due.Before(next) {
				next = due
			}
		}
	}
	blocked := cloudBlocked(c, account)
	status := "idle"
	if blocked {
		status = "error"
	} else if uploading > 0 {
		status = "syncing"
	} else if waiting > 0 {
		status = "pending"
	}
	nextAt := ""
	if !next.IsZero() && !blocked {
		nextAt = next.UTC().Format(time.RFC3339Nano)
	}
	return object{"enabled": flag(c["enabled"]), "status": status, "lastSuccessAt": str(a["lastSuccessAt"]), "lastError": lastError, "pending": pending, "waiting": waiting, "uploading": uploading, "failed": failed, "nextUploadAt": nextAt, "blocked": blocked}
}
func (s *Service) wakeCloud() {
	if s.cloudWake == nil {
		s.cloudWake = make(chan struct{}, 1)
	}
	select {
	case s.cloudWake <- struct{}{}:
	default:
	}
}
func (s *Service) cancelCloudAccount() {
	if s.cloudCancel != nil {
		s.cloudCancel()
	}
	for _, cancel := range s.cloudCancels {
		cancel()
	}
	s.cloudPlayback = ""
	s.cloudPlaybackEvent = ""
	s.cloudActive = ""
	// Retain account-bound pending work; never transfer it to another account.
	c := s.cloudStore()
	delete(c, "active")
	c["lastError"] = ""
	s.wakeCloud()
}
func (s *Service) cloudSyncSettings(body object) (any, error) {
	enabled, ok := body["enabled"].(bool)
	if !ok {
		return nil, errors.New("缺少 enabled")
	}
	next := clone(s.state)
	c := obj(next["cloudSync"])
	c["enabled"] = enabled
	next["cloudSync"] = c
	if err := s.saveState(next); err != nil {
		return nil, err
	}
	if !enabled && s.cloudCancel != nil {
		s.cloudCancel()
	}
	s.wakeCloud()
	return object{"ok": true, "cloudSync": s.cloudSyncView()}, nil
}
func (s *Service) playbackEvent(body object) (any, error) {
	event := str(body["event"])
	if event != "playing" && event != "paused" && event != "ended" {
		return nil, errors.New("播放事件无效")
	}
	ep := obj(body["episode"])
	eid := str(ep["eid"])
	position := num(body["position"])
	if !localID(eid) || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 7*24*3600 {
		return nil, errors.New("播放信息无效")
	}
	// Ignore late pause/end notifications from an episode that is no longer active.
	if event != "playing" && (s.cloudPlayback != eid || s.cloudPlaybackEvent == "ended") {
		return object{"ok": true, "cloudSync": s.cloudSyncView()}, nil
	}
	if event == "paused" && s.cloudPlaybackEvent == "paused" {
		return object{"ok": true, "cloudSync": s.cloudSyncView()}, nil
	}
	s.cloudPlayback = eid
	s.cloudPlaybackEvent = event
	next := clone(s.state)
	c := obj(next["cloudSync"])
	tasks := obj(c["tasks"])
	accountKey := str(obj(s.state["user"])["uid"])
	key := accountKey + ":" + eid
	playedAt := str(body["playedAt"])
	if _, err := time.Parse(time.RFC3339Nano, playedAt); err != nil {
		playedAt = nowISO()
	}
	c["active"] = object{"episode": clone(ep), "position": position, "playedAt": playedAt, "event": event, "account": accountKey}
	if event == "playing" {
		if t := obj(tasks[key]); str(t["event"]) == "paused" {
			if flag(t["inFlight"]) {
				t["suspended"] = true
				if s.cloudActive == eid && s.cloudCancel != nil {
					s.cloudCancel()
				}
			} else if str(t["error"]) == "" && !flag(t["progressDone"]) && !flag(t["historyDone"]) {
				delete(tasks, key)
			} else {
				t["suspended"] = true
			}
		}
	} else if flag(c["enabled"]) {
		account := str(obj(s.state["user"])["uid"])
		token, _, err := s.tokens()
		if err != nil {
			return nil, err
		}
		if account != "" && token != "" {
			due := time.Now()
			if event == "paused" {
				due = due.Add(5 * time.Minute)
			}
			last := obj(obj(c["completed"])[key])
			if num(last["position"]) == float64(int(position)) && flag(last["historyDone"]) {
				c["tasks"] = tasks
				next["cloudSync"] = c
				if err := s.saveState(next); err != nil {
					return nil, err
				}
				return object{"ok": true, "cloudSync": s.cloudSyncView()}, nil
			}
			previous := obj(tasks[key])
			if flag(previous["inFlight"]) {
				// Cancel before replacing the revision; successful old parts are reconciled below.
				if s.cloudCancel != nil {
					s.cloudCancel()
				}
			}
			tasks[key] = object{"key": key, "account": account, "eid": eid, "pid": str(ep["pid"]), "position": int(position), "playedAt": playedAt, "dueAt": due.UTC().Format(time.RFC3339Nano), "event": event, "revision": fmt.Sprint(time.Now().UnixNano()), "progressDone": false, "historyDone": false}
			task := obj(tasks[key])
			task["historyDone"] = flag(previous["historyDone"])
			if str(previous["error"]) != "" {
				task["error"] = previous["error"]
			}
			if len(previous) > 0 && num(previous["position"]) == float64(int(position)) {
				task["progressDone"] = flag(previous["progressDone"])
				task["playedAt"] = previous["playedAt"]
				if str(previous["error"]) != "" {
					task["error"] = previous["error"]
				}
			}
		}
	}
	c["tasks"] = tasks
	next["cloudSync"] = c
	if err := s.saveState(next); err != nil {
		return nil, err
	}
	s.wakeCloud()
	return object{"ok": true, "cloudSync": s.cloudSyncView()}, nil
}
func (s *Service) retryCloudSync() (any, error) {
	next := clone(s.state)
	c := obj(next["cloudSync"])
	for _, v := range obj(c["tasks"]) {
		t := obj(v)
		if str(t["account"]) == str(obj(s.state["user"])["uid"]) && str(t["error"]) != "" {
			delete(t, "error")
			delete(t, "suspended")
			t["dueAt"] = nowISO()
		}
	}
	a := cloudAccount(c, str(obj(s.state["user"])["uid"]))
	a["blocked"] = false
	delete(a, "error")
	c["lastError"] = ""
	next["cloudSync"] = c
	if err := s.saveState(next); err != nil {
		return nil, err
	}
	s.wakeCloud()
	return object{"ok": true, "cloudSync": s.cloudSyncView()}, nil
}

// StartCloudSync waits for the later of a task deadline and account cooldown.
// Failures require explicit retry; they are never polled.
func (s *Service) StartCloudSync(ctx context.Context) {
	s.mu.Lock()
	if s.cloudRunning {
		s.mu.Unlock()
		return
	}
	s.cloudRunning = true
	c0 := s.cloudStore()
	for _, v := range obj(c0["tasks"]) {
		t := obj(v)
		if flag(t["inFlight"]) {
			delete(t, "inFlight")
			t["error"] = "同步中断，请手动重试"
			a := cloudAccount(c0, str(t["account"]))
			a["blocked"] = true
			a["error"] = t["error"]
		}
	}
	trimCloudCompleted(c0)
	_ = s.saveState(clone(s.state))
	a := obj(c0["active"])
	if str(a["event"]) == "playing" && str(a["account"]) == str(obj(s.state["user"])["uid"]) {
		s.cloudPlayback = str(obj(a["episode"])["eid"])
		s.cloudPlaybackEvent = "playing"
		_, _ = s.playbackEvent(object{"event": "paused", "episode": a["episode"], "position": a["position"], "playedAt": a["playedAt"]})
		key := str(a["account"]) + ":" + str(obj(a["episode"])["eid"])
		if at, err := time.Parse(time.RFC3339Nano, str(a["playedAt"])); err == nil {
			obj(obj(s.cloudStore()["tasks"])[key])["dueAt"] = at.Add(5 * time.Minute).UTC().Format(time.RFC3339Nano)
			_ = s.saveState(clone(s.state))
		}
	}
	s.wakeCloud()
	wake := s.cloudWake
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.cloudRunning = false; s.mu.Unlock() }()
	for {
		if ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		c := s.cloudStore()
		var chosen object
		var deadline time.Time
		if flag(c["enabled"]) && !cloudBlocked(c, str(obj(s.state["user"])["uid"])) {
			for _, v := range obj(c["tasks"]) {
				t := obj(v)
				if str(t["account"]) != str(obj(s.state["user"])["uid"]) || str(t["error"]) != "" || flag(t["suspended"]) {
					continue
				}
				due := cloudDue(t, cloudAccount(c, str(t["account"])))
				if due.IsZero() {
					continue
				}
				if deadline.IsZero() || due.Before(deadline) {
					deadline = due
					chosen = clone(t)
				}
			}
		}
		if chosen != nil && !deadline.After(time.Now()) {
			token, _, err := s.tokens()
			if err != nil || token == "" {
				s.mu.Unlock()
				select {
				case <-ctx.Done():
					return
				case <-wake:
					continue
				}
			}
			runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			epoch := s.accountEpoch
			s.cloudCancel = cancel
			s.cloudActive = str(chosen["eid"])
			s.mu.Unlock()
			s.runCloudTask(runCtx, token, chosen, epoch)
			cancel()
			s.mu.Lock()
			s.cloudCancel = nil
			s.cloudActive = ""
			s.mu.Unlock()
			continue
		}
		s.mu.Unlock()
		var timer *time.Timer
		var tick <-chan time.Time
		if !deadline.IsZero() {
			timer = time.NewTimer(time.Until(deadline))
			tick = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-wake:
		case <-tick:
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

// Keep only the newest successful records globally; task failures are never pruned.
func trimCloudCompleted(c object) {
	completed := obj(c["completed"])
	keys := make([]string, 0, len(completed))
	for key := range completed {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		x, y := str(obj(completed[keys[i]])["completedAt"]), str(obj(completed[keys[j]])["completedAt"])
		tx, _ := time.Parse(time.RFC3339Nano, x)
		ty, _ := time.Parse(time.RFC3339Nano, y)
		if tx.Equal(ty) {
			return keys[i] < keys[j]
		}
		return tx.Before(ty)
	})
	for len(keys) > 300 {
		delete(completed, keys[0])
		keys = keys[1:]
	}
}
func (s *Service) runCloudTask(ctx context.Context, token string, t object, epoch uint64) {
	key, account := str(t["key"]), str(t["account"])
	s.mu.Lock()
	c := s.cloudStore()
	live := obj(obj(c["tasks"])[key])
	a := cloudAccount(c, account)
	allowed, _ := time.Parse(time.RFC3339Nano, str(a["nextAllowedAt"]))
	if ctx.Err() != nil || s.accountEpoch != epoch || str(live["revision"]) != str(t["revision"]) || !flag(c["enabled"]) || cloudBlocked(c, account) || flag(live["inFlight"]) || flag(live["suspended"]) || allowed.After(time.Now()) {
		s.mu.Unlock()
		return
	}
	live["inFlight"] = true
	a["nextAllowedAt"] = time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano)
	err := s.saveState(clone(s.state))
	s.mu.Unlock()
	if err != nil {
  s.mu.Lock()
  a := cloudAccount(s.cloudStore(), account)
  a["blocked"] = true; a["error"] = err.Error()
  live := obj(obj(s.cloudStore()["tasks"])[key])
  delete(live,"inFlight"); live["error"] = err.Error()
  s.mu.Unlock()
  return
 }
 defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		live := obj(obj(s.cloudStore()["tasks"])[key])
		if str(live["revision"]) == str(t["revision"]) {
			delete(live, "inFlight")
			_ = s.saveState(clone(s.state))
		}
	}()
	for _, part := range []string{"progressDone", "historyDone"} {
		s.mu.Lock()
		c = s.cloudStore()
		live = obj(obj(c["tasks"])[key])
		valid := s.accountEpoch == epoch && str(live["revision"]) == str(t["revision"]) && flag(c["enabled"]) && !flag(live["suspended"]) && ctx.Err() == nil
		done := flag(live[part])
		s.mu.Unlock()
		if !valid {
			return
		}
		if done {
			continue
		}
		send := func(token string) error {
			if part == "progressDone" {
				return s.client.UploadPlaybackProgress(ctx, token, str(t["eid"]), str(t["pid"]), int(num(t["position"])), str(t["playedAt"]))
			}
			return s.client.CreatePlayedHistory(ctx, token, str(t["eid"]))
		}
		err = s.cloudAuthorized(ctx, epoch, token, send)
		s.mu.Lock()
		c = s.cloudStore()
		live = obj(obj(c["tasks"])[key])
		// A replacement revision can retain only parts that apply to its payload.
		same := str(live["revision"]) == str(t["revision"])
		if err == nil && len(live) > 0 && (same || part == "historyDone" || num(live["position"]) == num(t["position"])) {
			live[part] = true
		}
		if err != nil && !(errors.Is(err, context.Canceled) && ctx.Err() != nil) {
			a = cloudAccount(c, account)
			a["blocked"] = true
			a["error"] = err.Error()
			if len(live) > 0 {
				live["error"] = err.Error()
			}
		}
		if same && flag(live["progressDone"]) && flag(live["historyDone"]) {
			delete(obj(c["tasks"]), key)
			completed := obj(c["completed"])
			live["completedAt"] = nowISO()
			delete(live, "inFlight")
			completed[key] = clone(live)
			c["completed"] = completed
			cloudAccount(c, account)["lastSuccessAt"] = nowISO()
			trimCloudCompleted(c)
		}
		saveErr := s.saveState(clone(s.state))
  if saveErr != nil {
   a := cloudAccount(s.cloudStore(),account)
   a["blocked"] = true; a["error"] = saveErr.Error()
   _ = s.saveState(clone(s.state))
  }
  s.mu.Unlock()
		if err != nil || saveErr != nil || !same {
			return
		}
	}
}

func (s *Service) pullCloudPlaylist(ctx context.Context, body object) (any, error) {
	s.mu.Lock()
	token, _, err := s.tokens()
	if err != nil || token == "" {
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return nil, errLogin
	}
	epoch := s.accountEpoch
	requestCtx, cancel := context.WithCancel(ctx)
	s.cloudSeq++
	seq := s.cloudSeq
	if s.cloudCancels == nil {
		s.cloudCancels = map[uint64]context.CancelFunc{}
	}
	s.cloudCancels[seq] = cancel
	metadata := map[string]object{}
	for _, key := range []string{"history", "queue"} {
		for _, v := range list(s.state[key]) {
			ep := obj(v)
			id := str(ep["eid"])
			existing := metadata[id]
			if existing == nil {
				existing = object{}
			}
			for k, v := range ep {
				if v != nil && v != "" {
					existing[k] = v
				}
			}
			metadata[id] = existing
		}
	}
	for key, v := range s.cache {
		if len(key) > 8 && key[:8] == "episode:" {
			ep := obj(obj(v)["value"])
			if metadata[str(ep["eid"])] == nil {
				id := str(ep["eid"])
				merged := metadata[id]
				if merged == nil {
					merged = object{}
				}
				for k, value := range ep {
					if value != nil && value != "" {
						merged[k] = value
					}
				}
				metadata[id] = merged
			}
		}
	}
	s.mu.Unlock()
	defer func() { cancel(); s.mu.Lock(); delete(s.cloudCancels, seq); s.mu.Unlock() }()
	var ids []string
	err = s.cloudAuthorized(requestCtx, epoch, token, func(token string) error { var e error; ids, e = s.client.PullPlaylist(requestCtx, token); return e })
	if err != nil {
		return nil, err
	}
	imported := map[string]bool{}
	queue := []any{}
	for _, id := range ids {
		if id == "" || imported[id] {
			continue
		}
		ep := metadata[id]
		if ep == nil {
			v, e := s.client.GetEpisode(requestCtx, id)
			if e != nil {
				return nil, e
			}
			ep = obj(v)
		}
		if str(ep["eid"]) != id {
			return nil, errors.New("云端单集信息无效")
		}
		queue = append(queue, ep)
		imported[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if epoch != s.accountEpoch || requestCtx.Err() != nil {
		return nil, errors.New("账号已切换，已取消同步")
	}
	next := clone(s.state)
	current := str(body["currentEid"])
	old := list(next["queue"])
	if current == "" {
		i := int(num(next["queueIndex"]))
		if i >= 0 && i < len(old) {
			current = str(obj(old[i])["eid"])
		}
	}
	seen := map[string]bool{}
	for id := range imported {
		seen[id] = true
	}
	for _, v := range old {
		id := str(obj(v)["eid"])
		if id != "" && !seen[id] {
			queue = append(queue, v)
			seen[id] = true
		}
	}
	index := 0
	for i, v := range queue {
		if str(obj(v)["eid"]) == current {
			index = i
			break
		}
	}
	// Preview only: the UI commits through saveQueue after checking that no
	// local queue edit or playback transition occurred during this request.

	return object{"ok": true, "queue": queue, "index": index, "cloudSync": s.cloudSyncView()}, nil
}

func (s *Service) cloudAuthorized(ctx context.Context, epoch uint64, token string, send func(string) error) error {
	preflight := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if epoch != s.accountEpoch {
			return errCommentAccountChanged
		}
		return nil
	}
	if err := preflight(); err != nil {
		return err
	}
	err := send(token)
	if !unauthorized(err) {
		return err
	}
	s.mu.Lock()
	_, refresh, e := s.tokens()
	changed := epoch != s.accountEpoch
	s.mu.Unlock()
	if changed {
		return errCommentAccountChanged
	}
	if e != nil {
		return e
	}
	if refresh == "" {
		return err
	}
	fresh, e := s.refreshCommentToken(ctx, epoch, token, refresh)
	if e != nil {
		return e
	}
	if err := preflight(); err != nil {
		return err
	}
	return send(fresh)
}
