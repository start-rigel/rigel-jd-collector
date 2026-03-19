package collector

import (
	"context"
	"fmt"
	"log"
	"time"
)

type ScheduleConfig struct {
	Enabled         bool
	RunAt           string
	RunOnStartup    bool
	QueryLimit      int
	Persist         bool
	RequestInterval time.Duration
}

func (s *Service) RunScheduleLoop(ctx context.Context, cfg ScheduleConfig, mode string) error {
	if cfg.RunOnStartup {
		if _, err := s.RunScheduledCollection(ctx, ScheduledCollectionRequest{
			Persist:         cfg.Persist,
			QueryLimit:      cfg.QueryLimit,
			RequestInterval: cfg.RequestInterval,
		}, mode); err != nil {
			log.Printf("scheduled startup collection failed: %v", err)
		}
	}
	if !cfg.Enabled {
		<-ctx.Done()
		return ctx.Err()
	}

	for {
		nextRun, err := nextScheduledTime(s.clock().In(time.Local), cfg.RunAt)
		if err != nil {
			return err
		}
		waitDuration := time.Until(nextRun)
		if waitDuration < 0 {
			waitDuration = 0
		}
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		if _, err := s.RunScheduledCollection(ctx, ScheduledCollectionRequest{
			Persist:         cfg.Persist,
			QueryLimit:      cfg.QueryLimit,
			RequestInterval: cfg.RequestInterval,
		}, mode); err != nil {
			log.Printf("scheduled daily collection failed: %v", err)
		}
	}
}

func nextScheduledTime(now time.Time, hhmm string) (time.Time, error) {
	parsed, err := time.ParseInLocation("15:04", hhmm, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("parse scheduled time %q: %w", hhmm, err)
	}
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	if !nextRun.After(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}
	return nextRun, nil
}
