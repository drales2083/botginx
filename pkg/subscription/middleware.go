package subscription

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/botginx/botginx/pkg/ctx"
)

type contextKey string

const stateKey contextKey = "subscription_state"

// Attach puts the subscription state on the request without gating anything.
//
// Separate from Enforce because pages that must stay reachable while a
// subscription is lapsed -- account settings, error pages -- still need the
// state to render an accurate banner.
func (s *Service) Attach(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := ctx.GetUser(r)

		state := State{Status: StatusNone}
		if user != nil {
			state = s.StateFor(user.ID)
		}

		// Staff are not billed; an admin managing the platform is never gated.
		if user != nil && user.IsAdmin() {
			state.Active = true
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), stateKey, state)))
	})
}

// Enforce blocks writes from users without an active subscription. It reads the
// state left by Attach, which must run before it.
//
// Enforcement is by HTTP method, not by URL: anything that is not a read (GET,
// HEAD, OPTIONS) requires an active subscription. Hiding buttons in templates
// is presentation only -- this is what actually stops a user calling the API
// directly, which is why the check lives here rather than in each handler.
//
// Scope it to product routes only. Account self-management must not be gated:
// a user whose subscription lapsed still has to be able to change their own
// password, which matters most exactly when an account is compromised.
func (s *Service) Enforce(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isRead(r.Method) || FromRequest(r).Active {
			next.ServeHTTP(w, r)
			return
		}
		deny(w, r)
	})
}

func isRead(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func deny(w http.ResponseWriter, r *http.Request) {
	const message = "An active subscription is required for this action. Contact an administrator."

	// API callers get JSON; a form post gets a page it can actually read.
	if strings.Contains(r.URL.Path, "/api/") ||
		strings.Contains(r.Header.Get("Accept"), "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" {

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":              message,
			"subscriptionExpired": true,
		})
		return
	}

	http.Error(w, message, http.StatusForbidden)
}

// FromRequest returns the subscription state attached by Middleware.
func FromRequest(r *http.Request) State {
	if r == nil {
		return State{Status: StatusNone}
	}
	if state, ok := r.Context().Value(stateKey).(State); ok {
		return state
	}
	return State{Status: StatusNone}
}
