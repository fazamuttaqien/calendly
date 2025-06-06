package router

import (
	"net/http"
	"os"
	"time"

	"github.com/fazamuttaqien/calendly/internal/dto"
	"github.com/fazamuttaqien/calendly/internal/presenter"
	"github.com/fazamuttaqien/calendly/middleware"
	"github.com/fazamuttaqien/calendly/pkg/validator"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func New(presenters presenter.Presenter) *chi.Mux {
	r := chi.NewRouter()

	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{os.Getenv("FRONTEND_ORIGIN")}, // Use this to allow specific origin hosts
		// AllowedOrigins:   []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Initialize middlewares
	authMiddleware := middleware.AuthMiddleware
	errorHandlerMiddleware := middleware.ErrorMiddleware

	// Global middleware stack
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))
	r.Use(errorHandlerMiddleware)
	r.Use(securityHeadersMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// --- Auth Routes (Public) ---
		r.Route("/auth", func(r chi.Router) {
			r.With(middleware.WithValidation[dto.RegisterDto](validator.SourceBody)).
				Post("/register", presenters.Controllers.Register)

			r.With(middleware.WithValidation[dto.LoginDto](validator.SourceBody)).
				Post("/login", presenters.Controllers.Login)
		})

		// --- Availability Routes ---
		r.Route("/availability", func(r chi.Router) {
			// Public availability endpoints
			r.Route("/public", func(r chi.Router) {
				r.Get("/{eventId}", presenters.Controllers.GetPublicEventAvailability)
			})

			// Protected availability endpoints
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware)
				r.Get("/", presenters.Controllers.GetUserAvailability)
				r.With(middleware.WithValidation[dto.UpdateAvailabilityDto](validator.SourceBody)).
					Put("/", presenters.Controllers.UpdateAvailability)
			})
		})

		// --- Event Routes ---
		r.Route("/event", func(r chi.Router) {
			// Public event endpoints
			r.Route("/public", func(r chi.Router) {
				r.Get("/{username}", presenters.Controllers.GetPublicByUsername)
				r.Get("/{username}/{slug}", presenters.Controllers.GetPublicBySlug)
			})

			// Protected event endpoints
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware)

				r.Get("/", presenters.Controllers.GetUserEvents)

				r.With(middleware.WithValidation[dto.CreateEventDto](validator.SourceBody)).
					Post("/", presenters.Controllers.CreateEvent)

				r.Route("/{eventId}", func(r chi.Router) {
					r.Put("/toggle-privacy", presenters.Controllers.TogglePrivacy)
					r.Delete("/", presenters.Controllers.DeleteEvent)
				})
			})
		})

		// --- Integration Routes ---
		r.Route("/integration", func(r chi.Router) {

			r.Get("/google/callback", presenters.Controllers.GoogleOAuthCallback)

			// Protected integration endpoints
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware)
				r.Get("/", presenters.Controllers.GetUserIntegrations)
				r.Get("/check/{appType}", presenters.Controllers.CheckIntegration)
				r.Get("/connect/{appType}", presenters.Controllers.ConnectApp)
			})
		})

		// --- Meeting Routes ---
		r.Route("/meeting", func(r chi.Router) {
			// Public meeting endpoints
			r.Route("/public", func(r chi.Router) {
				r.With(middleware.WithValidation[dto.CreateMeetingDto](validator.SourceBody)).
					Post("/", presenters.Controllers.CreateBooking)
			})

			// Protected meeting endpoints
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware)
				r.Get("/", presenters.Controllers.GetUserMeetings)
				r.Delete("/{meetingId}", presenters.Controllers.CancelMeeting)
			})
		})
	})

	// Health check endpoint for monitoring
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return r
}

// Enhanced security headers middleware
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic security headers
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Enhanced CSP policy
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self';")

		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(self), microphone=(), camera=(), payment=()")

		// Cache control for API responses
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	})
}
