// Package domainstats provides middleware and helpers for domain statistics.
package domainstats

import (
	"context"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/botginx/botginx/pkg/ctx"
)

type contextKey int

const statsKey contextKey = 1

// Stats holds domain statistics for a user.
type Stats struct {
	Count int
}

// HasDomains returns true if the user has at least one domain.
func (s Stats) HasDomains() bool {
	return s.Count > 0
}

// Middleware attaches domain stats to the request context.
func Middleware(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := ctx.GetUser(r)
			if user == nil {
				next.ServeHTTP(w, r)
				return
			}

			var count int
			db.Get(&count, `SELECT COUNT(*) FROM domains WHERE user_id = $1`, user.ID)

			stats := Stats{Count: count}
			r = r.WithContext(context.WithValue(r.Context(), statsKey, stats))
			next.ServeHTTP(w, r)
		})
	}
}

// FromRequest returns the domain stats attached by Middleware.
func FromRequest(r *http.Request) Stats {
	if r == nil {
		return Stats{}
	}
	if stats, ok := r.Context().Value(statsKey).(Stats); ok {
		return stats
	}
	return Stats{}
}
