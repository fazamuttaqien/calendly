package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/fazamuttaqien/calendly/helper"
	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
)

// ErrorMiddleware provides a centralized error handling mechanism.
// It recovers from panics, logs them, and writes JSON error responses.
func ErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Log the panic and stack trace for debugging
				log.Printf("Panic Recovered: %v\n%s", rec, debug.Stack())

				// Attempt to convert the recovered value to an error
				var err error
				switch typedRec := rec.(type) {
				case string:
					err = errors.New(typedRec)
				case error:
					err = typedRec
				default:
					err = fmt.Errorf("unknown panic type: %v", typedRec)
				}

				// Now handle the error like in your original logic
				var appErr *appError.AppError
				var statusCode int
				var message string
				var details interface{} // For validation errors

				if errors.As(err, &appErr) {
					statusCode = appErr.HTTPStatus()
					message = appErr.Error()
					details = appErr.GetErrorDetail()

					// Log the internal error details if they exist
					if internalErr := appErr.Unwrap(); internalErr != nil {
						log.Fatalf("AppError Internal Cause: %v", internalErr)
					} else {
						// Log the AppError itself if no inner cause
						log.Fatalf("AppError: Code=%s, Message=%s", appErr.Code, appErr.Error())
					}

				} else {
					// Handle non-AppError types (unexpected panics/errors)
					statusCode = http.StatusInternalServerError
					message = "An unexpected internal error occurred."

					// Log the original non-AppError
					log.Fatalf("Unhandled Internal Error: %v", err)

				}

				// Write the JSON error response
				helper.ResponseErrorJson(w, statusCode, message, details)
			}
		}()

		// Call the next handler in the chain. If it panics, the defer will catch it.
		next.ServeHTTP(w, r)
	})
}
