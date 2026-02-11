package notifications

import (
	"fmt"
	"log"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
)

// AlertEvaluator checks alert rules against current metrics and sends notifications.
type AlertEvaluator struct {
	analyticsRepo *repository.AnalyticsRepository
	notifRepo     *repository.NotificationRepository
	sender        *WebhookSender
	interval      time.Duration
	stopCh        chan struct{}
}

// NewAlertEvaluator creates a new evaluator.
func NewAlertEvaluator(
	analyticsRepo *repository.AnalyticsRepository,
	notifRepo *repository.NotificationRepository,
	sender *WebhookSender,
) *AlertEvaluator {
	return &AlertEvaluator{
		analyticsRepo: analyticsRepo,
		notifRepo:     notifRepo,
		sender:        sender,
		interval:      5 * time.Minute,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the evaluation loop. Run as a goroutine.
func (e *AlertEvaluator) Start() {
	log.Println("Alert evaluator started (5m interval)")
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.evaluate()
		case <-e.stopCh:
			log.Println("Alert evaluator stopped")
			return
		}
	}
}

// Stop halts the evaluator.
func (e *AlertEvaluator) Stop() {
	close(e.stopCh)
}

func (e *AlertEvaluator) evaluate() {
	rules, err := e.notifRepo.ListEnabledAlertRules()
	if err != nil {
		log.Printf("Alert evaluator: failed to list rules: %v", err)
		return
	}

	if len(rules) == 0 {
		return
	}

	// Get latest metrics
	metrics, err := e.analyticsRepo.GetLatestMetrics()
	if err != nil {
		log.Printf("Alert evaluator: failed to get metrics: %v", err)
		return
	}

	for _, rule := range rules {
		// Check cooldown
		if rule.LastTriggeredAt != nil {
			cooldownEnd := rule.LastTriggeredAt.Add(time.Duration(rule.CooldownMinutes) * time.Minute)
			if time.Now().Before(cooldownEnd) {
				continue
			}
		}

		triggered, message := e.checkCondition(rule, metrics)
		if !triggered {
			continue
		}

		// Get channel
		channel, err := e.notifRepo.GetChannel(rule.ChannelID)
		if err != nil {
			log.Printf("Alert evaluator: channel %s not found: %v", rule.ChannelID, err)
			continue
		}

		title := fmt.Sprintf("CineVault Alert: %s", rule.Name)
		sendErr := e.sender.Send(channel, title, message)

		// Log the alert
		success := sendErr == nil
		var errDetail *string
		if sendErr != nil {
			s := sendErr.Error()
			errDetail = &s
			log.Printf("Alert evaluator: failed to send alert %q: %v", rule.Name, sendErr)
		} else {
			log.Printf("Alert evaluator: sent alert %q to %s", rule.Name, channel.Name)
		}

		_ = e.notifRepo.LogAlert(&rule.ID, &rule.ChannelID, message, success, errDetail)
		_ = e.notifRepo.UpdateLastTriggered(rule.ID)
	}
}

func (e *AlertEvaluator) checkCondition(rule *models.AlertRule, metrics *models.SystemMetric) (bool, string) {
	switch rule.ConditionType {
	case "disk_space_low":
		if metrics.DiskFreeGB > 0 && metrics.DiskFreeGB < rule.Threshold {
			return true, fmt.Sprintf("Disk space is low: %.1f GB free (threshold: %.1f GB)", metrics.DiskFreeGB, rule.Threshold)
		}

	case "gpu_temp_high":
		if metrics.GPUTempCelsius != nil && *metrics.GPUTempCelsius > rule.Threshold {
			return true, fmt.Sprintf("GPU temperature is high: %.0f°C (threshold: %.0f°C)", *metrics.GPUTempCelsius, rule.Threshold)
		}

	case "transcode_failure":
		count, err := e.notifRepo.TranscodeFailureCount(60)
		if err == nil && count > int(rule.Threshold) {
			return true, fmt.Sprintf("High transcode failure rate: %d failures in the last hour (threshold: %.0f)", count, rule.Threshold)
		}

	case "stream_error":
		count, err := e.notifRepo.StreamErrorCount(60)
		if err == nil && count > int(rule.Threshold) {
			return true, fmt.Sprintf("High stream error rate: %d errors in the last hour (threshold: %.0f)", count, rule.Threshold)
		}
	}

	return false, ""
}
