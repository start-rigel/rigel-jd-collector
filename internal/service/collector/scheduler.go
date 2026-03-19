package collector

import (
	"context"
	"fmt"
	"log"
	"time"
)

const schedulePollInterval = 15 * time.Second

func (s *Service) RunScheduleLoop(ctx context.Context, serviceName, mode string) error {
	var activeSignature string
	var nextRun time.Time

	for {
		cfg, exists, err := s.repo.GetCollectorScheduleConfig(ctx, serviceName)
		if err != nil {
			log.Printf("load collector schedule config failed: %v", err)
			if err := sleepWithContext(ctx, schedulePollInterval); err != nil {
				return err
			}
			continue
		}

		if !exists || !cfg.Enabled {
			activeSignature = ""
			nextRun = time.Time{}
			if err := sleepWithContext(ctx, schedulePollInterval); err != nil {
				return err
			}
			continue
		}

		signature := fmt.Sprintf("%s|%t|%s|%d|%d", cfg.ServiceName, cfg.Enabled, cfg.ScheduleTime, cfg.RequestIntervalSeconds, cfg.QueryLimit)
		now := s.clock().In(time.Local)
		if signature != activeSignature || nextRun.IsZero() {
			scheduled, err := nextScheduledTime(now, cfg.ScheduleTime)
			if err != nil {
				return err
			}
			activeSignature = signature
			nextRun = scheduled
		}

		waitDuration := time.Until(nextRun)
		if waitDuration > schedulePollInterval {
			waitDuration = schedulePollInterval
		}
		if waitDuration > 0 {
			if err := sleepWithContext(ctx, waitDuration); err != nil {
				return err
			}
			continue
		}

		if _, err := s.RunScheduledCollection(ctx, ScheduledCollectionRequest{
			Persist:         true,
			QueryLimit:      cfg.QueryLimit,
			RequestInterval: time.Duration(cfg.RequestIntervalSeconds) * time.Second,
		}, mode); err != nil {
			log.Printf("scheduled daily collection failed: %v", err)
		}
		nextRun = nextRun.Add(24 * time.Hour)
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
