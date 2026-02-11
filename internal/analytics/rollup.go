package analytics

import (
	"log"
	"time"

	"github.com/JustinTDCT/CineVault/internal/repository"
)

// RunDailyRollup computes yesterday's stats and stores them.
func RunDailyRollup(repo *repository.AnalyticsRepository) {
	yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	log.Printf("Analytics rollup: computing stats for %s", yesterday.Format("2006-01-02"))

	stat, err := repo.ComputeDailyRollup(yesterday)
	if err != nil {
		log.Printf("Analytics rollup: compute failed: %v", err)
		return
	}

	if err := repo.UpsertDailyStat(stat); err != nil {
		log.Printf("Analytics rollup: upsert failed: %v", err)
		return
	}

	log.Printf("Analytics rollup: saved - plays=%d users=%d transcodes=%d bandwidth=%d bytes",
		stat.TotalPlays, stat.UniqueUsers, stat.Transcodes, stat.TotalBytesServed)
}

// StartRollupScheduler runs the daily rollup at midnight.
func StartRollupScheduler(repo *repository.AnalyticsRepository, stopCh chan struct{}) {
	log.Println("Analytics rollup scheduler started")

	// Run immediately for yesterday on startup
	RunDailyRollup(repo)

	for {
		// Calculate time until next midnight
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, now.Location())
		sleepDuration := nextMidnight.Sub(now)

		select {
		case <-time.After(sleepDuration):
			RunDailyRollup(repo)
		case <-stopCh:
			log.Println("Analytics rollup scheduler stopped")
			return
		}
	}
}
