package enum

import "strings"

// --- DayOfWeek ---
type DayOfWeek string

const (
	Sunday    DayOfWeek = "SUNDAY"
	Monday    DayOfWeek = "MONDAY"
	Tuesday   DayOfWeek = "TUESDAY"
	Wednesday DayOfWeek = "WEDNESDAY"
	Thursday  DayOfWeek = "THURSDAY"
	Friday    DayOfWeek = "FRIDAY"
	Saturday  DayOfWeek = "SATURDAY"
)

func AllDayOfWeek() []DayOfWeek {
	return []DayOfWeek{
		Sunday,
		Monday,
		Tuesday,
		Wednesday,
		Thursday,
		Friday,
		Saturday,
	}
}

func (e DayOfWeek) String() string { return string(e) }
func DayOfWeekValues() []string {
	vals := AllDayOfWeek()
	strs := make([]string, len(vals))

	for i, v := range vals {
		strs[i] = v.String()
	}

	return strs
}

// --- IntegrationProvider ---
type IntegrationProvider string

const (
	ProviderGoogle    IntegrationProvider = "GOOGLE"
	ProviderZoom      IntegrationProvider = "ZOOM"
	ProviderMicrosoft IntegrationProvider = "MICROSOFT"
)

func AllIntegrationProvider() []IntegrationProvider {
	return []IntegrationProvider{
		ProviderGoogle,
		ProviderZoom,
		ProviderMicrosoft,
	}
}

func (e IntegrationProvider) String() string { return string(e) }

func IntegrationProviderValues() []string {
	vals := AllIntegrationProvider()
	strs := make([]string, len(vals))

	for i, v := range vals {
		strs[i] = v.String()
	}

	return strs
}

// --- IntegrationAppType ---
type IntegrationAppType string

const (
	AppGoogleMeetAndCalendar IntegrationAppType = "GOOGLE_MEET_AND_CALENDAR"
	AppZoomMeeting           IntegrationAppType = "ZOOM_MEETING"
	AppOutlookCalendar       IntegrationAppType = "OUTLOOK_CALENDAR"
)

func AllIntegrationAppType() []IntegrationAppType {
	return []IntegrationAppType{
		AppGoogleMeetAndCalendar,
		AppZoomMeeting,
		AppOutlookCalendar,
	}
}

func (e IntegrationAppType) String() string { return string(e) }

func IntegrationAppTypeValues() []string {
	vals := AllIntegrationAppType()
	strs := make([]string, len(vals))

	for i, v := range vals {
		strs[i] = v.String()
	}

	return strs
}

// --- IntegrationCategory ---
type IntegrationCategory string

const (
	CategoryCalendarAndVideo IntegrationCategory = "CALENDAR_AND_VIDEO_CONFERENCING"
	CategoryVideo            IntegrationCategory = "VIDEO_CONFERENCING"
	CategoryCalendar         IntegrationCategory = "CALENDAR"
)

func AllIntegrationCategory() []IntegrationCategory {
	return []IntegrationCategory{
		CategoryCalendarAndVideo,
		CategoryVideo,
		CategoryCalendar,
	}
}
func (e IntegrationCategory) String() string { return string(e) }

func IntegrationCategoryValues() []string {
	vals := AllIntegrationCategory()
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = v.String()
	}
	return strs
}

// --- EventLocationType --- (Uses IntegrationAppType values)
type EventLocationType IntegrationAppType

const (
	LocationGoogleMeetAndCalendar EventLocationType = EventLocationType(AppGoogleMeetAndCalendar)
	LocationZoomMeeting           EventLocationType = EventLocationType(AppZoomMeeting)
	// Note: Outlook not included in the TS definition for EventLocationEnumType
)

func AllEventLocationType() []EventLocationType {
	return []EventLocationType{LocationGoogleMeetAndCalendar, LocationZoomMeeting}
}
func (e EventLocationType) String() string { return string(e) }
func EventLocationTypeValues() []string {
	vals := AllEventLocationType()
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = v.String()
	}
	return strs
}

// --- MeetingStatus ---
type MeetingStatus string

const (
	Scheduled MeetingStatus = "SCHEDULED"
	Cancelled MeetingStatus = "CANCELLED"
)

func AllMeetingStatus() []MeetingStatus {
	return []MeetingStatus{
		Scheduled,
		Cancelled,
	}
}

func (e MeetingStatus) String() string { return string(e) }
func MeetingStatusValues() []string {
	vals := AllDayOfWeek()
	strs := make([]string, len(vals))

	for i, v := range vals {
		strs[i] = v.String()
	}

	return strs
}

// MeetingFilter represents the type for meeting filter statuses.
// It's based on the underlying type string.
type MeetingFilter string

// Define the possible constant values for MeetingFilter.
const (
	// MeetingFilterUpcoming represents upcoming meetings.
	MeetingFilterUpcoming MeetingFilter = "UPCOMING"

	// MeetingFilterPast represents past meetings.
	MeetingFilterPast MeetingFilter = "PAST"

	// MeetingFilterCancelled represents cancelled meetings.
	MeetingFilterCancelled MeetingFilter = "CANCELLED"
)

// --- Optional Helpers ---

// AllMeetingFilters returns a slice containing all possible MeetingFilter values.
// Useful for validation or populating UI elements.
func AllMeetingFilters() []MeetingFilter {
	return []MeetingFilter{
		MeetingFilterUpcoming,
		MeetingFilterPast,
		MeetingFilterCancelled,
	}
}

// IsValid checks if the MeetingFilter value is one of the predefined constants.
func (mf MeetingFilter) IsValid() bool {
	switch mf {
	case MeetingFilterUpcoming, MeetingFilterPast, MeetingFilterCancelled:
		return true
	default:
		return false
	}
}

// String returns the string representation of the MeetingFilter.
// This method satisfies the fmt.Stringer interface.
func (mf MeetingFilter) String() string {
	return string(mf)
}

// --- Helper to join enum values for 'oneof' tag ---
// (Could be generated or put in a utility package)
func joinEnumValues(values []string) string {
	return strings.Join(values, " ")
}
