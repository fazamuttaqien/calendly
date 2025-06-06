package helper

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	nonAlphanumericDashRegex = regexp.MustCompile(`[^\w\-]+`)
	multipleDashesRegex      = regexp.MustCompile(`\-\-+`)
	leadingDashRegex         = regexp.MustCompile(`^-+`)
	trailingDashRegex        = regexp.MustCompile(`-+$`)
	whitespaceRegex          = regexp.MustCompile(`\s+`)
)

// Slugify creates a URL-friendly slug from text with a short UUID suffix.
func Slugify(text string) string {
	// Generate short UUID suffix (first 4 chars often enough for uniqueness here)
	uid := uuid.NewString()
	shortUUID := uid[:4]

	// Convert to lowercase
	slug := strings.ToLower(text)

	// Replace whitespace with dashes
	slug = whitespaceRegex.ReplaceAllString(slug, "-")

	// Remove invalid characters
	slug = nonAlphanumericDashRegex.ReplaceAllString(slug, "")

	// Collapse multiple dashes
	slug = multipleDashesRegex.ReplaceAllString(slug, "-")

	// Trim leading/trailing dashes
	slug = leadingDashRegex.ReplaceAllString(slug, "")
	slug = trailingDashRegex.ReplaceAllString(slug, "")

	if slug == "" {
		// Handle cases where the text results in an empty slug
		return shortUUID
	}

	return slug + "-" + shortUUID
}
