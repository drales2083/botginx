// Package supportbadge provides middleware for showing unread support ticket count.
package supportbadge

import (
	"context"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/botginx/botginx/pkg/ctx"
)

type contextKey int

const badgeKey contextKey = 1

// Badge holds the unread support ticket count for a user.
type Badge struct {
	Count int
}

// HasUnread returns true if there are unread tickets.
func (b Badge) HasUnread() bool {
	return b.Count > 0
}

// Middleware attaches support badge count to the request context.
func Middleware(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := ctx.GetUser(r)
			if user == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Count tickets with status 'answered' (admin replied, user hasn't responded)
			var count int
			db.Get(&count, `
				SELECT COUNT(*) FROM support_tickets
				WHERE user_id = $1 AND status = 'answered'
			`, user.ID)

			badge := Badge{Count: count}
			r = r.WithContext(context.WithValue(r.Context(), badgeKey, badge))
			next.ServeHTTP(w, r)
		})
	}
}

// FromRequest returns the support badge attached by Middleware.
func FromRequest(r *http.Request) Badge {
	if r == nil {
		return Badge{}
	}
	if badge, ok := r.Context().Value(badgeKey).(Badge); ok {
		return badge
	}
	return Badge{}
}
