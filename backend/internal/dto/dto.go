package dto

import (
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/fazamuttaqien/calendly/pkg/enum"
	"github.com/go-playground/validator/v10"
)

// --- Auth DTO ---

type RegisterDto struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginDto struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// --- Availability DTO ---

type DayAvailabilityDto struct {
	Day         enum.DayOfWeek `json:"day" validate:"required,oneof=SUNDAY MONDAY TUESDAY WEDNESDAY THURSDAY FRIDAY SATURDAY"`
	StartTime   string         `json:"startTime" validate:"required,time_hm"`
	EndTime     string         `json:"endTime" validate:"required,time_hm"`
	IsAvailable bool           `json:"isAvailable" validate:"required,boolean"`
}

type UpdateAvailabilityDto struct {
	TimeGap int                  `json:"timeGap" validate:"required,gte=0"`
	Days    []DayAvailabilityDto `json:"days" validate:"required,dive"`
}

// --- Event DTO ---

type CreateEventDto struct {
	Title        string                 `json:"title" validate:"required"`
	Description  string                 `json:"description" validate:"omitempty"`
	Duration     int                    `json:"duration" validate:"required,gte=1"`
	LocationType enum.EventLocationType `json:"locationType" validate:"required,oneof=GOOGLE_MEET_AND_CALENDAR ZOOM_MEETING"`
}

type UserEventScanDto struct {
	// User fields (guaranteed non-null if row exists)
	UserID   string `db:"user_id"`
	Username string `db:"username"`

	// Event fields (nullable due to LEFT JOIN)
	EventID           sql.NullString `db:"event_id"`
	EventTitle        sql.NullString `db:"event_title"`
	EventDescription  sql.NullString `db:"event_description"`
	EventDuration     sql.NullInt64  `db:"event_duration"`
	EventSlug         sql.NullString `db:"event_slug"`
	EventIsPrivate    sql.NullBool   `db:"event_is_private"`
	EventLocationType sql.NullString `db:"event_location_type"`
	EventCreatedAt    sql.NullTime   `db:"event_created_at"`
	EventUpdatedAt    sql.NullTime   `db:"event_updated_at"`
}

// Note: For 'oneof', list the *string* values of the enum constants.

// --- Param/Query DTO (Often used for URL parameters or query strings) ---

// EventIdDto is typically used for path parameters like /events/{eventId}
type EventIdDto struct {
	EventID string `param:"eventId" validate:"required,uuid4"`
}

// UserNameDto is typically used for path parameters like /users/{username}
type UserNameDto struct {
	Username string `param:"username" validate:"required"`
}

// UserNameAndSlugDto is typically used for path parameters like /users/{username}/events/{slug}
type UserNameAndSlugDto struct {
	Username string `param:"username" validate:"required"`
	Slug     string `param:"slug" validate:"required"`
}

// AppTypeDTO could be used for query parameters like /integrations?appType=...
type AppTypeDTO struct {
	AppType enum.IntegrationAppType `query:"appType" validate:"required,oneof=GOOGLE_MEET_AND_CALENDAR ZOOM_MEETING OUTLOOK_CALENDAR"`
}

// --- Meeting DTO ---

// Note on IsDateString: Go validator common use is RFC3339 format.
// Use 'datetime=YYYY-MM-DDTHH:mm:ssZ07:00' or a specific layout.
// Often, you validate it's a string, then parse to time.Time in the handler.
// Here, we'll use a basic datetime validation. Adjust format as needed.
const rfc3339Full = "2006-01-02T15:04:05Z07:00"

type CreateMeetingDto struct {
	EventID        string    `json:"eventId" validate:"required,uuid4"`
	StartTime      time.Time `json:"startTime" validate:"required"`
	EndTime        time.Time `json:"endTime" validate:"required,gtfield=StartTime"`
	GuestName      string    `json:"guestName" validate:"required"`
	GuestEmail     string    `json:"guestEmail" validate:"required,email"`
	AdditionalInfo string    `json:"additionalInfo" validate:"omitempty"`
}

// MeetingIdDto is typically used for path parameters like /meetings/{meetingId}
type MeetingIdDto struct {
	MeetingID string `param:"meetingId" validate:"required,uuid4"`
}

// --- Helper to add custom time validation ---
// You would register this with your validator instance

var timeRegex = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

func ValidateTimeHM(fl validator.FieldLevel) bool {
	return timeRegex.MatchString(fl.Field().String())
}

// In your init() where you create the validator:
// validate.RegisterValidation("time_hm", ValidateTimeHM)

// --- Helper to join enum values for 'oneof' tag ---
// (Could be generated or put in a utility package)
// Usage example in tags: validate:"required,oneof="+joinEnumValues(codes.IntegrationAppTypeValues())
func joinEnumValues(values []string) string {
	return strings.Join(values, " ")
}
