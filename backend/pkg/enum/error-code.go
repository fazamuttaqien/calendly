package enum

// ErrorCode represents specific error identifiers used throughout the application.
// It's based on the underlying type string.
type ErrorCode string

// Define the possible constant values for ErrorCode.
const (
	// --- Authentication Errors ---

	// AuthUserNotFound indicates the user was not found during an auth process.
	AuthUserNotFound ErrorCode = "AUTH_USER_NOT_FOUND"
	// AuthEmailAlreadyExists indicates an attempt to register with an existing email.
	AuthEmailAlreadyExists ErrorCode = "AUTH_EMAIL_ALREADY_EXISTS"
	// AuthInvalidToken indicates a provided token (JWT, session, etc.) is invalid or expired.
	AuthInvalidToken ErrorCode = "AUTH_INVALID_TOKEN"
	// AuthNotFound generic authentication details not found.
	AuthNotFound ErrorCode = "AUTH_NOT_FOUND"
	// AuthTooManyAttempts indicates too many failed login or similar attempts.
	AuthTooManyAttempts ErrorCode = "AUTH_TOO_MANY_ATTEMPTS"
	// AuthUnauthorizedAccess indicates missing or insufficient credentials for an action.
	AuthUnauthorizedAccess ErrorCode = "AUTH_UNAUTHORIZED_ACCESS"
	// AuthTokenNotFound indicates that an expected authentication token was not provided.
	AuthTokenNotFound ErrorCode = "AUTH_TOKEN_NOT_FOUND"

	// --- Access Control Errors ---

	// AccessUnauthorized indicates the authenticated user lacks permission for the resource/action.
	AccessUnauthorized ErrorCode = "ACCESS_UNAUTHORIZED"

	// --- Validation and Resource Errors ---

	// ValidationError indicates input data failed validation rules.
	ValidationError ErrorCode = "VALIDATION_ERROR"
	// ResourceNotFound indicates a requested resource (e.g., via ID) does not exist.
	ResourceNotFound ErrorCode = "RESOURCE_NOT_FOUND"

	// --- System Errors ---

	// InternalServerError indicates an unexpected error occurred on the server.
	InternalServerError ErrorCode = "INTERNAL_SERVER_ERROR"

	BadRequest ErrorCode = "BAD_REQUEST"
)

// --- Optional Helpers ---

// AllErrorCodes returns a slice containing all possible ErrorCode values.
func AllErrorCodes() []ErrorCode {
	return []ErrorCode{
		AuthUserNotFound,
		AuthEmailAlreadyExists,
		AuthInvalidToken,
		AuthNotFound,
		AuthTooManyAttempts,
		AuthUnauthorizedAccess,
		AuthTokenNotFound,
		AccessUnauthorized,
		ValidationError,
		ResourceNotFound,
		InternalServerError,
	}
}

// IsValid checks if the ErrorCode value is one of the predefined constants.
func (ec ErrorCode) IsValid() bool {
	switch ec {
	case AuthUserNotFound,
		AuthEmailAlreadyExists,
		AuthInvalidToken,
		AuthNotFound,
		AuthTooManyAttempts,
		AuthUnauthorizedAccess,
		AuthTokenNotFound,
		AccessUnauthorized,
		ValidationError,
		ResourceNotFound,
		InternalServerError:
		return true
	default:
		return false
	}
}

// String returns the string representation of the ErrorCode.
// This method satisfies the fmt.Stringer interface.
func (ec ErrorCode) String() string {
	return string(ec)
}

// You might also add methods related to specific error types, e.g.,
// func (ec ErrorCode) IsAuthError() bool { ... }
// func (ec ErrorCode) IsClientError() bool { ... } // For 4xx type errors
// func (ec ErrorCode) IsServerError() bool { ... } // For 5xx type errors
