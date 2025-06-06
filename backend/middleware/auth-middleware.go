package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
	"github.com/fazamuttaqien/calendly/pkg/enum"
	pkgJwt "github.com/fazamuttaqien/calendly/pkg/jwt"
	"github.com/fazamuttaqien/calendly/types"

	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware creates a middleware handler for JWT authentication.
// It verifies the token and adds the userID to the request context.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			err := appError.NewAppError(enum.AuthTokenNotFound, "Authorization header not found", nil)
			appError.WriteError(w, err)
			return
		}

		// Check for "Bearer " prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			err := appError.NewAppError(enum.AuthInvalidToken, "Invalid authorization header format", nil)
			appError.WriteError(w, err)
			return
		}

		tokenString := parts[1]
		jwtSecret := []byte(os.Getenv("JWT_SECRET"))

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(
			tokenString, &pkgJwt.JWTCustomClaims{}, func(token *jwt.Token) (any, error) {
				// Ensure the signing method is what you expect (e.g., HMAC)
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return jwtSecret, nil
			})

		// Handle parsing errors
		if err != nil {
			var appErr *appError.AppError
			if errors.Is(err, jwt.ErrTokenExpired) {
				appErr = appError.NewAppError(enum.AuthInvalidToken, "Token has expired", err)
			} else if errors.Is(err, jwt.ErrSignatureInvalid) {
				appErr = appError.NewAppError(enum.AuthInvalidToken, "Invalid token signature", err)
			} else {
				// Other parsing errors
				appErr = appError.NewAppError(enum.AuthInvalidToken, "Invalid token", err)
			}
			appError.WriteError(w, appErr)
			return
		}

		// Check if token is valid and claims can be asserted
		if claims, ok := token.Claims.(*pkgJwt.JWTCustomClaims); ok && token.Valid {
			if claims.UserID == "" {
				// Should not happen if token generation is correct, but check anyway
				err := appError.NewAppError(enum.AuthInvalidToken, "Token missing required user information", nil)
				appError.WriteError(w, err)
				return
			}

			// Add userID to context
			ctx := context.WithValue(r.Context(), types.UserIDKey, claims.UserID) // Use defined UserIDKey

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			// Token is invalid for other reasons
			err := appError.NewAppError(enum.AuthInvalidToken, "Invalid token claims", nil)
			appError.WriteError(w, err)
			return
		}
	})
}

// GetUserIDFromContext retrieves the user ID stored by auth middleware.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(types.UserIDKey).(string)
	return userID, ok
}
