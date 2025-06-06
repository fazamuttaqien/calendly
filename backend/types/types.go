package types

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string
// ValidatedDTOKey is the key used to store the validated DTO in the request context.
const ValidatedDTOKey ContextKey = "validatedDTO"
// UserIDKey is the key used to store the authenticated user's ID in the request context.
const UserIDKey ContextKey = "userId"
