package httpStatus

import "net/http"

// HttpStatusCode represents an HTTP status code using a custom type.
// Note: Using the built-in int and net/http constants is generally preferred.
type HttpStatusCode int

// Define common HTTP status codes as constants of the custom type.
const (
	// --- Success Status ---
	OK        HttpStatusCode = 200
	Created   HttpStatusCode = 201
	Accepted  HttpStatusCode = 202
	NoContent HttpStatusCode = 204

	// --- Client Error Responses ---
	BadRequest          HttpStatusCode = 400
	Unauthorized        HttpStatusCode = 401
	Forbidden           HttpStatusCode = 403
	NotFound            HttpStatusCode = 404
	MethodNotAllowed    HttpStatusCode = 405
	Conflict            HttpStatusCode = 409
	UnprocessableEntity HttpStatusCode = 422
	TooManyRequests     HttpStatusCode = 429

	// --- Server Error Responses ---
	InternalServerError HttpStatusCode = 500
	NotImplemented      HttpStatusCode = 501
	BadGateway          HttpStatusCode = 502
	ServiceUnavailable  HttpStatusCode = 503
	GatewayTimeout      HttpStatusCode = 504
)

// String returns the standard HTTP reason phrase for the status code.
// This implements the fmt.Stringer interface.
func (sc HttpStatusCode) String() string {
	// Delegate to the standard library's StatusText function
	return http.StatusText(int(sc))
}

// Int returns the underlying integer value of the status code.
func (sc HttpStatusCode) Int() int {
	return int(sc)
}

// --- Optional helper methods ---

// IsSuccess checks if the status code represents success (2xx).
func (sc HttpStatusCode) IsSuccess() bool {
	return sc >= 200 && sc < 300
}

// IsClientError checks if the status code represents a client error (4xx).
func (sc HttpStatusCode) IsClientError() bool {
	return sc >= 400 && sc < 500
}

// IsServerError checks if the status code represents a server error (5xx).
func (sc HttpStatusCode) IsServerError() bool {
	return sc >= 500 && sc < 600
}
