package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/fazamuttaqien/calendly/pkg/enum"
	pkgValidator "github.com/fazamuttaqien/calendly/pkg/validator"
	"github.com/fazamuttaqien/calendly/types"

	"github.com/go-playground/validator/v10"
)

// WithValidation creates middleware to validate request data against a struct (DTO).
// T is the type of the struct to validate against.
// source indicates where to find the data ("body", "query", "params").
func WithValidation[T any](source string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a zero instance of the target struct type T
			var dto T // dto := new(T) works too but T is often cleaner

			var err error
			switch source {
			case pkgValidator.SourceBody:
				// Decode JSON body
				if r.Body == nil {
					err = fmt.Errorf("request body is empty")
				} else {
					decoder := json.NewDecoder(r.Body)
					// decoder.DisallowUnknownFields() // Optional: Be strict about fields
					if decodeErr := decoder.Decode(&dto); decodeErr != nil {
						err = fmt.Errorf("failed to decode request body: %w", decodeErr)
					}
					// NOTE: Consider closing r.Body if necessary, though Decode might handle it.
				}
				if err != nil {
					// Use your error handling mechanism, here we write response directly
					pkgValidator.WriteValidationErrorResponse(w, http.StatusBadRequest, enum.ValidationError, "Invalid request body.", nil)
					return
				}

			case pkgValidator.SourceQuery:
				// TODO: Implement query parameter parsing and mapping to dto.
				// This is more complex. Libraries like 'gorilla/schema' can help.
				// For now, return an error indicating it's not implemented.
				pkgValidator.WriteValidationErrorResponse(w, http.StatusNotImplemented, enum.InternalServerError, "Query parameter validation not implemented.", nil)
				return

			case pkgValidator.SourceParams:
				// TODO: Implement path parameter parsing (requires router integration)
				// and mapping to dto fields. This often involves manual mapping.
				// Example (using chi): chi.URLParam(r, "id")
				pkgValidator.WriteValidationErrorResponse(w, http.StatusNotImplemented, enum.InternalServerError, "Path parameter validation not implemented.", nil)
				return

			default:
				// Invalid source configuration
				pkgValidator.WriteValidationErrorResponse(w, http.StatusInternalServerError, enum.InternalServerError, "Internal server error: Invalid validation source.", nil)
				return
			}

			// Perform validation
			validationErr := pkgValidator.Validate.Struct(dto)
			if validationErr != nil {
				// Check if it's validation errors
				var ve validator.ValidationErrors
				if ok := errors.As(validationErr, &ve); ok {
					// Format errors and send response
					formattedErrors := pkgValidator.FormatValidationErrors(ve)
					pkgValidator.WriteValidationErrorResponse(w, http.StatusBadRequest, enum.ValidationError, "Validation failed", formattedErrors)
					return
				} else {
					// Handle unexpected error during validation itself
					pkgValidator.WriteValidationErrorResponse(w, http.StatusInternalServerError, enum.InternalServerError, "Error during validation process.", nil)
					log.Fatalf("Unexpected validation error: %v\n", validationErr)
					return
				}
			}

			// Validation successful! Store the validated DTO in the context.
			ctx := context.WithValue(r.Context(), types.ValidatedDTOKey, dto)

			// Call the next handler with the updated context.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
