package controller

import (
	"database/sql"
	"fmt"
	"net/http"
	"slices"

	"github.com/fazamuttaqien/calendly/helper"
	"github.com/fazamuttaqien/calendly/internal/dto"
	"github.com/fazamuttaqien/calendly/internal/model"
	"github.com/fazamuttaqien/calendly/middleware"
	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
	"github.com/fazamuttaqien/calendly/pkg/enum"
	"github.com/fazamuttaqien/calendly/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// POST /events
func (e *Controller) CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		// This should ideally be caught by auth middleware, but good practice to check
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	dto, ok := validator.GetValidatedDTOFromContext[dto.CreateEventDto](ctx)
	if !ok {
		// Should be caught by validation middleware normally
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Validated DTO not found in context", nil))
		return
	}

	// Basic validation (can also rely on DB enum constraint)
	isValidLocation := slices.Contains(enum.AllEventLocationType(), dto.LocationType)

	if !isValidLocation {
		appError.WriteError(w, appError.NewAppError(enum.ValidationError, "Invalid location type provided", nil))
		return
	}

	slug := helper.Slugify(dto.Title)

	var event model.Event
	query := `
		INSERT INTO events (user_id, title, description, duration, slug, location_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, user_id, title, description, duration, slug, is_private, location_type, created_at, updated_at
	`

	// Use sql.NullString for optional description
	var description sql.NullString
	if dto.Description != "" {
		description = sql.NullString{
			String: dto.Description,
			Valid:  true,
		}
	}

	err := e.db.GetContext(ctx, &event, query,
		userID, dto.Title, description, dto.Duration, slug, dto.LocationType)
	if err != nil {
		// Consider checking for specific DB errors like unique constraint violations if needed
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to create event", err))
		return
	}

	response := map[string]any{
		"message": "Event created successfully",
		"event":   event,
	}
	helper.ResponseJson(w, http.StatusCreated, response)
}

// GET /me/events
func (e *Controller) GetUserEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		// This should ideally be caught by auth middleware, but good practice to check
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	// 1. Check if user exists and get username
	var username string
	errUser := e.db.GetContext(ctx, &username, "SELECT username FROM users WHERE id = $1", userID)
	if errUser != nil {
		if errUser == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError("User", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to check user existence", errUser))
		return
	}

	// 2. Scan User and potentially NULL Event data using userEventScanDTO
	var scanResults []dto.UserEventScanDto

	// Query with aliases matching the scan DTO's db tags
	userEventsQuery := `
	SELECT
		u.id           AS user_id,
		u.username     AS username,
		e.id           AS event_id,
		e.title        AS event_title,
		e.description  AS event_description,
		e.duration     AS event_duration,
		e.slug         AS event_slug,
		e.is_private   AS event_is_private,
		e.location_type AS event_location_type,
		e.created_at   AS event_created_at,
		e.updated_at   AS event_updated_at
	FROM users u
	LEFT JOIN events e ON u.id = e.user_id -- LEFT JOIN is the key part
	WHERE u.id = $1
	ORDER BY e.created_at DESC; -- Ordering by event creation might put NULL events first/last depending on DB
`

	if err := e.db.SelectContext(ctx, &scanResults, userEventsQuery, userID); err != nil && err != sql.ErrNoRows { // Ignore ErrNoRows here, handled by initial user check
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to retrieve user events data", err))
		return
	}

	// 3. Process scan results, filtering out NULL events
	validEventsMap := make(map[string]model.Event)
	eventID := make([]string, 0, len(scanResults))

	for _, row := range scanResults {
		// Check if the event ID is valid (meaning the LEFT JOIN found a matching event)
		if row.EventID.Valid {
			// Construct the non-nullable models.Event from the valid scan DTO fields
			event := model.Event{
				ID:           row.EventID.String,
				UserID:       row.UserID,                  // UserID is guaranteed non-null here
				Title:        row.EventTitle.String,       // Assume title is NOT NULL in DB based on entity
				Description:  row.EventDescription.String, // Assign NullString directly
				Duration:     row.EventDuration.Int64,
				Slug:         row.EventSlug.String, // Assume slug is NOT NULL
				IsPrivate:    row.EventIsPrivate.Bool,
				LocationType: enum.EventLocationType(row.EventLocationType.String), // Convert string to enum
				CreatedAt:    row.EventCreatedAt.Time,
				UpdatedAt:    row.EventUpdatedAt.Time,
			}
			// Add validation checks for required fields if necessary based on Null* types
			validEventsMap[event.ID] = event
			eventID = append(eventID, event.ID)
		}
	}

	// If no valid events were found after scanning, return early
	if len(eventID) == 0 {
		appError.WriteError(w, appError.NewAppError(enum.ResourceNotFound, "Events not found", nil))
		return
	}

	// 4. Get Meeting Counts for the *valid* Event ID
	type meetingCountResult struct {
		EventID string `db:"event_id"`
		Count   int    `db:"count"`
	}
	var counts []meetingCountResult

	countQuery, args, err := sqlx.In(`
		SELECT event_id, COUNT(*) as count
		FROM meetings
		WHERE event_id IN (?)
		GROUP BY event_id;
	`, eventID)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to build meeting count query", err))
		return
	}

	countQuery = e.db.Rebind(countQuery)
	err = e.db.SelectContext(ctx, &counts, countQuery, args...)
	if err != nil && err != sql.ErrNoRows {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to retrieve meeting counts", err))
		return
	}

	// Map counts back to the events
	countsMap := make(map[string]int)
	for _, c := range counts {
		countsMap[c.EventID] = c.Count
	}

	// 5. Construct final result with counts
	finalEventsWithCount := make([]EventWithCount, 0, len(validEventsMap))
	// Iterate over eventID to maintain the original query's order (approximated)
	// Note: A more robust ordering might require storing the original order or re-querying events.
	// For simplicity, iterating eventIDs collected earlier.
	processedOrder := make(map[string]bool) // To handle potential duplicates if scanResults had them (unlikely with PK)
	for _, id := range eventID {
		if _, done := processedOrder[id]; !done {
			event := validEventsMap[id]
			finalEventsWithCount = append(finalEventsWithCount, EventWithCount{
				Event:        event,
				MeetingCount: countsMap[event.ID],
			})
			processedOrder[id] = true
		}
	}

	// Construct the specific response structure from TS
	response := map[string]any{
		"message": "User event fetched successfully",
		"data": map[string]any{
			"events":   finalEventsWithCount,
			"username": username,
		},
	}

	helper.ResponseJson(w, http.StatusOK, response)
}

// PATCH /events/{eventId}/privacy
func (e *Controller) TogglePrivacy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing eventId in path", nil))
		return
	}
	// Optional: Add UUID validation here if needed,
	// although service layer might handle invalid ID format error

	var event model.Event
	query := `
		UPDATE events
		SET is_private = NOT is_private, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, title, description, duration, slug, is_private, location_type, created_at, updated_at
	`

	err := e.db.GetContext(ctx, &event, query, eventID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError(fmt.Sprintf("Event with ID %s for user", eventID), nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to toggle event privacy", err))
		return
	}

	privacyStatus := "public"
	if event.IsPrivate {
		privacyStatus = "private"
	}

	message := fmt.Sprintf("Event sent to %s successfully", privacyStatus)
	helper.ResponseJson(w, http.StatusOK, helper.SimpleMessage{Message: message})
}

// GET /public/users/{username}/events
func (e *Controller) GetPublicByUsername(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := chi.URLParam(r, "username")
	if username == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing username in path", nil))
		return
	}

	var results []struct {
		// User fields (prefixed u_)
		UserID       string         `db:"u_id"`
		UserName     string         `db:"u_name"`
		UserImageURL sql.NullString `db:"u_image_url"`

		// Event fields (prefixed e_) - use Null types for LEFT JOIN safety
		EventID           sql.NullString `db:"e_id"`
		EventTitle        sql.NullString `db:"e_title"`
		EventDescription  sql.NullString `db:"e_description"`
		EventSlug         sql.NullString `db:"e_slug"`
		EventDuration     sql.NullInt64  `db:"e_duration"` // Use NullInt64 for nullable integers
		EventLocationType sql.NullString `db:"e_location_type"`
		EventCreatedAt    sql.NullTime   `db:"e_created_at"`
		EventUpdatedAt    sql.NullTime   `db:"e_updated_at"`
	}

	query := `
		SELECT
			u.id         AS u_id,
			u.name       AS u_name,
			u.image_url  AS u_image_url,
			e.id         AS e_id,
			e.title      AS e_title,
			e.description AS e_description,
			e.slug       AS e_slug,
			e.duration   AS e_duration,
			e.location_type AS e_location_type,
            e.created_at AS e_created_at,
            e.updated_at AS e_updated_at
		FROM users u
		LEFT JOIN events e ON u.id = e.user_id AND e.is_private = FALSE
		WHERE u.username = $1
		ORDER BY e.created_at DESC;
	`

	err := e.db.SelectContext(ctx, &results, query, username)
	if err != nil && err != sql.ErrNoRows {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to retrieve public events", err))
		return
	}

	if len(results) == 0 {
		// No user found with that username
		appError.WriteError(w, appError.NewNotFoundError("User", nil))
	}

	// User found, extract user info and events
	userInfo := PublicUserInfo{
		ID:       results[0].UserID,
		Name:     results[0].UserName,
		ImageURL: results[0].UserImageURL,
	}

	events := make([]model.Event, 0, len(results))
	for _, row := range results {
		// Check if the event part is valid (e.g., EventID is not NULL)
		if row.EventID.Valid {
			events = append(events, model.Event{
				ID:           row.EventID.String,
				UserID:       userInfo.ID,
				Title:        row.EventTitle.String,
				Description:  row.EventDescription.String,
				Duration:     row.EventDuration.Int64,
				Slug:         row.EventSlug.String,
				IsPrivate:    false,
				LocationType: enum.EventLocationType(row.EventLocationType.String),
				CreatedAt:    row.EventCreatedAt.Time,
				UpdatedAt:    row.EventUpdatedAt.Time,
			})
		}
	}

	// Construct response matching TS structure
	response := map[string]any{
		"message": "Public events fetched successfully",
		"user": map[string]any{
			"name":     userInfo.Name,
			"username": username,
			"imageUrl": userInfo.ImageURL.String,
		},
		"events": events,
	}

	// Handle null image URL cleanly
	if !userInfo.ImageURL.Valid {
		response["user"].(map[string]any)["imageUrl"] = nil
	}

	helper.ResponseJson(w, http.StatusOK, response)
}

// GET /public/users/{username}/events/{slug}
func (e *Controller) GetPublicBySlug(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := chi.URLParam(r, "username")
	slug := chi.URLParam(r, "slug")

	if username == "" || slug == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing username or slug in path", nil))
		return
	}

	dto := dto.UserNameAndSlugDto{
		Username: username,
		Slug:     slug,
	}

	// We need to scan into a flat struct first because sqlx doesn't directly support nested structs with prefixes easily during GetContext
	var flatResult struct {
		model.Event
		UserID       string         `db:"user_id"`
		UserName     string         `db:"user_name"`
		UserImageURL sql.NullString `db:"user_image_url"`
	}

	query := `
		SELECT
			e.id, e.user_id, e.title, e.description, e.duration, e.slug, e.is_private, e.location_type, e.created_at, e.updated_at,
			u.id as user_id, u.name as user_name, u.image_url as user_image_url
		FROM events e
		JOIN users u ON e.user_id = u.id
		WHERE u.username = $1 AND e.slug = $2 AND e.is_private = FALSE;
	`

	err := e.db.GetContext(ctx, &flatResult, query, dto.Username, dto.Slug)
	if err != nil {
		if err == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError("Public Event", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to retrieve public event", err))
		return
	}

	// Construct the nested result
	result := &EventWithPublicUserInfo{
		Event: flatResult.Event,
		User: PublicUserInfo{
			ID:       flatResult.UserID,
			Name:     flatResult.UserName,
			ImageURL: flatResult.UserImageURL,
		},
	}

	// Ensure the embedded Event's UserID is correct (might be overwritten by user_id scan)
	result.Event.UserID = sql.NullString{String: flatResult.UserID, Valid: true}.String

	response := map[string]any{
		"message": "Event details fetched successfully",
		"event":   result,
	}

	helper.ResponseJson(w, http.StatusOK, response)
}

// DELETE /events/{eventId}
func (e *Controller) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing eventId in path", nil))
		return
	}
	// Optional: Add UUID validation

	query := `DELETE FROM events WHERE id = $1 AND user_id = $2;`

	result, err := e.db.ExecContext(ctx, query, eventID, userID)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to delete event", err))
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		appError.WriteError(w, appError.NewNotFoundError("Could not get rows affected after delete", err))
		return
	}

	if affected == 0 {
		// Event not found for this user or already deleted
		appError.WriteError(w, appError.NewNotFoundError(fmt.Sprintf("Event with ID %s for user", eventID), nil))
		return
	}

	helper.ResponseJson(w, http.StatusOK, helper.SimpleMessage{Message: "Event deleted successfully"})
}
