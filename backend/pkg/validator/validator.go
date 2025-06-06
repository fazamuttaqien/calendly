package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/fazamuttaqien/calendly/pkg/enum"
	"github.com/fazamuttaqien/calendly/types"
	"github.com/go-playground/validator/v10"
)

const (
	SourceBody   = "body"
	SourceQuery  = "query"
	SourceParams = "params"
)

// ValidationErrorDetail describes a single validation failure.
type ValidationErrorDetail struct {
	Field   string `json:"field"`   // Field name that failed validation
	Message any    `json:"message"` // Validation error message(s) (can be map or string)
}

// ValidationErrorResponse is the structured JSON response for validation errors.
type ValidationErrorResponse struct {
	Message   string                  `json:"message"`   //	General message
	ErrorCode enum.ErrorCode          `json:"errorCode"` //	Specific error code
	Errors    []ValidationErrorDetail `json:"errors"`    // List of specific field errors
}

// Validator instance (create once for efficiency)
var Validate *validator.Validate

func init() {
	Validate = validator.New()
	// Optional: Register custom validation functions if needed
	// validate.RegisterValidation(...)

	// Optional: Customize how field names are reported (e.g., use json tags)
	Validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "" {
			return ""
		}
		return name
	})
}

// FormatValidationErrors translates validator errors into the desired response structure.
func FormatValidationErrors(ve validator.ValidationErrors) []ValidationErrorDetail {
	out := make([]ValidationErrorDetail, len(ve))
	for i, fe := range ve {
		out[i] = ValidationErrorDetail{
			Field:   fe.Field(), // Use Field() which might respect RegisterTagNameFunc
			Message: ValidationMessageForTag(fe),
		}
	}
	return out
}

// ValidationMessageForTag provides a basic error message for a validation tag.
// You can make this much more sophisticated (e.g., using translations).
func ValidationMessageForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("Value must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("Value must not exceed %s", fe.Param())
	// Add more cases for common tags like 'len', 'uuid', 'url', etc.
	default:
		return fmt.Sprintf("Invalid value (validation: %s)", fe.Tag()) // Fallback message
	}
}

// GetValidatedDTO retrieves the validated DTO from context, performing type assertion.
func GetValidatedDTO[T any](ctx context.Context) (T, error) {
	dto, ok := ctx.Value(types.ValidatedDTOKey).(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("could not retrieve validated DTO from context or type mismatch")
	}
	return dto, nil
}

// GetValidatedDTOFromContext retrieves the validated DTO stored by validation middleware.
// Returns zero value of T and false if not found or type mismatch.
func GetValidatedDTOFromContext[T any](ctx context.Context) (T, bool) {
	dto, ok := ctx.Value(types.ValidatedDTOKey).(T)
	return dto, ok
}

// WriteValidationErrorResponse is a helper to write structured JSON error responses.
func WriteValidationErrorResponse(
	w http.ResponseWriter,
	code int, errorCode enum.ErrorCode,
	message string, detail []ValidationErrorDetail,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := ValidationErrorResponse{
		Message:   message,
		ErrorCode: errorCode,
		Errors:    detail,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Fatalf("Error encoding validation response: %v\n", err)
	}
}
