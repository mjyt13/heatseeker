package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/app/notify"
	"heatseeker/api/internal/domain"
)

// NotificationDTO is one line of the bell list.
type NotificationDTO struct {
	ID      uuid.UUID `json:"id"`
	GroupID uuid.UUID `json:"group_id"`
	Type    string    `json:"type" enum:"MESSAGE_NEW,MESSAGE_REPLY,MATERIAL_ADDED,MATERIAL_BATCH,TASK_CREATED,TASK_PINNED,TASK_DUE_SOON,TASK_OVERDUE,TASK_STATUS_CHANGED,SCHEDULE_CHANGED,MEMBER_JOINED,ANNOUNCEMENT,REMINDER,PROPOSAL_NEW,MODERATION"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	// Data carries the deep link: {"screen":"thread","thread_id":"…"}.
	Data      json.RawMessage `json:"data" doc:"Куда вести по нажатию: {\"screen\":\"thread\",\"thread_id\":\"…\"}."`
	ReadAt    *time.Time      `json:"read_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

func toNotificationDTO(n *domain.Notification) NotificationDTO {
	return NotificationDTO{
		ID: n.ID, GroupID: n.GroupID, Type: string(n.Type), Title: n.Title, Body: n.Body,
		Data: n.Data, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt,
	}
}

// NotificationPrefDTO is one switch on the settings screen.
type NotificationPrefDTO struct {
	Type    string `json:"type" enum:"MESSAGE_NEW,MESSAGE_REPLY,MATERIAL_ADDED,MATERIAL_BATCH,TASK_CREATED,TASK_PINNED,TASK_DUE_SOON,TASK_OVERDUE,TASK_STATUS_CHANGED,SCHEDULE_CHANGED,MEMBER_JOINED,ANNOUNCEMENT,REMINDER,PROPOSAL_NEW,MODERATION"`
	Enabled bool   `json:"enabled"`
	Custom  bool   `json:"custom" doc:"true — участник задал это сам; false — значение по умолчанию."`
}

func toPrefDTO(p notify.Pref) NotificationPrefDTO {
	return NotificationPrefDTO{Type: string(p.Type), Enabled: p.Enabled, Custom: p.Custom}
}

// NotificationSettingsDTO holds the personal delivery rules.
type NotificationSettingsDTO struct {
	PushEnabled bool   `json:"push_enabled"`
	QuietFrom   *int16 `json:"quiet_from,omitempty" minimum:"0" maximum:"1439" doc:"Минуты от полуночи по часам участника; нет — без тихих часов."`
	QuietTo     *int16 `json:"quiet_to,omitempty" minimum:"0" maximum:"1439"`
	// UrgentInQuiet is the only way anything reaches the phone at night.
	UrgentInQuiet bool `json:"urgent_in_quiet,omitempty" doc:"Пускать срочные объявления в тихие часы; по умолчанию нет."`
}

func toSettingsDTO(s domain.NotificationSettings) NotificationSettingsDTO {
	return NotificationSettingsDTO{
		PushEnabled: s.PushEnabled, QuietFrom: s.QuietFrom, QuietTo: s.QuietTo,
		UrgentInQuiet: s.UrgentInQuiet,
	}
}

// MuteDTO is one silenced scope.
type MuteDTO struct {
	ID        uuid.UUID `json:"id"`
	GroupID   uuid.UUID `json:"group_id"`
	ScopeType string    `json:"scope_type" enum:"GROUP,SUBJECT,THREAD,TYPE"`
	ScopeID   string    `json:"scope_id" doc:"id предмета или обсуждения; для TYPE — тип уведомления."`
	Until     time.Time `json:"until"`
}

func toMuteDTO(m *domain.NotificationMute) MuteDTO {
	return MuteDTO{ID: m.ID, GroupID: m.GroupID, ScopeType: string(m.Scope), ScopeID: m.ScopeID, Until: m.Until}
}
