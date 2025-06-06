package controller

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
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
)

// GET /me/availability
func (a *Controller) GetUserAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	var dbDetail []AvailabilityDetail
	query := `
		SELECT
			a.time_gap,
			d.day,
			d.start_time::TEXT, -- Cast TIME to TEXT for easier parsing in Go
			d.end_time::TEXT,   -- Cast TIME to TEXT
			d.is_available
		FROM availability a
		JOIN day_availability d ON a.id = d.availability_id
		WHERE a.user_id = $1;
	`

	err := a.db.SelectContext(ctx, &dbDetail, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			var exists bool
			errCheck := a.db.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID)
			if errCheck == nil && exists {
				appError.WriteError(w, appError.NewNotFoundError("User availability", nil))
				return
			}
			appError.WriteError(w, appError.NewNotFoundError("User", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to retrieve user availability", err))
	}

	if len(dbDetail) == 0 {
		appError.WriteError(w, appError.NewNotFoundError("User availability data", nil))
		return
	}

	availability := &AvailabilityResponse{
		TimeGap: dbDetail[0].TimeGap, //	TimeGap is the same for all rows for this user
		Days:    make([]DayAvailabilityDetail, 0, len(dbDetail)),
	}

	for _, detail := range dbDetail {
		// Parse HH:MM:SS string from DB into HH:MM
		startTimeHM, _, _ := strings.Cut(detail.StartTime, ":")     // Get HH
		startTimeMM, _, _ := strings.Cut(detail.StartTime[3:], ":") // Get MM
		endTimeHM, _, _ := strings.Cut(detail.EndTime, ":")         // Get HH
		endTimeMM, _, _ := strings.Cut(detail.EndTime[3:], ":")     // Get MM

		availability.Days = append(availability.Days, DayAvailabilityDetail{
			Day:         detail.Day,
			StartTime:   fmt.Sprintf("%s:%s", startTimeHM, startTimeMM), // Format HH:MM
			EndTime:     fmt.Sprintf("%s:%s", endTimeHM, endTimeMM),     // Format HH:MM
			IsAvailable: detail.IsAvailable,
		})
	}

	response := map[string]any{
		"message":      "Fetched availability successfully",
		"availability": availability,
	}
	helper.ResponseJson(w, http.StatusOK, response)
}

// PUT /me/availability
func (a *Controller) UpdateAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	dto, ok := validator.GetValidatedDTOFromContext[dto.UpdateAvailabilityDto](ctx)
	if !ok {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Validated DTO not found in context", nil))
		return
	}

	tx, err := a.db.BeginTxx(ctx, nil)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to start transaction", err))
		return
	}
	// Ensure rollback on error
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			// An error occurred, rollback
			// Log rollback error if needed: tx.Rollback()
			_ = tx.Rollback()
		} else {
			// Success, commit
			err = tx.Commit()
			if err != nil {
				err = appError.NewAppError(enum.InternalServerError, "Failed to commit transaction", err)
			}
		}
	}()

	// 1. Find Availability ID for the user
	var availabilityID string
	err = tx.GetContext(ctx, &availabilityID,
		"SELECT id FROM availability WHERE user_id = $1",
		userID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Availability doesn't exist, create it first
			err = tx.GetContext(ctx, &availabilityID, "INSERT INTO availability (user_id, time_gap) VALUES ($1, $2) RETURNING id", userID, dto.TimeGap)
			if err != nil {
				appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to create availability record", err))
				return
			}
			// If creation succeeds, proceed without updating timeGap again below
		} else {
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to find availability record", err))
			return
		}
	} else {
		// Availability exists, Update timeGap
		_, err = tx.ExecContext(ctx,
			"UPDATE availability SET time_gap = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
			dto.TimeGap,
			availabilityID,
		)
		if err != nil {
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to update time gap", err))
			return
		}
	}

	// 2. Delete old DayAvailability records
	_, err = tx.ExecContext(ctx,
		"DELETE FROM day_availability WHERE availability_id = $1",
		availabilityID,
	)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to delete old availability days", err))
		return
	}

	// 3. Prepare and Insert new DayAvailability records
	if len(dto.Days) > 0 {
		insertQuery := `
			INSERT INTO day_availability (availability_id, day, start_time, end_time, is_available)
			VALUES (:availability_id, :day, :start_time, :end_time, :is_available)
		`
		dayInserts := make([]map[string]interface{}, len(dto.Days))
		for i, dayDto := range dto.Days {
			// Validate HH:MM format before saving (optional but good)
			_, errST := time.Parse(layoutHM, dayDto.StartTime)
			_, errET := time.Parse(layoutHM, dayDto.EndTime)
			if errST != nil || errET != nil {
				err = fmt.Errorf(
					"invalid time format for day %s: start='%s', end='%s'",
					dayDto.Day, dayDto.StartTime, dayDto.EndTime,
				)
				appError.WriteError(w, appError.NewAppError(enum.ValidationError, err.Error(), nil))
				return
			}

			dayInserts[i] = map[string]any{
				"availability_id": availabilityID,
				"day":             dayDto.Day,
				"start_time":      dayDto.StartTime, // Store as HH:MM directly if DB type is TIME or VARCHAR
				"end_time":        dayDto.EndTime,
				"is_available":    dayDto.IsAvailable,
			}
		}

		_, err = tx.NamedExecContext(ctx, insertQuery, dayInserts)
		if err != nil {
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to insert new availability days", err))
			return
		}
	}

	helper.ResponseJson(w, http.StatusOK, helper.SimpleMessage{Message: "Availability updated successfully"})
}

// GET /public/events/{eventId}/availability
func (a *Controller) GetPublicEventAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing eventId in path", nil))
		return
	}

	// Optional: Add UUID validation if service doesn't handle format errors well

	// 1. Fetch Event, User, Availability, and Day rules
	var dbResult []struct {
		model.Event
		AvailabilityID sql.NullString `db:"availability_id"` // From event table
		TimeGap        sql.NullInt64  `db:"time_gap"`
		Day            enum.DayOfWeek `db:"day"`          // From day_availability
		StartTime      string         `db:"start_time"`   // TIME as TEXT
		EndTime        string         `db:"end_time"`     // TIME as TEXT
		IsAvailable    bool           `db:"is_available"` // From day_availability
	}

	query := `
		SELECT
			e.*, -- Select all from event
			a.id as availability_id,
			a.time_gap,
			d.day,
			d.start_time::TEXT,
			d.end_time::TEXT,
			d.is_available
		FROM events e
		JOIN users u ON e.user_id = u.id
		LEFT JOIN availability a ON u.id = a.user_id  -- Use LEFT JOIN for availability
		LEFT JOIN day_availability d ON a.id = d.availability_id -- LEFT JOIN for days
		WHERE e.id = $1 AND e.is_private = FALSE;
	`

	err := a.db.SelectContext(ctx, &dbResult, query, eventID)
	if err != nil {
		if err == sql.ErrNoRows {
			appError.WriteError(w, appError.NewNotFoundError("Public event", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to fetch event and availabilty", err))
		return
	}

	if len(dbResult) == 0 || !dbResult[0].AvailabilityID.Valid {
		// Event found, but no availability configured for the user
		// Return empty list as per TS logic?
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Event found but no availability for user", nil))
		return
	}

	event := dbResult[0].Event
	timeGap := int(dbResult[0].TimeGap.Int64)
	userID := event.UserID

	// Organize day rules fetched from DB
	dayRules := make(map[enum.DayOfWeek]AvailabilityDetail)

	for _, row := range dbResult {
		// Only process if day availability details are present
		if row.AvailabilityID.Valid && row.Day != "" {
			dayRules[row.Day] = AvailabilityDetail{
				TimeGap:     timeGap, // Use consistent timeGap
				Day:         row.Day,
				StartTime:   row.StartTime,
				EndTime:     row.EndTime,
				IsAvailable: row.IsAvailable,
			}
		}
	}

	// 2. Calculate dates for the next 7 days (or desired range)
	datesToCheck := make(map[enum.DayOfWeek]time.Time)
	dateRangeStart := time.Now()
	dateRangeEnd := dateRangeStart.AddDate(0, 0, 7)

	var errDate error
	for _, day := range enum.AllDayOfWeek() {
		datesToCheck[day], errDate = GetNextDateForDay(day)
		if errDate != nil {
			// Log this error
			log.Printf("Warning: Could not calculate next date for %s: %v\n", day, errDate)
			// Decide whether to skip or return error. Skipping for now.
		}
	}

	// 3. Fetch meetings for the relevant user within the date range ONCE
	var meetingsInRange []model.Meeting
	meetingsQuery := `
	    SELECT id, user_id, event_id, guest_name, guest_email, additional_info, start_time, end_time, meet_link, calendar_event_id, calendar_app_type, status, created_at, updated_at
        FROM meetings
        WHERE user_id = $1 AND start_time < $2 AND end_time > $3
	`

	err = a.db.SelectContext(ctx, &meetingsInRange, meetingsQuery, userID, dateRangeEnd, dateRangeStart)
	if err != nil && err != sql.ErrNoRows {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to fetch meetings", err))
		return
	}

	// 4. Generate slots for each day
	resultSlots := make([]DailyAvailabilitySlots, 0, len(enum.AllDayOfWeek()))
	for _, dayOfWeek := range enum.AllDayOfWeek() {
		targetDate, dateOk := datesToCheck[dayOfWeek]
		if !dateOk {
			continue
		}

		rule, ruleExists := dayRules[dayOfWeek]

		dailyResult := DailyAvailabilitySlots{
			Day:         dayOfWeek,
			Slots:       []string{},
			IsAvailable: ruleExists && rule.IsAvailable,
		}

		if dailyResult.IsAvailable {
			// Filter meetings spesifically for this targetDate
			meetingsForThisDate := make([]model.Meeting, 0)
			dayStart := targetDate
			dayEnd := targetDate.AddDate(0, 0, 1)

			for _, m := range meetingsInRange {
				if m.StartTime.Before(dayEnd) && m.EndTime.After(dayStart) {
					meetingsForThisDate = append(meetingsForThisDate, m)
				}
			}

			slots, errSlots := GenerateAvailableTimeSlots(
				rule.StartTime,
				rule.EndTime,
				int(event.Duration),
				timeGap,
				meetingsForThisDate,
				targetDate,
			)
			if errSlots != nil {
				// Log error but potentially continue for other days
				log.Printf("Error generating slots for %s on %s: %v\n", dayOfWeek, targetDate.Format(layoutDate), errSlots)
			} else {
				dailyResult.Slots = slots
			}
		}
		resultSlots = append(resultSlots, dailyResult)
	}

	response := map[string]any{
		"message": "Event availability fetched successfully",
		"data":    resultSlots,
	}
	helper.ResponseJson(w, http.StatusOK, response)
}
