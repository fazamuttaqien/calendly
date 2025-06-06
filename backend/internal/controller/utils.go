package controller

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fazamuttaqien/calendly/internal/model"
	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
	"github.com/fazamuttaqien/calendly/pkg/enum"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	// Layout for HH:MM format
	layoutHM = "15:04"
	// Layout for YYYY-MM-DD format
	layoutDate = "2006-01-02"
	// Layout for DB TIME format (adjust if different)
	layoutDBTime = "15:04:05"
)

// GetNextDateForDay calculates the date of the next occurrence of a given weekday.
func GetNextDateForDay(dayOfWeek enum.DayOfWeek) (time.Time, error) {
	days := map[enum.DayOfWeek]time.Weekday{
		enum.Sunday:    time.Sunday,
		enum.Monday:    time.Monday,
		enum.Tuesday:   time.Tuesday,
		enum.Wednesday: time.Wednesday,
		enum.Thursday:  time.Thursday,
		enum.Friday:    time.Friday,
		enum.Saturday:  time.Saturday,
	}

	targetWeekDay, ok := days[dayOfWeek]
	if !ok {
		return time.Time{}, fmt.Errorf("invalid day of week: %s", dayOfWeek)
	}

	today := time.Now()
	todayWeekDay := today.Weekday()

	daysUntilTarget := int(targetWeekDay-todayWeekDay+7) % 7

	// If today is the target day, calculate slots for today unless daysUntilTarget is explicitly 0
	// This logic needs refinement based on whether "next date" includes today
	// Let's assume getNextDateForDay means "today or the next occurrence"

	// if daysUntilTarget == 0 && targetWeekday != todayWeekday {
	// 	daysUntilTarget = 7 // If target is same day next week
	// }

	// Simpler: always calculate from today
	nextDate := today.AddDate(0, 0, daysUntilTarget)
	// Return date part only (midnight)
	year, month, day := nextDate.Date()

	return time.Date(year, month, day, 0, 0, 0, 0, today.Location()), nil
}

// GenerateAvailableTimeSlots creates HH:MM slots based on availability, duration, and existing meetings.
func GenerateAvailableTimeSlots(dayStartTimeStr, dayEndTimeStr string, durationMinutes, timeGapMinutes int, meetingsOnDate []model.Meeting, targetDate time.Time,
) ([]string, error) {

	// Parse the start/end times from DB format
	dayStartParsed, err := time.Parse(layoutDBTime, dayStartTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start time format '%s': %w", dayStartTimeStr, err)
	}

	dayEndParsed, err := time.Parse(layoutDBTime, dayEndTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end time format '%s': %w", dayEndTimeStr, err)
	}

	// Combine targetDate with parsed times
	location := targetDate.Location() // Use location of targetDate

	slotStartBase := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(),
		dayStartParsed.Hour(), dayEndParsed.Minute(), 0, 0, location)

	dayEndBoundary := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(),
		dayEndParsed.Hour(), dayEndParsed.Minute(), 0, 0, location)

	slots := []string{}
	now := time.Now().In(location)

	currentSlotStart := slotStartBase

	if timeGapMinutes <= 0 {
		timeGapMinutes = 30
	}

	for currentSlotStart.Before(dayEndBoundary) {
		slotEnd := currentSlotStart.Add(time.Minute * time.Duration(durationMinutes))

		// Ensure slot doesn't exceed the day's end boundary
		if slotEnd.After(dayEndBoundary) {
			break
		}

		// Check if slot is in the future (relative to 'now')
		isFutureSlot := currentSlotStart.After(now) || currentSlotStart.Equal(now) // Allow slot starting exactly now

		if isFutureSlot && IsSlotAvailable(currentSlotStart, slotEnd, meetingsOnDate) {
			slots = append(slots, currentSlotStart.Format(layoutHM))
		}

		// Move to the next potential slot start time
		currentSlotStart = currentSlotStart.Add(time.Minute * time.Duration(timeGapMinutes))
	}

	return slots, nil
}

// GetCalendarClient helper initializes the Google Calendar client, handling token refresh.
func GetCalendarClient(ctx context.Context, integration model.Integration) (*calendar.Service, enum.IntegrationAppType, error) {
	appType := integration.AppType // Get app type from the integration model

	switch appType {
	case enum.AppGoogleMeetAndCalendar:
		if !integration.RefreshToken.Valid || integration.RefreshToken.String == "" {
			// Handle case where refresh token is missing but required
			return nil, appType, appError.NewAppError(enum.AuthUnauthorizedAccess, "Google integration missing refresh token for offline access.", nil)
		}

		// Use the IntegrationService's token validation logic
		// This assumes ValidateGoogleToken handles refresh and returns a valid access token
		// AND that the IntegrationService *persists* the refreshed token if necessary.
		validAccessToken, err := ValidateGoogleToken(
			ctx,
			integration.AccessToken.String,
			integration.RefreshToken.String, // Pass the string value
			integration.ExpiryDate.Int64,    // Pass the int64 value
		)
		if err != nil {
			return nil, appType, appError.NewAppError(enum.AuthInvalidToken, "Failed to validate/refresh Google token", err)
		}

		// Prepare token struct for oauth2 client
		token := &oauth2.Token{
			AccessToken:  validAccessToken,
			RefreshToken: integration.RefreshToken.String, // Include refresh token if present
			// Expiry: time.Unix(integration.ExpiryDate.Int64, 0), // Expiry is handled by TokenSource implicitly
		}
		if integration.ExpiryDate.Valid {
			token.Expiry = time.Unix(integration.ExpiryDate.Int64, 0)
		}

		// Use global oauth config (initialized in integration service or elsewhere)
		httpClient := googleOAuthConfig.Client(ctx, token)

		// Create Calendar service
		calendarSvc, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			return nil, appType, appError.NewAppError(enum.InternalServerError, "Failed to create Google Calendar service client", err)
		}
		return calendarSvc, appType, nil

	// case codes.AppOutlookCalendar:
	// TODO: Implement Outlook Calendar client initialization
	// return nil, appType, apperror.NewAppError(enums.InternalServerError, "Outlook Calendar not implemented", nil)

	default:
		msg := fmt.Sprintf("Unsupported calendar provider app type: %s", appType)
		return nil, appType, appError.NewAppError(enum.BadRequest, msg, nil)
	}
}

// IsSlotAvailable checks if a potential slot conflicts with existing meetings.
func IsSlotAvailable(slotStart, slotEnd time.Time, meetings []model.Meeting) bool {
	for _, meeting := range meetings {
		// Check for overlap: (SlotStart < MeetingEnd) and (SlotEnd > MeetingStart)
		if slotStart.Before(meeting.EndTime) && slotEnd.After(meeting.StartTime) {
			return false // Slot overlaps with a meeting
		}
	}

	return true
}

func IntegrationAppTypeFromEventLocation(loc enum.EventLocationType) (enum.IntegrationAppType, bool) {
	switch loc {
	case enum.LocationGoogleMeetAndCalendar:
		return enum.AppGoogleMeetAndCalendar, true
	case enum.LocationZoomMeeting:
		// Assuming Zoom might relate to a Zoom integration app type
		return enum.AppZoomMeeting, true
	default:
		return "", false
	}
}

// ValidateGoogleToken checks expiry and refreshes if needed using oauth2 package.
func ValidateGoogleToken(ctx context.Context, accessToken, refreshToken string, expiryDateUnix int64) (string, error) {
	// Convert expiryDateUnix (assuming seconds) to time.Time
	expiryTime := time.Unix(expiryDateUnix, 0)

	if refreshToken == "" {
		// If no refresh token, we can't refresh. Check expiry but return current token.
		if time.Now().After(expiryTime) {
			// Optionally return an error indicating expired token and no refresh capability
			// return accessToken, fmt.Errorf("token expired and no refresh token available")
		}
		return accessToken, nil // Cannot refresh
	}

	currentToken := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiryTime,
	}

	// Create a TokenSource with the existing token
	tokenSource := googleOAuthConfig.TokenSource(ctx, currentToken)

	// GetToken will automatically refresh if the token is expired or close to expiry
	newToken, err := tokenSource.Token()
	if err != nil {
		// Handle refresh errors (e.g., invalid grant)
		return "", appError.NewAppError(enum.AuthInvalidToken, "Failed to refresh Google token", err)
	}

	// Check if the token was actually refreshed
	if newToken.AccessToken != accessToken {
		log.Printf("Google token refreshed.")
		// IMPORTANT: You should PERSIST the potentially new newToken.AccessToken,
		// newToken.RefreshToken (if changed), and newToken.Expiry back to your database here!
		// errUpdate := s.UpdateIntegrationToken(...)
		// if errUpdate != nil { ... handle update error ... }
	}

	return newToken.AccessToken, nil
}
