package model

import (
	"time"
)

// Notification represents a notification event that can be sent to subscribers.
type Notification struct {
	// ID is the unique identifier for this notification.
	ID string `json:"id"`
	// Type is the notification type ("config_changed", "version_created", etc.).
	Type string `json:"type"`
	// AppID is the application context.
	AppID string `json:"app_id"`
	// Environment is the environment context.
	Environment string `json:"environment"`
	// Message is the notification message.
	Message string `json:"message"`
	// Data contains additional notification data.
	Data map[string]interface{} `json:"data,omitempty"`
	// CreatedAt is when the notification was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewNotification creates a new Notification.
func NewNotification(notifyType, appID, env, message string, data map[string]interface{}) *Notification {
	return &Notification{
		ID:          "notif-" + time.Now().Format("20060102150405.000000000"),
		Type:        notifyType,
		AppID:       appID,
		Environment: env,
		Message:     message,
		Data:        data,
		CreatedAt:   time.Now(),
	}
}

// NotificationTypes defines the supported notification types.
var NotificationTypes = map[string]string{
	"config_created":   "Configuration item created",
	"config_updated":   "Configuration item updated",
	"config_deleted":   "Configuration item deleted",
	"batch_updated":    "Configuration batch updated",
	"version_created":  "New version created",
	"rollback":         "Configuration rolled back",
	"validation_failed": "Configuration validation failed",
	"app_created":      "Application created",
	"app_deleted":      "Application deleted",
}

// NotificationSubscription represents a subscription to notification events.
type NotificationSubscription struct {
	// ID is the subscription identifier.
	ID string `json:"id"`
	// AppID is the application to subscribe to.
	AppID string `json:"app_id"`
	// Environment is the environment to subscribe to (empty for all).
	Environment string `json:"environment"`
	// EventTypes is the list of event types to subscribe to.
	EventTypes []string `json:"event_types"`
	// CallbackURL is the URL to receive webhook notifications.
	CallbackURL string `json:"callback_url"`
	// Active indicates if the subscription is active.
	Active bool `json:"active"`
	// CreatedAt is when the subscription was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewSubscription creates a new NotificationSubscription.
func NewSubscription(appID, env string, eventTypes []string, callbackURL string) *NotificationSubscription {
	return &NotificationSubscription{
		ID:          "sub-" + time.Now().Format("20060102150405.000000000"),
		AppID:       appID,
		Environment: env,
		EventTypes:  eventTypes,
		CallbackURL: callbackURL,
		Active:      true,
		CreatedAt:   time.Now(),
	}
}

// Matches checks if a subscription matches a notification.
func (s *NotificationSubscription) Matches(n *Notification) bool {
	if !s.Active {
		return false
	}
	if s.AppID != "" && s.AppID != n.AppID {
		return false
	}
	if s.Environment != "" && s.Environment != n.Environment {
		return false
	}
	if len(s.EventTypes) > 0 {
		found := false
		for _, et := range s.EventTypes {
			if et == n.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
