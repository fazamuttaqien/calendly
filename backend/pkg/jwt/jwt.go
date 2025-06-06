package jwt

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTCustomClaims defines the claims for the JWT.
type JWTCustomClaims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

// SignJwtToken creates a new JWT for the given user ID.
func SignJwtToken(userID string) (tokenString string, expirestAt time.Time, err error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	expirestAt = expirationTime

	claims := &JWTCustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "calendly-app",
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))

	signedString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return signedString, expirestAt, nil
}
