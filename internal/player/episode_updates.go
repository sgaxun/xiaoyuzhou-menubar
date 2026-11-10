package player

import (
	"context"
	"sort"
	"time"
)

// All feed/state access is under mu; updateMu serializes manual and automatic checks.
func (s *Service) resetEpisodeUpdates() error {
	s.updateEpoch++
	if s.updateAutoCancel != nil {
		s.updateAutoCancel()
	}
	next := clone(s.state)
	delete(next, "episodeUpdates")
	return s.saveState(next)
}
func (s *Service) episodeUpdateFeed() object {
	f := obj(s.state["episodeUpdates"])
	subscriptions := map[string]object{}
	for _, v := range list(obj(s.cache["subs"])["value"]) {
		sub := obj(v)
		subscriptions[str(sub["pid"])] = sub
	}
	episodes := []any{}
	for _, v := range list(f["episodes"]) {
		ep := obj(v)
		sourcePID := str(ep["subscriptionPid"])
		if sourcePID == "" {
			sourcePID = str(ep["pid"])
		} // Older feeds stored the subscribed PID here.
		if sub, exists := subscriptions[sourcePID]; exists {
			ep = subscriptionEpisode(ep, sub)
		}
		episodes = append(episodes, ep)
	}
	return object{"ok": true, "episodes": episodes, "checkedAt": str(f["checkedAt"]), "failures": list(f["failures"]), "remaining": s.updateRemaining(f), "automatic": s.episodeUpdatesAutomatic(), "checking": s.updateChecking}
}

// Restore source metadata for old queue entries using only local evidence.
// An explicit source wins; otherwise require a unique subscribed seen record.
func (s *Service) queueSubscriptionSource(ep object) object {
	subs := map[string]object{}
	for _, v := range list(obj(s.cache["subs"])["value"]) {
		sub := obj(v)
		subs[str(sub["pid"])] = sub
	}
	if pid := str(ep["subscriptionPid"]); pid != "" {
		if sub, ok := subs[pid]; ok {
			return subscriptionEpisode(ep, sub)
		}
		return ep
	}
	var source object
	for pid, v := range obj(obj(s.state["episodeUpdates"])["programs"]) {
		if sub, ok := subs[pid]; ok && flag(obj(obj(v)["seen"])[str(ep["eid"])]) {
			if source != nil {
				return ep
			} // Ambiguous: do not guess.
			source = sub
		}
	}
	if source != nil {
		return subscriptionEpisode(ep, source)
	}
	if sub, ok := subs[str(ep["pid"])]; ok {
		return subscriptionEpisode(ep, sub)
	}
	return ep
}

// Old history dropped source fields. Recover only from the same episode's
// explicit queue source, or the existing unambiguous local subscription evidence.
func (s *Service) historySubscriptionSource(ep object) object {
	if str(ep["subscriptionPid"]) != "" {
		return s.queueSubscriptionSource(ep)
	}
	var source object
	for _, v := range list(s.state["queue"]) {
		queued := obj(v)
		if str(queued["eid"]) != str(ep["eid"]) || str(queued["subscriptionPid"]) == "" {
			continue
		}
		if source != nil && str(source["subscriptionPid"]) != str(queued["subscriptionPid"]) {
			return ep
		}
		source = queued
	}
	if source != nil {
		out := clone(ep)
		for _, k := range []string{"subscriptionPid", "subscriptionTitle", "subscriptionImage", "isCoCreated"} {
			if v, ok := source[k]; ok {
				out[k] = v
			}
		}
		return s.queueSubscriptionSource(out)
	}
	return s.queueSubscriptionSource(ep)
}

// Keep canonical episode metadata for playback; source fields describe why the
// episode appears in this user's subscription feed. Also decorates older caches.
func subscriptionEpisode(ep, sub object) object {
	out := clone(ep)
	sourcePID := str(sub["pid"])
	out["subscriptionPid"] = sourcePID
	out["subscriptionTitle"] = sub["title"]
	out["subscriptionImage"] = sub["image"]
	canonicalPID := str(ep["pid"])
	// Legacy entries already replaced canonical PID with source PID. Their
	// different podcast title is retained, allowing the source to be explained.
	if str(ep["subscriptionPid"]) == "" {
		out["isCoCreated"] = (canonicalPID != "" && canonicalPID != sourcePID) || (str(ep["podcastTitle"]) != "" && str(sub["title"]) != "" && str(ep["podcastTitle"]) != str(sub["title"]))
	}
	return out
}

func (s *Service) updateRemaining(f object) int {
	n := 0
	seen := map[string]bool{}
	for _, v := range list(obj(s.cache["subs"])["value"]) {
		pid := str(obj(v)["pid"])
		if pid != "" && !seen[pid] && time.Since(updateDate(obj(obj(f["programs"])[pid])["attemptedAt"])) >= 15*time.Minute {
			n++
		}
		seen[pid] = true
	}
	return n
}
func updateDate(v any) time.Time { t, _ := time.Parse(time.RFC3339Nano, str(v)); return t }
func (s *Service) episodeUpdatesAutomatic() bool {
	enabled, exists := s.state["episodeUpdatesAutomatic"].(bool)
	return !exists || enabled
}
func (s *Service) checkEpisodeUpdates(ctx context.Context) (any, error) {
	return s.checkEpisodeUpdatesWith(ctx, 20, 45*time.Second, nil)
}
func (s *Service) checkEpisodeUpdatesWith(ctx context.Context, limit int, budget time.Duration, automaticAt *time.Time) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.updateMu.TryLock() {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.episodeUpdateFeed(), nil
	}
	defer s.updateMu.Unlock()
	ctx, cancelBudget := context.WithTimeout(ctx, budget)
	defer cancelBudget()
	s.mu.Lock()
	if automaticAt != nil {
		feed := obj(s.state["episodeUpdates"])
		if !s.episodeUpdatesAutomatic() || len(list(obj(s.cache["subs"])["value"])) == 0 || automaticAt.Sub(updateDate(feed["lastAutomaticCheck"])) < time.Hour {
			out := s.episodeUpdateFeed()
			s.mu.Unlock()
			return out, nil
		}
		next := clone(s.state)
		nextFeed := obj(next["episodeUpdates"])
		nextFeed["lastAutomaticCheck"] = automaticAt.UTC().Format(time.RFC3339Nano)
		next["episodeUpdates"] = nextFeed
		if err := s.saveState(next); err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.updateAutoCancel = cancelBudget
	}
	s.updateChecking = true
	defer func() { s.mu.Lock(); s.updateChecking = false; s.updateAutoCancel = nil; s.mu.Unlock() }()
	epoch := s.updateEpoch
	// Snapshot only local subscription and podcast caches. Never acquire credentials.
	snapshot := clone(s.cache)
	subs := list(obj(snapshot["subs"])["value"])
	f := obj(s.state["episodeUpdates"])
	cursor := int(num(f["cursor"]))
	if cursor < 0 {
		cursor = 0
	}
	if limit == 0 {
		limit = len(subs)
	}
	s.mu.Unlock()

	checked := 0
	visited := map[string]bool{}
	for offset := 0; offset < len(subs) && checked < limit; offset++ {
		pos := (cursor + offset) % len(subs)
		sub := obj(subs[pos])
		pid := str(sub["pid"])
		if pid == "" || visited[pid] {
			continue
		}
		visited[pid] = true
		s.mu.Lock()
		if epoch != s.updateEpoch {
			out := s.episodeUpdateFeed()
			s.mu.Unlock()
			return out, nil
		}
		next := clone(s.state)
		feed := obj(next["episodeUpdates"])
		programs := obj(feed["programs"])
		program := obj(programs[pid])
		if time.Since(updateDate(program["attemptedAt"])) < 15*time.Minute {
			s.mu.Unlock()
			continue
		}
		s.mu.Unlock()
		if delay := time.Until(s.updateLast.Add(time.Second)); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				goto done
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			break
		}
		s.mu.Lock()
		if epoch != s.updateEpoch {
			out := s.episodeUpdateFeed()
			s.mu.Unlock()
			return out, nil
		}
		// Re-read after waiting so dismissals and queue writes cannot be overwritten.
		next = clone(s.state)
		feed = obj(next["episodeUpdates"])
		programs = obj(feed["programs"])
		program = obj(programs[pid])
		if !flag(program["initialized"]) {
			ids := object{}
			watermark := updateDate(sub["latestEpisodePubDate"])
			for _, v := range list(obj(obj(snapshot["podcast:"+pid])["value"])["episodes"]) {
				ep := obj(v)
				if id := str(ep["eid"]); id != "" {
					ids[id] = true
				}
				if d := updateDate(ep["pubDate"]); d.After(watermark) {
					watermark = d
				}
			}
			if floor := time.Now().Add(-7 * 24 * time.Hour); watermark.Before(floor) {
				watermark = floor
			}
			program["seen"] = ids
			program["watermark"] = watermark.UTC().Format(time.RFC3339Nano)
			program["initialized"] = true
		}
		program["attemptedAt"] = nowISO()
		programs[pid] = program
		feed["programs"] = programs
		feed["cursor"] = (pos + 1) % len(subs)
		next["episodeUpdates"] = feed
		err := s.saveState(next)
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
		checked++
		s.updateLast = time.Now()
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		result, fetchErr := s.client.GetPodcast(requestCtx, pid)
		cancel()
		s.mu.Lock()
		if ctx.Err() != nil {
			s.mu.Unlock()
			break
		}
		if epoch != s.updateEpoch {
			out := s.episodeUpdateFeed()
			s.mu.Unlock()
			return out, nil
		}
		next = clone(s.state)
		feed = obj(next["episodeUpdates"])
		programs = obj(feed["programs"])
		program = obj(programs[pid])
		failures := []any{}
		for _, failure := range list(feed["failures"]) {
			if str(obj(failure)["pid"]) != pid {
				failures = append(failures, failure)
			}
		}
		if fetchErr != nil {
			failures = append(failures, object{"pid": pid, "error": fetchErr.Error()})
		} else {
			seen := obj(program["seen"])
			watermark := updateDate(program["watermark"])
			newest := watermark
			pending := list(feed["episodes"])
			pendingIDs := map[string]bool{}
			queuedIDs := map[string]bool{}
			for _, v := range list(s.state["queue"]) {
				queuedIDs[str(obj(v)["eid"])] = true
			}
			for _, v := range pending {
				pendingIDs[str(obj(v)["eid"])] = true
			}
			for _, v := range list(result["episodes"]) {
				ep := obj(v)
				id := str(ep["eid"])
				if id == "" {
					continue
				}
				date := updateDate(ep["pubDate"])
				if !flag(seen[id]) && !date.IsZero() && !date.Before(watermark) && !pendingIDs[id] && !queuedIDs[id] {
					ep = subscriptionEpisode(ep, sub)
					if str(ep["pid"]) == "" {
						ep["pid"] = pid
					}
					if str(ep["podcastTitle"]) == "" {
						ep["podcastTitle"] = sub["title"]
					}
					pending = append(pending, ep)
					pendingIDs[id] = true
				}
				seen[id] = true
				if date.After(newest) {
					newest = date
				}
			}
			program["seen"] = seen
			program["watermark"] = newest.UTC().Format(time.RFC3339Nano)
			programs[pid] = program
			feed["programs"] = programs
			sort.SliceStable(pending, func(i, j int) bool {
				return updateDate(obj(pending[i])["pubDate"]).After(updateDate(obj(pending[j])["pubDate"]))
			})
			feed["episodes"] = pending
		}
		feed["failures"] = failures
		feed["checkedAt"] = nowISO()
		next["episodeUpdates"] = feed
		err = s.saveState(next)
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
done:
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateChecking = false
	return s.episodeUpdateFeed(), nil
}
