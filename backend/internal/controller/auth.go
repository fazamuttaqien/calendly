package controller

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/fazamuttaqien/calendly/helper"
	"github.com/fazamuttaqien/calendly/internal/dto"
	"github.com/fazamuttaqien/calendly/internal/model"
	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
	"github.com/fazamuttaqien/calendly/pkg/enum"
	pkgJwt "github.com/fazamuttaqien/calendly/pkg/jwt"
	"github.com/fazamuttaqien/calendly/pkg/validator"
	"github.com/google/uuid"
)

// POST /auth/register
func (h *Controller) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dto, ok := validator.GetValidatedDTOFromContext[dto.RegisterDto](ctx)
	if !ok {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Validated DTO not found in context", nil))
		return
	}

	// 1. Check if user already exists
	var exists bool
	err := h.db.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", dto.Email)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to check user existence", err))
		return
	}
	if exists {
		appError.WriteError(w, appError.NewAppError(enum.AuthEmailAlreadyExists, "User with this email already exists", nil))
		return
	}

	// 2. Hash password
	hashedPassword, err := helper.HashPassword(dto.Password)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to hash password", err))
		return
	}

	// 3. Generate unique username
	username, err := h.generateUsername(ctx, dto.Name)
	if err != nil {
		appError.WriteError(w, err)
		return
	}

	// 4. Start Transaction
	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to start transaction", err))
		return
	}
	// Defer rollback/commit logic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
			if err != nil {
				err = appError.NewAppError(enum.InternalServerError, "Failed to commit transaction", err)
			}
		}
	}()

	// 5. Insert User
	var createdUser model.User
	userInsertQuery := `
		INSERT INTO users (name, email, username, password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, name, email, username, image_url, created_at, updated_at; -- Do NOT return password hash
	`

	if err := tx.GetContext(ctx, &createdUser, userInsertQuery, dto.Name, dto.Email, username, hashedPassword); err != nil {
		// Could check for unique constraint violation on username if generateUsername had race condition
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to insert user", err))
		return
	}

	// 6. Create Availability Record
	var availabilityID string
	availInsertQuery := `
		INSERT INTO availability (user_id, time_gap, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW()) RETURNING id;
	`
	err = tx.GetContext(ctx, &availabilityID, availInsertQuery, createdUser.ID, 30) // Default timeGap = 30
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to create availability", err))
		return
	}

	// 7. Prepare and Insert Default DayAvailability Records
	dayInserts := make([]map[string]interface{}, 0, len(enum.AllDayOfWeek()))
	defaultStartTime := "09:00:00" // Default 9 AM
	defaultEndTime := "17:00:00"   // Default 5 PM

	for _, day := range enum.AllDayOfWeek() {
		isAvailable := (day != enum.Sunday && day != enum.Saturday) // Not available on weekends
		dayInserts = append(dayInserts, map[string]any{
			"availability_id": availabilityID,
			"day":             day,
			"start_time":      defaultStartTime,
			"end_time":        defaultEndTime,
			"is_available":    isAvailable,
		})
	}

	if len(dayInserts) > 0 {
		dayInsertQuery := `
			INSERT INTO day_availability (availability_id, day, start_time, end_time, is_available)
			VALUES (:availability_id, :day, :start_time, :end_time, :is_available);
		`
		_, err = tx.NamedExecContext(ctx, dayInsertQuery, dayInserts)
		if err != nil {
			appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to insert default day availability", err))
		}
	}

	// Commit is handled by defer
	if err != nil { // Check if commit failed
		appError.WriteError(w, err)
	}

	response := map[string]any{
		"message": "User created successfully",
		"user":    createdUser,
	}
	helper.ResponseJson(w, http.StatusCreated, response)
}

// POST /auth/login
func (h *Controller) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dto, ok := validator.GetValidatedDTOFromContext[dto.LoginDto](ctx)
	if !ok {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Validated DTO not found in context", nil))
		return
	}

	// 1. Find User by Email (including password hash)
	var user model.User
	query := `SELECT id, name, email, username, password, image_url, created_at, updated_at FROM users WHERE email = $1;`
	err := h.db.GetContext(ctx, &user, query, dto.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Use specific error code for user not found during login attempt
			appError.WriteError(w, appError.NewAppError(enum.AuthUserNotFound, "Invalid email or password", nil))
			return
		}
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to query user", err))
		return
	}

	// 2. Compare Password
	err = helper.ComparePassword(user.Password, dto.Password)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.AuthUnauthorizedAccess, "Invalid email or password", nil))
		return
	}

	// 3. Generate JWT
	accessToken, expiresAt, err := pkgJwt.SignJwtToken(user.ID)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to generate access token", err))
		return
	}

	// 4. Prepare and Return Response (omit password)
	user.Password = "" // Explicitly clear password before returning

	response := map[string]any{
		"message":     "User logged in successfully",
		"user":        user,
		"accessToken": accessToken,
		"expiresAt":   expiresAt,
	}
	helper.ResponseJson(w, http.StatusOK, response)
}

var (
	// Precompile regex for username generation
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)
	whitespaceRegexUser  = regexp.MustCompile(`\s+`)
)

// generateUsername creates a unique username based on the name.
// It needs access to the AuthService's db connection.
func (h *Controller) generateUsername(ctx context.Context, name string) (string, error) {
	// Clean base name
	baseUsername := strings.ToLower(name)
	baseUsername = whitespaceRegexUser.ReplaceAllString(baseUsername, "")  // Remove spaces
	baseUsername = nonAlphanumericRegex.ReplaceAllString(baseUsername, "") // Remove non-alphanumeric

	if baseUsername == "" {
		baseUsername = "user" // Fallback if name becomes empty
	}

	// Max attempts to prevent infinite loop (highly unlikely)
	maxAttempts := 10
	for range maxAttempts {
		// Generate suffix
		uid := uuid.NewString()
		// Use a slightly longer suffix for better uniqueness chances initially
		suffix := uid[:6]
		username := baseUsername + suffix

		// Check if username exists
		var exists bool
		err := h.db.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", username)
		if err != nil {
			return "", appError.NewAppError(enum.InternalServerError, "Failed to check username uniqueness", err)
		}

		if !exists {
			return username, nil // Found a unique username
		}
		// If exists, loop will continue
	}

	// If loop finishes, we failed to find a unique username after max attempts
	return "", appError.NewAppError(enum.InternalServerError, "Failed to generate unique username after multiple attempts", nil)
}
