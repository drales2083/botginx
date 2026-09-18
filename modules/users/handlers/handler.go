package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	authservices "github.com/botginx/botginx/modules/auth/services"
	"github.com/botginx/botginx/modules/users/models"
	"github.com/botginx/botginx/modules/users/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/subscription"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service       *services.UserService
	subscriptions *subscription.Service
	templates     *module.TemplateEngine
	authService   *authservices.AuthService
}

func NewHandler(
	service *services.UserService,
	subscriptions *subscription.Service,
	templates *module.TemplateEngine,
) *Handler {
	return &Handler{
		service:       service,
		subscriptions: subscriptions,
		templates:     templates,
	}
}

// SetAuthService sets the auth service for impersonation
func (h *Handler) SetAuthService(s *authservices.AuthService) {
	h.authService = s
}

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	users, _ := h.service.List()

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	// Calculate stats
	subscriptions := h.subscriptions.StatesFor(ids)
	var totalBalance float64
	var activeSubscriptions int
	for _, u := range users {
		totalBalance += u.Balance
	}
	for _, state := range subscriptions {
		if state.Active {
			activeSubscriptions++
		}
	}

	module.Render(w, r, h.templates, "users:list.html", map[string]interface{}{
		"Title":               "Users",
		"Users":               users,
		"Subscriptions":       subscriptions,
		"TotalUsers":          len(users),
		"ActiveSubscriptions": activeSubscriptions,
		"TotalBalance":        totalBalance,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "users:new.html", map[string]interface{}{
		"Title": "Create User",
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	module.Render(w, r, h.templates, "users:show.html", map[string]interface{}{
		"Title":            user.Name,
		"TargetUser":       user,
		"SubscriptionInfo": h.subscriptions.StateFor(user.ID),
	})
}

// Subscription management

// APIGrantSubscription activates or extends a user's subscription.
func (h *Handler) APIGrantSubscription(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	var input struct {
		Days      int    `json:"days"`
		ExpiresAt string `json:"expiresAt"`
		Plan      string `json:"plan"`
		Notes     string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := h.service.Get(userID); err != nil {
		h.jsonError(w, "User not found", http.StatusNotFound)
		return
	}

	grantedBy := ctx.GetUserID(r)

	// An explicit date wins; otherwise extend by a number of days.
	switch {
	case input.ExpiresAt != "":
		expires, err := time.Parse("2006-01-02", input.ExpiresAt)
		if err != nil {
			h.jsonError(w, "Invalid date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		// End of the chosen day, so a subscription set to today is not already
		// expired the moment it is granted.
		expires = expires.Add(24*time.Hour - time.Second)
		if err := h.subscriptions.Grant(userID, input.Plan, expires, grantedBy, input.Notes); err != nil {
			h.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case input.Days > 0:
		if err := h.subscriptions.Extend(userID, input.Days, grantedBy); err != nil {
			h.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

	default:
		h.jsonError(w, "Provide either days or expiresAt", http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"subscription": h.subscriptions.StateFor(userID),
	})
}

// APIRevokeSubscription ends a user's access immediately.
func (h *Handler) APIRevokeSubscription(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	if err := h.subscriptions.Revoke(userID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"subscription": h.subscriptions.StateFor(userID),
	})
}

// API handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.List()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"users": users})
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	var input models.CreateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Email == "" || input.Password == "" || input.Name == "" {
		h.jsonError(w, "Email, password and name are required", http.StatusBadRequest)
		return
	}

	user, err := h.service.Create(input)
	if err != nil {
		if err == services.ErrEmailExists {
			h.jsonError(w, "Email already registered", http.StatusConflict)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"user": user})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "User not found", http.StatusNotFound)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"user": user})
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.UpdateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"user": user})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

// APITopUpBalance adds funds to a user's balance
func (h *Handler) APITopUpBalance(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	var input struct {
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Amount <= 0 {
		h.jsonError(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	if _, err := h.service.Get(userID); err != nil {
		h.jsonError(w, "User not found", http.StatusNotFound)
		return
	}

	description := input.Description
	if description == "" {
		description = "Admin top-up"
	}

	if err := h.service.TopUpBalance(userID, input.Amount, description); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balance, _ := h.service.GetBalance(userID)

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"balance": balance,
	})
}

// APIImpersonate starts impersonation - admin logs in as another user
func (h *Handler) APIImpersonate(w http.ResponseWriter, r *http.Request) {
	// Verify current user is admin
	currentUser := ctx.GetUser(r)
	if currentUser == nil || !currentUser.IsAdmin() {
		h.jsonError(w, "Admin access required", http.StatusForbidden)
		return
	}

	if h.authService == nil {
		h.jsonError(w, "Auth service not available", http.StatusServiceUnavailable)
		return
	}

	targetUserID := chi.URLParam(r, "id")

	// Don't allow impersonating yourself
	if targetUserID == currentUser.ID {
		h.jsonError(w, "Cannot impersonate yourself", http.StatusBadRequest)
		return
	}

	// Get target user info
	targetUser, err := h.service.Get(targetUserID)
	if err != nil {
		h.jsonError(w, "User not found", http.StatusNotFound)
		return
	}

	// Create a new session for the target user
	newToken, err := h.authService.CreateImpersonationSession(targetUserID)
	if err != nil {
		h.jsonError(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Store admin's current session in a separate cookie so we can restore it
	adminCookie, err := r.Cookie("session")
	if err != nil {
		h.jsonError(w, "Admin session not found", http.StatusUnauthorized)
		return
	}

	// Set the admin session cookie for later restoration
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    adminCookie.Value,
		Path:     "/",
		MaxAge:   4 * 60 * 60, // 4 hours
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Set the new session cookie (impersonating target user)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    newToken,
		Path:     "/",
		MaxAge:   4 * 60 * 60, // 4 hours
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Set impersonation marker cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "impersonating",
		Value:    targetUser.Email,
		Path:     "/",
		MaxAge:   4 * 60 * 60,
		HttpOnly: false, // Allow JS to read for banner
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "Now impersonating " + targetUser.Email,
		"redirectUrl": "/user/dashboard",
	})
}

// APIExitImpersonation ends impersonation and returns to admin session
func (h *Handler) APIExitImpersonation(w http.ResponseWriter, r *http.Request) {
	// Get the stored admin session
	adminCookie, err := r.Cookie("admin_session")
	if err != nil || adminCookie.Value == "" {
		h.jsonError(w, "No admin session found", http.StatusBadRequest)
		return
	}

	// Restore admin session
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    adminCookie.Value,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Clear impersonation cookies
	http.SetCookie(w, &http.Cookie{
		Name:   "admin_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   "impersonating",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "Returned to admin session",
		"redirectUrl": "/admin/users",
	})
}

// Helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}
