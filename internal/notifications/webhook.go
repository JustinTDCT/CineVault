package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"strings"
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
	case "telegram":
		return w.sendTelegram(channel, title, message)
	case "pushover":
		return w.sendPushover(channel, title, message)
	case "gotify":
		return w.sendGotify(channel, title, message)
	case "email":
		return w.sendEmail(channel, title, message)
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

// ── Telegram ──

func (w *WebhookSender) sendTelegram(channel *models.NotificationChannel, title, message string) error {
	config := channel.GetConfig()
	botToken := config["bot_token"]
	chatID := config["chat_id"]
	if botToken == "" || chatID == "" {
		return fmt.Errorf("telegram requires bot_token and chat_id in config")
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       fmt.Sprintf("*%s*\n%s", title, message),
		"parse_mode": "Markdown",
	}
	return w.postJSON(url, payload)
}

// ── Pushover ──

func (w *WebhookSender) sendPushover(channel *models.NotificationChannel, title, message string) error {
	config := channel.GetConfig()
	appToken := config["app_token"]
	userKey := config["user_key"]
	if appToken == "" || userKey == "" {
		return fmt.Errorf("pushover requires app_token and user_key in config")
	}
	payload := map[string]interface{}{
		"token":   appToken,
		"user":    userKey,
		"title":   title,
		"message": message,
	}
	return w.postJSON("https://api.pushover.net/1/messages.json", payload)
}

// ── Gotify ──

func (w *WebhookSender) sendGotify(channel *models.NotificationChannel, title, message string) error {
	config := channel.GetConfig()
	serverURL := config["server_url"]
	appToken := config["app_token"]
	if serverURL == "" || appToken == "" {
		return fmt.Errorf("gotify requires server_url and app_token in config")
	}
	serverURL = strings.TrimRight(serverURL, "/")
	url := fmt.Sprintf("%s/message", serverURL)

	payload := map[string]interface{}{
		"title":   title,
		"message": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", appToken)

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("gotify post: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gotify returned status %d", resp.StatusCode)
	}
	return nil
}

// ── Email (SMTP) ──

func (w *WebhookSender) sendEmail(channel *models.NotificationChannel, title, message string) error {
	config := channel.GetConfig()
	host := config["smtp_host"]
	port := config["smtp_port"]
	user := config["smtp_user"]
	pass := config["smtp_password"]
	from := config["smtp_from"]
	to := config["smtp_to"]
	if host == "" || from == "" || to == "" {
		return fmt.Errorf("email requires smtp_host, smtp_from, smtp_to in config")
	}
	if port == "" {
		port = "587"
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	subject := fmt.Sprintf("Subject: %s\r\n", title)
	mime := "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"
	body := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\n%s%s%s", from, to, subject, mime, message))

	var auth smtp.Auth
	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	return smtp.SendMail(addr, auth, from, strings.Split(to, ","), body)
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
