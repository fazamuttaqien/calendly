package appError

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/fazamuttaqien/calendly/helper"
	"github.com/fazamuttaqien/calendly/pkg/enum"
)

// ErrorDetail holds configuration details for a specific application ErrorCode.
type ErrorDetail struct {
	HTTPStatus int
	Message    string
}

// appErrorConfig maps application Errorenums to their details.
// It's kept private to the package and accessed via functions.
var appErrorConfig map[enum.ErrorCode]ErrorDetail

func init() {
	// Initialize the error configuration map
	appErrorConfig = map[enum.ErrorCode]ErrorDetail{
		// --- Authentication Errors ---
		enum.AuthUserNotFound: {
			HTTPStatus: http.StatusNotFound,
			Message:    "Authentication failed: User not found",
		},
		enum.AuthEmailAlreadyExists: {
			HTTPStatus: http.StatusConflict, // 409
			Message:    "Cannot register: Email address is already in use.",
		},
		enum.AuthInvalidToken: {
			HTTPStatus: http.StatusUnauthorized, // 401
			Message:    "Authentication failed: Invalid or expired token.",
		},
		enum.AuthNotFound: {
			HTTPStatus: http.StatusNotFound, // 404
			Message:    "Authentication details not found.",
		},
		enum.AuthTooManyAttempts: {
			HTTPStatus: http.StatusTooManyRequests, // 429
			Message:    "Too many authentication attempts. Please try again later.",
		},
		enum.AuthUnauthorizedAccess: {
			HTTPStatus: http.StatusUnauthorized, // 401
			Message:    "Authentication required to access this resource.",
		},
		enum.AuthTokenNotFound: {
			HTTPStatus: http.StatusUnauthorized, // 401
			Message:    "Authentication token not provided.",
		},

		// --- Access Control Errors ---
		enum.AccessUnauthorized: {
			HTTPStatus: http.StatusForbidden, // 403
			Message:    "Access denied: You do not have permission to perform this action.",
		},

		// --- Validation and Resource Errors ---
		enum.ValidationError: {
			HTTPStatus: http.StatusBadRequest, // 400 (Often, but could be 422)
			// Message: "Input validation failed. Please check the provided data.", // Can add more specific details later
			Message: "Input validation failed.",
		},
		enum.ResourceNotFound: {
			HTTPStatus: http.StatusNotFound, // 404
			Message:    "The requested resource could not be found.",
		},

		// --- System Errors ---
		enum.InternalServerError: {
			HTTPStatus: http.StatusInternalServerError, // 500
			Message:    "An unexpected internal error occurred. Please try again later.",
		},

		// Add mappings for any other ErrorCode constants...
	}
}

// AppError is a custom error type for application-specific errors.
// It implements the standard `error` interface.
type AppError struct {
	Code    enum.ErrorCode // The specific application error code
	Message string         // Specific message for this *instance* of the error (can override default)
	Err     error          // Optional: The underlying wrapped error (for context)
}

// NewAppError creates a new application error.
// If msg is empty, the default message for the code will be used when Error() is called.
// cause is the underlying error, if any (can be nil).
func NewAppError(code enum.ErrorCode, msg string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
		Err:     cause,
	}
}

// GetErrorDetail retrieves the configured details for a given ErrorCode.
// It returns a default InternalServerError detail if the code is not found.
func (e *AppError) GetErrorDetail() ErrorDetail {
	detail, ok := appErrorConfig[e.Code]
	if !ok {
		// Fallback for any unmapped error codes
		return ErrorDetail{
			HTTPStatus: http.StatusInternalServerError,
			Message:    "An unknown internal error occurred.",
		}
	}
	return detail
}

// Error implements the standard error interface.
// It provides a user-friendly message, falling back to the default if necessary.
func (e *AppError) Error() string {
	// Prefer the specific message if provided for this instance
	if e.Message != "" {
		return e.Message
	}
	// Otherwise, use the default message from the configuration
	detail := e.GetErrorDetail()
	return detail.Message
}

// HTTPStatus returns the appropriate HTTP status code for this error.
func (e *AppError) HTTPStatus() int {
	return e.GetErrorDetail().HTTPStatus
}

// Unwrap allows retrieving the underlying error (for use with errors.Is/As).
// Requires Go 1.13+
func (e *AppError) Unwrap() error {
	return e.Err
}

// --- Helper Functions for Common Errors ---

func NewNotFoundError(resource string, cause error) *AppError {
	msg := fmt.Sprintf("Resource '%s' not found.", resource)
	// If no specific resource, the default message from config will be used by Error()
	if resource == "" {
		msg = ""
	}
	return NewAppError(enum.ResourceNotFound, msg, cause)
}

func NewValidationError(specificMessage string, cause error) *AppError {
	// If specificMessage is empty, the default validation error message will be used
	return NewAppError(enum.ValidationError, specificMessage, cause)
}

func NewUnauthorizedError(cause error) *AppError {
	// Typically uses the default message
	return NewAppError(enum.AccessUnauthorized, "", cause)
}

// ... add more helpers as needed ...

/*
// WriteError checks if the error is an AppError and writes the appropriate response.
// Falls back to a generic 500 error if it's not an AppError.
// NOTE: This is useful if handlers *don't* panic but return errors directly,
// OR if middleware needs to write an error *before* panicking/calling next.
*/
func WriteError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		helper.ResponseErrorJson(w, appErr.HTTPStatus(), appErr.Error(), appErr.GetErrorDetail())
		// Log internal details
		if internalErr := appErr.Unwrap(); internalErr != nil {
			log.Fatalf("AppError Internal Cause: %v", internalErr)
		} else {
			log.Fatalf("AppError: Code=%s, Message=%s", appErr.Code, appErr.Error())
		}
	} else {
		// Generic internal error
		log.Fatalf("Unhandled Internal Error: %v", err)
		helper.ResponseErrorJson(w, http.StatusInternalServerError, "An unexpected internal error occurred.", nil)
	}
}
