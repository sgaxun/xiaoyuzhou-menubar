package player

import (
	"context"
	"time"
)

// RunEpisodeUpdateScheduler runs only while the application is alive. The
// persisted attempt time prevents relaunch/wake from creating catch-up bursts.
func (s *Service) RunEpisodeUpdateScheduler(ctx context.Context) {
	initial := time.NewTimer(30 * time.Second)
	defer initial.Stop()
	select {
	case <-ctx.Done():
		return
	case <-initial.C:
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		// Errors leave the feed intact; an attempted sweep is not retried until
		// the next hourly slot. No subscription/authentication endpoint is used.
		_ = s.runAutomaticEpisodeUpdates(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) runAutomaticEpisodeUpdates(ctx context.Context, now time.Time) error {
	_, err := s.checkEpisodeUpdatesWith(ctx, 0, 10*time.Minute, &now)
	return err
}
