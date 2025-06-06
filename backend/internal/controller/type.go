package controller

import (
	"database/sql"

	"github.com/fazamuttaqien/calendly/internal/model"
	"github.com/fazamuttaqien/calendly/pkg/enum"
)

type EventWithCount struct {
	model.Event
	MeetingCount int `db:"meeting_count"`
}

type UserWithEvents struct {
	Username string `db:"username"`
	Events   []EventWithCount
}

type PublicUserInfo struct {
	ID       string         `db:"id"`
	Name     string         `db:"name"`
	ImageURL sql.NullString `db:"image_url"`
}

type PublicUserInfoWithEvents struct {
	User   PublicUserInfo
	Events []model.Event
}

type EventWithPublicUserInfo struct {
	model.Event
	User PublicUserInfo `db:"user"`
}

type AvailabilityResponse struct {
	TimeGap int                     `json:"timeGap"`
	Days    []DayAvailabilityDetail `json:"days"`
}

type DayAvailabilityDetail struct {
	Day         enum.DayOfWeek `json:"day"`
	StartTime   string         `json:"startTime"`
	EndTime     string         `json:"endTime"`
	IsAvailable bool           `json:"isAvailable"`
}

type DailyAvailabilitySlots struct {
	Day         enum.DayOfWeek `json:"day"`
	Slots       []string       `json:"slots"`
	IsAvailable bool           `json:"isAvailable"`
}

// --- Helper Struct for DB Scan in GetUserAvailability ---
type AvailabilityDetail struct {
	TimeGap int            `db:"time_gap"`
	Day     enum.DayOfWeek `db:"day"`
	// Read TIME type as string initially, parse later
	StartTime   string `db:"start_time"`
	EndTime     string `db:"end_time"`
	IsAvailable bool   `db:"is_available"`
}

type IntegrationStatus struct {
	Provider    enum.IntegrationProvider `json:"provider"`
	Title       string                   `json:"title"`
	AppType     enum.IntegrationAppType  `json:"app_type"`
	Category    enum.IntegrationCategory `json:"category"`
	IsConnected bool                     `json:"isConnected"`
}

type CreateIntegration struct {
	UserID       string
	AppType      enum.IntegrationAppType
	AccessToken  string
	RefreshToken sql.NullString
	ExpiryDate   sql.NullInt64
	Metadata     any
}
