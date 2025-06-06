package controller

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/fazamuttaqien/calendly/helper"
	"github.com/fazamuttaqien/calendly/internal/model"
	"github.com/fazamuttaqien/calendly/middleware"
	appError "github.com/fazamuttaqien/calendly/pkg/app-error"
	"github.com/fazamuttaqien/calendly/pkg/enum"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GET /me/integrations
func (i *Controller) GetUserIntegrations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	var userIntegrations []model.Integration

	query := `SELECT app_type FROM integrations WHERE user_id = $1 AND is_connected = TRUE;`

	err := i.db.SelectContext(ctx, &userIntegrations, query, userID)
	if err != nil && err != sql.ErrNoRows {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to fetch user integrations", err))
		return
	}

	connectedMap := make(map[enum.IntegrationAppType]bool)
	for _, integration := range userIntegrations {
		connectedMap[enum.IntegrationAppType(integration.AppType)] = true
	}

	integrations := make([]IntegrationStatus, 0, len(enum.AllIntegrationAppType()))
	for _, appType := range enum.AllIntegrationAppType() {
		// Use maps define above to get details
		provider, _ := appTypeToProviderMap[appType] //	Handle missing entries if maps aren't exclusive
		category, _ := appTypeToCategoryMap[appType]
		title, _ := appTypeToTitleMap[appType]

		integrations = append(integrations, IntegrationStatus{
			Provider:    provider,
			Title:       title,
			AppType:     appType,
			Category:    category,
			IsConnected: connectedMap[appType],
		})
	}

	response := map[string]any{
		"message":      "Fetched user integrations successfully",
		"integrations": integrations,
	}

	helper.ResponseJson(w, http.StatusOK, response)
}

// GET /me/integrations/check/{appType}
func (i *Controller) CheckIntegration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	appTypeStr := chi.URLParam(r, "appType")
	if appTypeStr == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing appType in path", nil))
		return
	}

	// Convert string to enum type
	appType := enum.IntegrationAppType(strings.ToUpper(appTypeStr))
	isValid := slices.Contains(enum.AllIntegrationAppType(), appType)
	if !isValid {
		msg := fmt.Sprintf("Invalid appType provided: %s", appTypeStr)
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, msg, nil))
		return
	}

	var isConnected bool
	query := `SELECT EXISTS (SELECT 1 FROM integrations WHERE user_id = $1 AND app_type = $2 AND is_connected = TRUE);`

	err := i.db.GetContext(ctx, &isConnected, query, userID, appType)
	if err != nil {
		// Do not treat ErrNoRows as error, GetContext handles it correctly for EXISTS
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to check integration existence", err))
		return
	}

	response := map[string]any{
		"message":     "Integration checked successfully",
		"isConnected": isConnected,
	}
	helper.ResponseJson(w, http.StatusOK, response)
}

// POST /me/integrations/connect/{appType}
func (i *Controller) ConnectApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		appError.WriteError(w, appError.NewUnauthorizedError(nil))
		return
	}

	appTypeStr := chi.URLParam(r, "appType")
	if appTypeStr == "" {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Missing appType in path", nil))
		return
	}

	// Convert string to enum type
	appType := enum.IntegrationAppType(strings.ToUpper(appTypeStr))
	isValid := slices.Contains(enum.AllIntegrationAppType(), appType)
	if !isValid {
		msg := fmt.Sprintf("Invalid appType provided: %s", appTypeStr)
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, msg, nil))
		return
	}

	stateData := OAuthState{
		UserID:  userID,
		AppType: appType,
	}

	stateString, err := EncodeState(stateData)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to encode state", err))
		return
	}

	var authUrl string

	switch appType {
	case enum.AppGoogleMeetAndCalendar:
		// Add options for offline access (refresh token) and consent prompt
		opts := []oauth2.AuthCodeOption{
			oauth2.AccessTypeOffline,
			oauth2.ApprovalForce, // Equivalent to prompt=consent
		}
		authUrl = googleOAuthConfig.AuthCodeURL(stateString, opts...)

	case enum.AppZoomMeeting, enum.AppOutlookCalendar:
		// TODO: Implement OAuth flow for Zoom and Microsoft
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Unsupported app type for connection", nil))
		return
	default:
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Unknown app type", nil))
		return
	}

	response := map[string]any{
		"url": authUrl,
	}
	helper.ResponseJson(w, http.StatusOK, response)
}

// GET /auth/google/callback
// NOTE: This handler usually DOES NOT have the JWT AuthMiddleware applied.
func (i *Controller) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	code := query.Get("code")
	stateEncoded := query.Get("state")

	// --- Build Redirect URL (helper) ---
	buildRedirectURL := func(appType enum.IntegrationAppType, queryParams map[string]string) string {
		// Start with base frontend URL + app type indicator
		baseURL := i.frontendUrl
		if !strings.Contains(baseURL, "?") {
			baseURL += "?"
		} else if !strings.HasSuffix(baseURL, "&") && !strings.HasSuffix(baseURL, "?") {
			baseURL += "&"
		}
		// Ensure app_type is always present, even on error, for context on FE
		redirectURL := fmt.Sprintf("%sapp_type=%s", baseURL, url.QueryEscape(string(appType)))

		// Append other query parameters (like error or success)
		for key, val := range queryParams {
			redirectURL += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(val))
		}
		return redirectURL
	}

	// --- State Validation ---
	if stateEncoded == "" {
		// Redirect with generic error, as we don't know the appType yet
		errorRedirectURL := fmt.Sprintf("%s?error=%s", i.frontendUrl, url.QueryEscape("Invalid state parameter"))
		http.Redirect(w, r, errorRedirectURL, http.StatusTemporaryRedirect)
		return
	}

	state, err := DecodeState(stateEncoded) // Use DecodeState from services
	// Use state.AppType to build specific redirect URL on error
	if err != nil {
		redirectURL := buildRedirectURL(
			state.AppType,
			map[string]string{"error": "Invalid state parameter"},
		)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
	if state.UserID == "" {
		redirectURL := buildRedirectURL(
			state.AppType,
			map[string]string{"error": "UserID is required in state"},
		)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
	// Add CSRF validation here if implemented

	// --- Code Validation ---
	if code == "" {
		redirectURL := buildRedirectURL(
			state.AppType,
			map[string]string{"error": "Invalid authorization code"},
		)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// --- Token Exchange ---
	// Use global or service's oauth config
	token, err := GetGoogleOAuthConfig().Exchange(ctx, code)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to exchange token: %v", err)
		redirectURL := buildRedirectURL(state.AppType, map[string]string{"error": errMsg})
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
	if !token.Valid() || token.AccessToken == "" {
		redirectURL := buildRedirectURL(
			state.AppType,
			map[string]string{"error": "Invalid token received"},
		)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// --- Prepare Integration Data ---
	var expiryDate sql.NullInt64
	if !token.Expiry.IsZero() {
		expiryDate = sql.NullInt64{Int64: token.Expiry.Unix(), Valid: true}
	}
	refreshToken := sql.NullString{String: token.RefreshToken, Valid: token.RefreshToken != ""}

	// Extract metadata (example for Google)
	metadata := map[string]any{
		"scope":      token.Extra("scope"),
		"token_type": token.TokenType,
		// Add other relevant metadata if needed
	}

	data := CreateIntegration{
		UserID:       state.UserID,
		AppType:      state.AppType,
		AccessToken:  token.AccessToken,
		RefreshToken: refreshToken,
		ExpiryDate:   expiryDate,
		Metadata:     metadata,
	}

	// --- Create Integration in DB ---
	// Determine Provider and Category from AppType
	provider, okP := appTypeToProviderMap[data.AppType]
	category, okC := appTypeToCategoryMap[data.AppType]
	if !okP || !okC {
		appError.WriteError(w, appError.NewAppError(enum.BadRequest, "Invalid app type provided", nil))
		return
	}

	// Marshal metadata to JSONB
	metadataJSON, err := json.Marshal(data.Metadata)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(enum.InternalServerError, "Failed to marshal metadata", err))
		return
	}

	// Use ON CONFLICT to handle existing integrations (UPSERT)
	var integration model.Integration
	queryIntegrations := `
		INSERT INTO integrations (
			user_id, provider, category, app_type, access_token,
			refresh_token, expiry_date, metadata, is_connected,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
		RETURNING id, user_id, provider, category, app_type, access_token, refresh_token, expiry_date, metadata, is_connected, created_at, updated_at;
	`
	// ON CONFLICT (user_id, app_type) DO UPDATE SET
	// 	access_token = EXCLUDED.access_token,
	// 	refresh_token = EXCLUDED.refresh_token,
	// 	expiry_date = EXCLUDED.expiry_date,
	// 	metadata = EXCLUDED.metadata,
	// 	is_connected = TRUE, -- Ensure it's marked connected on update
	// 	updated_at = CURRENT_TIMESTAMP

	if err := i.db.GetContext(ctx, &integration, queryIntegrations,
		data.UserID, provider, category, data.AppType, data.AccessToken,
		data.RefreshToken, data.ExpiryDate, metadataJSON,
	); err != nil {
		errMsg := fmt.Sprintf("Failed to save integration: %v", err)
		// Check if it's a known error like "already connected" if CreateIntegration uses ON CONFLICT
		if appErr, ok := err.(*appError.AppError); ok && appErr.Code == enum.BadRequest {
			errMsg = appErr.Error()
		}
		redirectURL := buildRedirectURL(state.AppType, map[string]string{"error": errMsg})
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	// --- Success Redirect ---
	successRedirectURL := buildRedirectURL(state.AppType, map[string]string{"success": "true"})
	http.Redirect(w, r, successRedirectURL, http.StatusTemporaryRedirect)
}

// Helper function (example) to update token data in DB - call this after refreshing!
func (i *Controller) UpdateIntegrationToken(ctx context.Context, userID string, appType enum.IntegrationAppType, newToken *oauth2.Token) error {
	expiryUnix := sql.NullInt64{Valid: false}
	if !newToken.Expiry.IsZero() {
		expiryUnix = sql.NullInt64{Int64: newToken.Expiry.Unix(), Valid: true}
	}
	refreshToken := sql.NullString{Valid: false}
	if newToken.RefreshToken != "" {
		refreshToken = sql.NullString{String: newToken.RefreshToken, Valid: true}
	}

	query := `
        UPDATE integrations SET
            access_token = $1,
            refresh_token = $2,
            expiry_date = $3,
            updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $4 AND app_type = $5;
    `
	_, err := i.db.ExecContext(ctx, query, newToken.AccessToken, refreshToken, expiryUnix, userID, appType)
	if err != nil {
		return appError.NewAppError(enum.InternalServerError, "Failed to update integration token in DB", err)
	}
	return nil
}

// --- Mappings ---

var (
	appTypeToProviderMap = map[enum.IntegrationAppType]enum.IntegrationProvider{
		enum.AppGoogleMeetAndCalendar: enum.ProviderGoogle,
		enum.AppZoomMeeting:           enum.ProviderZoom,
		enum.AppOutlookCalendar:       enum.ProviderMicrosoft,
	}

	appTypeToCategoryMap = map[enum.IntegrationAppType]enum.IntegrationCategory{
		enum.AppGoogleMeetAndCalendar: enum.CategoryCalendarAndVideo,
		enum.AppZoomMeeting:           enum.CategoryVideo,
		enum.AppOutlookCalendar:       enum.CategoryCalendar,
	}

	appTypeToTitleMap = map[enum.IntegrationAppType]string{
		enum.AppGoogleMeetAndCalendar: "Google Meet & Calendar",
		enum.AppZoomMeeting:           "Zoom",
		enum.AppOutlookCalendar:       "Outlook Calendar",
	}
)

// --- OAuth2 Configuration (Global or within Service) ---

var googleOAuthConfig *oauth2.Config

func init() {
	// Initialize Google OAuth2 Config
	googleOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URI"),
		Scopes: []string{
			"https://www.googleapis.com/auth/calendar.events",
			// Add other required scopes here
		},
		Endpoint: google.Endpoint,
	}
}

func GetGoogleOAuthConfig() *oauth2.Config {
	// Ensure googleOAuthConfig is initialized (e.g., in init())
	if googleOAuthConfig == nil {
		panic("Google OAuth2 config not initialized")
	}
	return googleOAuthConfig
}

// --- State Encoding/Decoding ---

// OAuthState represents the data encoded in the OAuth state parameter.
type OAuthState struct {
	UserID  string                  `json:"userId"`
	AppType enum.IntegrationAppType `json:"appType"`
	// Add other relevant fields like redirect URL, CSRF token etc. if needed
}

// EncodeState encodes state data into a Base64 string.
func EncodeState(state OAuthState) (string, error) {
	jsonData, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(jsonData), nil
}

// DecodeState decodes a Base64 string back into state data.
func DecodeState(encodedState string) (OAuthState, error) {
	jsonData, err := base64.URLEncoding.DecodeString(encodedState)
	if err != nil {
		return OAuthState{}, fmt.Errorf("failed to decode base64 state: %w", err)
	}

	var state OAuthState
	err = json.Unmarshal(jsonData, &state)
	if err != nil {
		return OAuthState{}, fmt.Errorf("failed to unmarshal state JSON: %w", err)
	}

	return state, nil
}
