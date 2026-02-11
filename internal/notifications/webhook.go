package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
)

// WebhookSender sends messages to notification channels.
type WebhookSender struct {
	client *http.Client
}

// NewWebhookSender creates a new sender with a timeout.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send dispatches a message to the given channel.
func (w *WebhookSender) Send(channel *models.NotificationChannel, title, message string) error {
	switch channel.ChannelType {
	case "discord":
		return w.sendDiscord(channel.WebhookURL, title, message)
	case "slack":
		return w.sendSlack(channel.WebhookURL, title, message)
	case "generic":
		return w.sendGeneric(channel.WebhookURL, title, message)
	default:
		return fmt.Errorf("unknown channel type: %s", channel.ChannelType)
	}
}

// SendTest sends a test message to validate the webhook.
func (w *WebhookSender) SendTest(channel *models.NotificationChannel) error {
	return w.Send(channel, "CineVault Test", "This is a test notification from CineVault. Your webhook is working correctly!")
}

func (w *WebhookSender) sendDiscord(url, title, message string) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": message,
				"color":       5814783, // CineVault blue
				"footer": map[string]string{
					"text": "CineVault Analytics",
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
	return w.postJSON(url, payload)
}

func (w *WebhookSender) sendSlack(url, title, message string) error {
	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": title,
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": message,
				},
			},
			{
				"type": "context",
				"elements": []map[string]string{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("_CineVault Analytics · %s_", time.Now().Format("Jan 2, 3:04 PM")),
					},
				},
			},
		},
	}
	return w.postJSON(url, payload)
}

func (w *WebhookSender) sendGeneric(url, title, message string) error {
	payload := map[string]interface{}{
		"title":     title,
		"message":   message,
		"source":    "cinevault",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return w.postJSON(url, payload)
}

func (w *WebhookSender) postJSON(url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := w.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		log.Printf("Webhook: %s returned status %d", url, resp.StatusCode)
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
