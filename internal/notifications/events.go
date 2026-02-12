package notifications

import (
	"log"

	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

// EventType represents a notification event category.
type EventType string

const (
	EventNewContent       EventType = "new_content"
	EventRequestFulfilled EventType = "request_fulfilled"
	EventWatchlistAvail   EventType = "watchlist_available"
)

// EventDispatcher checks user notification preferences and sends notifications.
type EventDispatcher struct {
	notifRepo *repository.NotificationRepository
	sender    *WebhookSender
}

// NewEventDispatcher creates a new event dispatcher.
func NewEventDispatcher(notifRepo *repository.NotificationRepository, sender *WebhookSender) *EventDispatcher {
	return &EventDispatcher{
		notifRepo: notifRepo,
		sender:    sender,
	}
}

// Dispatch sends a notification event to all users subscribed to the event type.
func (d *EventDispatcher) Dispatch(eventType EventType, title, message string) {
	// Get all user preferences for this event type
	prefs, err := d.notifRepo.GetPreferencesForEvent(string(eventType))
	if err != nil {
		log.Printf("EventDispatcher: failed to get preferences for %s: %v", eventType, err)
		return
	}

	// Group by channel to avoid sending duplicates
	channelMessages := make(map[uuid.UUID]bool)
	for _, pref := range prefs {
		if !pref.Enabled || channelMessages[pref.ChannelID] {
			continue
		}
		channelMessages[pref.ChannelID] = true

		channel, err := d.notifRepo.GetChannel(pref.ChannelID)
		if err != nil || !channel.IsEnabled {
			continue
		}

		if err := d.sender.Send(channel, title, message); err != nil {
			log.Printf("EventDispatcher: failed to send %s to channel %s: %v", eventType, channel.Name, err)
		}
	}
}

// NotificationPreference represents a user's preference for a specific event type on a channel.
type NotificationPreference struct {
	UserID    uuid.UUID
	EventType string
	ChannelID uuid.UUID
	Enabled   bool
}
