package ctx

import (
	"context"
	"net/http"
)

type contextKey string

const UserKey contextKey = "user"

// User represents the authenticated user in context
type User struct {
	ID    string
	Email string
	Name  string
	Role  string // "user" or "admin"
}

// IsAdmin returns true if the user has admin role
func (u *User) IsAdmin() bool {
	return u != nil && u.Role == "admin"
}

// GetUser retrieves the authenticated user from the request context
func GetUser(r *http.Request) *User {
	if user, ok := r.Context().Value(UserKey).(*User); ok {
		return user
	}
	return nil
}

// GetUserID retrieves just the user ID, returns empty string if not authenticated
func GetUserID(r *http.Request) string {
	if user := GetUser(r); user != nil {
		return user.ID
	}
	return ""
}

// WithUser adds a user to the context
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}
