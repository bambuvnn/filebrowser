package http

import (
	"context"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

const trashRetentionDuration = 7 * 24 * time.Hour
const trashSchedulerInterval = 24 * time.Hour

// startTrashScheduler starts a background goroutine that auto-purges trash items
// older than 7 days. It runs every 24 hours.
func startTrashScheduler(ctx context.Context) {
	go func() {
		logger.Info("Trash scheduler started (retention: 7 days, interval: 24h)")
		// Run once immediately on startup, then tick
		runTrashPurge()

		ticker := time.NewTicker(trashSchedulerInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runTrashPurge()
			case <-ctx.Done():
				logger.Info("Trash scheduler stopped.")
				return
			}
		}
	}()
}

// runTrashPurge queries and permanently deletes all trash items older than the retention period.
func runTrashPurge() {
	if store == nil || store.Trash == nil {
		return
	}

	expired, err := store.Trash.GetExpired(trashRetentionDuration)
	if err != nil {
		logger.Errorf("Trash scheduler: failed to query expired items: %v", err)
		return
	}

	if len(expired) == 0 {
		logger.Debugf("Trash scheduler: no expired items found")
		return
	}

	logger.Infof("Trash scheduler: purging %d expired item(s)", len(expired))
	for _, item := range expired {
		permanentlyDeleteTrashItem(item)
	}
}
