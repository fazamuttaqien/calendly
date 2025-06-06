package controller

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/fazamuttaqien/calendly/helper"
	"github.com/fazamuttaqien/calendly/internal/dto"
	"github.com/fazamuttaqien/calendly/internal/model"
	"github.com/fazamuttaqien/calendly/middleware"
	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
	"github.com/fazamuttaqien/calendly/pkg/enum"
	"github.com/fazamuttaqien/calendly/pkg/validator"

	"github.com/go-chi/chi/v5"
	"google.golang.org/api/calendar/v3"
)

// GET /me/meetings
func (m *Controller) GetUserMeetings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	// Get filter from query param, default to UPCOMING
	filterQuery := r.URL.Query().Get("filter")
	var filter enum.MeetingFilter

	switch strings.ToUpper(filterQuery) {
	case string(enum.MeetingFilterUpcoming):
		filter = enum.MeetingFilterUpcoming
	case string(enum.MeetingFilterPast):
		filter = enum.MeetingFilterPast
	case string(enum.MeetingFilterCancelled):
		filter = enum.MeetingFilterCancelled
	default:
		// Default to UPCOMING if empty or invalid
		filter = enum.MeetingFilterUpcoming
	}

	var meetings []model.Meeting

	var args []any
	args = append(args, userID)

	// Base query joining meetings and events
	baseQuery := `
		SELECT
			m.*,
			e.title AS event_title, -- Alias joined event fields
			e.description AS event_description,
            e.location_type AS event_location_type
            -- Add other event fields as needed
		FROM meetings m
		JOIN events e ON m.event_id = e.id
		WHERE m.user_id = $1
	`

	filterClause := ""

	now := time.Now()

	switch filter {
	case enum.MeetingFilterUpcoming:
		filterClause = " AND m.status = $2 AND m.start_time > $3"
		args = append(args, enum.Scheduled, now)
	case enum.MeetingFilterPast:
		filterClause = " AND m.status = $2 AND m.start_time < $3"
		args = append(args, enum.Scheduled, now)
	case enum.MeetingFilterCancelled:
		filterClause = " AND m.status = $2"
		args = append(args, enum.Cancelled)
	default: // Default to UPCOMING if filter is invalid or not provided
		filterClause = " AND m.status = $2 AND m.start_time > $3"
		args = append(args, enum.Scheduled, now)
	}

	orderByClause := " ORDER BY m.start_time ASC"
	finalQuery := baseQuery + filterClause + orderByClause + ";"

	err := m.db.SelectContext(ctx, &meetings, finalQuery, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError("No meetings found", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to retrieve useer meetings", err))
		return
	}

	response := map[string]any{
		"message":  "Meetings fetched successfully",
		"meetings": meetings,
	}
	helper.ResponseJson(w, http.StatusOK, response)
}

func (m *Controller) CreateBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dto, ok := validator.GetValidatedDTOFromContext[dto.CreateMeetingDto](ctx)
	if !ok {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Validated DTO not found in context", nil))
		return
	}

	// 1. Parse times
	// startTime, err := time.Parse(time.RFC3339, dto.StartTime.String()) // Assuming RFC3339 format from DTO
	// if err != nil {
	// 	appError.WriteError(w, appError.NewAppError(
	// 		enums.ValidationError,
	// 		"Invalid start time format",
	// 		err,
	// 	)
	// }

	// endTime, err := time.Parse(time.RFC3339, dto.EndTime.String()) // Assuming RFC3339 format from DTO
	// if err != nil {
	// 	appError.WriteError(w, appError.NewAppError(
	// 		enums.ValidationError,
	// 		"Invalid end time format",
	// 		err,
	// 	)
	// }

	startTime := dto.StartTime.Format(time.RFC3339)
	endTime := dto.EndTime.Format(time.RFC3339)

	// 2. Fetch Event and User
	var event model.Event // Assuming Event model has UserID field
	eventQuery := `SELECT e.* FROM events e WHERE e.id = $1 AND e.is_private = FALSE;`
	err := m.db.GetContext(ctx, &event, eventQuery, dto.EventID)
	if err != nil {
		if err == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError("Public event", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to fetch event", err))
		return
	}

	// Simple validation for location type enum (can be improved)
	isValidLocation := slices.Contains(enum.AllEventLocationType(), event.LocationType)
	if !isValidLocation {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, fmt.Sprintf("Event has invalid location type: %s", event.LocationType), nil))
		return
	}

	// 3. Fetch Integration for the event's use
	var integration model.Integration
	// Derive appType from event's locationType
	requiredAppType, ok := IntegrationAppTypeFromEventLocation(event.LocationType)
	if !ok {
		// This check might be redundant if previous validation passed, but safer
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Cannot map event location to integration app type", nil))
		return
	}

	integrationQuery := `SELECT * FROM integrations WHERE user_id = $1 AND app_type = $2 AND is_connected = TRUE;`
	err = m.db.GetContext(ctx, &integration, integrationQuery, event.UserID, requiredAppType)
	if err != nil {
		if err == sql.ErrNoRows {
			msg := fmt.Sprintf("Required integration '%s' not found or disconnected for the event owner.", requiredAppType)
			appError.WriteError(w, appError.NewAppError(enum.BadRequest, msg, nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to fetch integration", err))
		return
	}

	// 4. Interact with Calendar API (if applicable)
	meetLink := ""
	calendarEventID := ""
	calendarAppTypeStr := ""

	if event.LocationType == enum.LocationGoogleMeetAndCalendar {
		calendarSvc, appType, err := GetCalendarClient(ctx, integration)
		if err != nil {
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, err.Error(), err))
			return
		}
		calendarAppTypeStr = string(appType) // Store the string representation

		// Create Google Calendar event request
		calEvent := &calendar.Event{
			Summary:     fmt.Sprintf("%s - %s", dto.GuestName, event.Title),
			Description: dto.AdditionalInfo, // Assumes DTO field matches
			// Start:       &calendar.EventDateTime{DateTime: startTime.Format(time.RFC3339)},
			Start: &calendar.EventDateTime{DateTime: startTime},
			// End:         &calendar.EventDateTime{DateTime: endTime.Format(time.RFC3339)},
			End: &calendar.EventDateTime{DateTime: endTime},
			Attendees: []*calendar.EventAttendee{
				{Email: dto.GuestEmail},
				{Email: integration.User.Email}, // Assuming Integration model has UserEmail fetched or available
			},
			ConferenceData: &calendar.ConferenceData{
				CreateRequest: &calendar.CreateConferenceRequest{
					RequestId:             fmt.Sprintf("%s-%d", event.ID, time.Now().UnixNano()), // Unique request ID
					ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"}, // Request Google Meet
				},
			},
		}

		createdCalEvent, err := calendarSvc.Events.Insert("primary", calEvent).ConferenceDataVersion(1).Do()
		if err != nil {
			// Log detailed Google API error if possible
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to create calendar event", err))
			return
		}

		// Extract results
		meetLink = createdCalEvent.HangoutLink
		if createdCalEvent.Id == "" {
			// Handle case where ID might be missing unexpectedly
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Created calendar event missing ID", nil))
			return
		}
		calendarEventID = createdCalEvent.Id

	} else {
		// Handle other location types (e.g., Zoom) if necessary
		// For now, assume link/id remain empty if not Google
	}

	// 5. Insert Meeting into Database
	var createdMeeting model.Meeting
	insertQuery := `
	INSERT INTO meetings (
			user_id, event_id, guest_name, guest_email, additional_info,
			start_time, end_time, meet_link, calendar_event_id, calendar_app_type,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING *;
	`
	addInfo := sql.NullString{String: dto.AdditionalInfo, Valid: dto.AdditionalInfo != ""}

	err = m.db.GetContext(ctx, &createdMeeting, insertQuery,
		event.UserID, event.ID, dto.GuestName, dto.GuestEmail, addInfo,
		startTime, endTime, meetLink, calendarEventID, calendarAppTypeStr,
		enum.Scheduled, // Default status
	)
	if err != nil {
		// Consider handling specific DB errors like constraint violations
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to save meeting record", err))
		return
	}

	response := map[string]any{
		"message": "Meeting scheduled successfully",
		"data": map[string]any{
			"meetLink": meetLink,
			"meeting":  createdMeeting,
		},
	}
	helper.ResponseJson(w, http.StatusCreated, response)
}

// DELETE /meetings/{meetingId}
// NOTE: Assumes authorization is handled within the service layer based on meetingId,
// or via a separate mechanism (like a unique cancellation token/link) if public cancellation is allowed.
// If only the host can cancel, this route should be authenticated.
func (m *Controller) CancelMeeting(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meetingID := chi.URLParam(r, "meetingId")
	if meetingID == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing meetingId in path", nil))
		return
	}
	// Optional: Add UUID validation

	// If host cancellation is required, get userID from context and pass to service:
	// userID, ok := getUserIDFromContext(ctx)
	// if !ok {
	//     apperror.WriteError(w, apperror.NewUnauthorizedError(nil))
	//     return
	// }
	// err := h.meetingService.CancelMeeting(ctx, meetingID, userID) // Modified service signature

	// 1. Fetch Meeting, Event, and User info needed
	var meeting struct {
		model.Meeting
		EventUserID string `db:"event_user_id"`
	}
	fetchQuery := `
		SELECT m.*, e.user_id AS event_user_id
		FROM meetings m
		JOIN events e ON m.event_id = e.id
		WHERE m.id = $1;
	`
	err := m.db.GetContext(ctx, &meeting, fetchQuery, meetingID)
	if err != nil {
		if err == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError("Meeting", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to fetch meeting", err))
		return
	}

	// Check if already cancelled
	if meeting.Status == enum.Cancelled {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Meeting is already cancelled", nil))
		return
	}

	// 2. Attempt to delete from Calendar API (best effort)
	if meeting.CalendarEventID != "" && meeting.CalendarAppType != "" {
		calendarAppType := enum.IntegrationAppType(meeting.CalendarAppType) // Convert string back to enum type

		var integration model.Integration
		integrationQuery := `SELECT * FROM integrations WHERE user_id = $1 AND app_type = $2 AND is_connected = TRUE;`
		err = m.db.GetContext(ctx, &integration, integrationQuery, meeting.EventUserID, calendarAppType)

		if err != nil && err != sql.ErrNoRows {
			// Log error fetching integration, but proceed to DB cancel
			log.Printf("Warning: Failed to fetch integration for calendar deletion (MeetingID: %s): %v\n",
				meetingID, err)
		} else if err == nil { // Integration found
			calendarSvc, _, errClient := GetCalendarClient(ctx, integration) // Pass context
			if errClient != nil {
				// Log error getting client, but proceed to DB cancel
				log.Printf("Warning: Failed to get calendar client for deletion (MeetingID: %s): %v\n",
					meetingID, errClient)
			} else {
				// Call delete
				errDelete := calendarSvc.Events.Delete("primary", meeting.CalendarEventID).Do()
				if errDelete != nil {
					// IMPORTANT: Decide how critical calendar deletion failure is.
					// Log it, maybe notify someone, but allow DB cancellation?
					// Returning error here matches TS behavior.
					log.Printf("Warning: Failed to delete calendar event (MeetingID: %s, CalID: %s): %v\n",
						meetingID, meeting.CalendarEventID, errDelete)
					// Optionally return a specific error:
					// return apperror.NewAppError(enums.InternalServerError, "Failed to delete event from calendar", errDelete)
				} else {
					log.Printf("Successfully deleted calendar event (MeetingID: %s, CalID: %s)\n",
						meetingID, meeting.CalendarEventID)
				}
			}
		}
	}

	// 3. Update Meeting Status in DB
	updateQuery := `UPDATE meetings SET status = $1, updated_at = NOW() WHERE id = $2;`
	result, err := m.db.ExecContext(ctx, updateQuery, enum.Cancelled, meetingID)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to update meeting status", err))
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Should not happen if fetch succeeded, but good check
		appError.WriteError(w, appError.NewNotFoundError("Meeting (for update)", nil))
		return
	}

	helper.ResponseJson(w, http.StatusOK, helper.SimpleMessage{Message: "Meeting cancelled successfully"})
}
