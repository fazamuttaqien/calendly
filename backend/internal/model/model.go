package model

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/fazamuttaqien/calendly/pkg/enum"
)

type User struct {
	ID        string         `db:"id" json:"id"`
	Name      string         `db:"name" json:"name"`
	Username  string         `db:"username" json:"username"`
	Email     string         `db:"email" json:"email"`
	Password  string         `db:"password" json:"-"`
	ImageURL  sql.NullString `db:"image_url" json:"imageUrl"`
	CreatedAt time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time      `db:"updated_at" json:"updatedAt"`
}

type Availability struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	TimeGap   int       `db:"time_gap" json:"timeGap"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type DayAvailability struct {
	ID             string         `db:"id" json:"id"`
	AvailabilityID string         `db:"availability_id" json:"availabilityId"`
	Day            enum.DayOfWeek `db:"day" json:"day"`
	StartTime      string         `db:"start_time" json:"startTime"`
	EndTime        string         `db:"end_time" json:"endTime"`
	IsAvailable    bool           `db:"is_available" json:"isAvailable"`
	CreatedAt      time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updatedAt"`
}

type Event struct {
	ID           string                 `db:"id" json:"id"`
	UserID       string                 `db:"user_id" json:"userId"`
	Title        string                 `db:"title" json:"title"`
	Description  string                 `db:"description" json:"description"`
	Duration     int64                  `db:"duration" json:"duration"`
	Slug         string                 `db:"slug" json:"slug"`
	IsPrivate    bool                   `db:"is_private" json:"isPrivate"`
	LocationType enum.EventLocationType `db:"location_type" json:"locationType"`
	CreatedAt    time.Time              `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time              `db:"updated_at" json:"updatedAt"`
}

// Integration represents the 'integrations' table.
type Integration struct {
	ID           string                   `db:"id" json:"id"`
	UserID       string                   `db:"user_id" json:"userId"` // Foreign key
	Provider     enum.IntegrationProvider `db:"provider" json:"provider"`
	Category     enum.IntegrationCategory `db:"category" json:"category"`
	AppType      enum.IntegrationAppType  `db:"app_type" json:"appType"`
	AccessToken  sql.NullString           `db:"access_token" json:"-"`              // Often sensitive, exclude from default JSON
	RefreshToken sql.NullString           `db:"refresh_token" json:"-"`             // Often sensitive, exclude from default JSON
	ExpiryDate   sql.NullInt64            `db:"expiry_date" json:"-"`               // Exclude expiry details from default JSON
	Metadata     json.RawMessage          `db:"metadata" json:"metadata,omitempty"` // Use json.RawMessage for flexibility with JSONB
	IsConnected  bool                     `db:"is_connected" json:"isConnected"`
	CreatedAt    time.Time                `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time                `db:"updated_at" json:"updatedAt"`
	User         User                     `db:"user" json:"-"` // Example: Add if frequently needed via JOIN, exclude from JSON
}

// Meeting represents the 'meetings' table.
type Meeting struct {
	ID              string             `db:"id" json:"id"`
	UserID          string             `db:"user_id" json:"userId"` // User who owns the event slot
	EventID         string             `db:"event_id" json:"eventId"`
	GuestName       string             `db:"guest_name" json:"guestName"`
	GuestEmail      string             `db:"guest_email" json:"guestEmail"`
	AdditionalInfo  string             `db:"additional_info" json:"additionalInfo,omitempty"`
	StartTime       time.Time          `db:"start_time" json:"startTime"`
	EndTime         time.Time          `db:"end_time" json:"endTime"`
	MeetLink        string             `db:"meet_link" json:"meetLink"`                // Assuming not nullable based on SQL
	CalendarEventID string             `db:"calendar_event_id" json:"calendarEventId"` // Assuming not nullable
	CalendarAppType string             `db:"calendar_app_type" json:"calendarAppType"` // Assuming not nullable
	Status          enum.MeetingStatus `db:"status" json:"status"`
	CreatedAt       time.Time          `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time          `db:"updated_at" json:"updatedAt"`
	// Event           Event               `db:"event" json:"event"` // Example: Add if frequently needed via JOIN, exclude from JSON

	// --- Example fields if joining Event data often ---
	// These require specific SELECT aliases (e.g., "e.title AS event_title")

	EventTitle        string                 `db:"event_title" json:"eventTitle,omitempty"`
	EventDescription  string                 `db:"event_description" json:"eventDescription,omitempty"`
	EventLocationType enum.EventLocationType `db:"event_location_type" json:"eventLocationType,omitempty"` // Needs alias
}
