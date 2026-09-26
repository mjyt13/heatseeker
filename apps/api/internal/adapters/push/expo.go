// Package push delivers notifications to devices. Expo is the provider for
// the Android build; Web Push and Telegram plug in behind the same interface
// (docs/PLAN.md §8.1).
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"heatseeker/api/internal/domain"
)

// ExpoEndpoint is Expo's push service.
const ExpoEndpoint = "https://exp.host/--/api/v2/push/send"

// batchSize is Expo's limit for one request.
const batchSize = 100

// Expo sends through Expo's push service, which forwards to FCM and APNs.
type Expo struct {
	url    string
	token  string
	client *http.Client
}

// NewExpo builds the provider. The access token is optional: it is required
// only for projects with "enhanced security" switched on.
func NewExpo(accessToken string) *Expo {
	return &Expo{url: ExpoEndpoint, token: accessToken, client: &http.Client{Timeout: 20 * time.Second}}
}

// WithEndpoint points the provider at another URL (tests).
func (e *Expo) WithEndpoint(url string) *Expo {
	e.url = url
	return e
}

type expoMessage struct {
	To        string          `json:"to"`
	Title     string          `json:"title"`
	Body      string          `json:"body,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Sound     string          `json:"sound,omitempty"`
	ChannelID string          `json:"channelId,omitempty"`
	Priority  string          `json:"priority,omitempty"`
}

type expoTicket struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type expoResponse struct {
	Data   []expoTicket `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Push implements domain.Pusher.
func (e *Expo) Push(ctx context.Context, n domain.Notification, targets []domain.PushTarget) ([]domain.PushResult, error) {
	out := make([]domain.PushResult, 0, len(targets))
	for start := 0; start < len(targets); start += batchSize {
		end := min(start+batchSize, len(targets))
		chunk := targets[start:end]
		messages := make([]expoMessage, len(chunk))
		for i, t := range chunk {
			messages[i] = expoMessage{
				To: t.Token, Title: n.Title, Body: n.Body, Data: n.Data,
				Sound: "default", ChannelID: channelOf(n.Type), Priority: "high",
			}
		}
		tickets, err := e.send(ctx, messages)
		if err != nil {
			return out, err
		}
		for i, t := range chunk {
			r := domain.PushResult{DeviceID: t.DeviceID, Sent: true}
			if i < len(tickets) {
				ticket := tickets[i]
				if ticket.Status != "ok" {
					r.Sent = false
					r.Error = ticket.Message
					// The app was uninstalled or the token was replaced.
					r.Dead = ticket.Details.Error == "DeviceNotRegistered"
				}
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func (e *Expo) send(ctx context.Context, messages []expoMessage) ([]expoTicket, error) {
	body, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("encode push: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		// Worth another attempt later.
		return nil, fmt.Errorf("expo push: %s", resp.Status)
	}
	var parsed expoResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("expo push: %s: %w", resp.Status, err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("expo push: %s", parsed.Errors[0].Message)
	}
	return parsed.Data, nil
}

// channelOf names the Android notification channel, so the phone can rank and
// silence kinds of messages on its own. The app creates these channels.
func channelOf(t domain.NotificationType) string {
	switch t {
	case domain.NotifyMessageNew, domain.NotifyMessageReply:
		return "messages"
	case domain.NotifyTaskDueSoon, domain.NotifyTaskOverdue:
		return "deadlines"
	case domain.NotifyScheduleChanged:
		return "schedule"
	default:
		return "default"
	}
}
