package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/botginx/botginx/modules/auth/models"
	"github.com/botginx/botginx/modules/auth/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/subscription"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service          *services.AuthService
	templates        *module.TemplateEngine
	globalWhitelist  *services.GlobalWhitelistService
}

func NewHandler(service *services.AuthService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) SetGlobalWhitelistService(s *services.GlobalWhitelistService) {
	h.globalWhitelist = s
}

// Pages

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	module.RenderAuth(w, r, h.templates, "auth:login.html", map[string]interface{}{
		"Title": "Sign In",
	})
}

func (h *Handler) SignupPage(w http.ResponseWriter, r *http.Request) {
	module.RenderAuth(w, r, h.templates, "auth:signup.html", map[string]interface{}{
		"Title": "Sign Up",
	})
}

// SettingsPage is the signed-in user's own account page, mounted under
// /user/settings rather than /auth so it sits inside the app chrome.
func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	account, err := h.service.GetUser(user.ID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	var globalWhitelist []models.GlobalWhitelist
	if h.globalWhitelist != nil {
		globalWhitelist, _ = h.globalWhitelist.List(user.ID)
	}

	module.RenderUserSection(w, r, h.templates, "auth:settings.html", map[string]interface{}{
		"Title":           "Settings",
		"Account":         account,
		"GlobalWhitelist": globalWhitelist,
		"MaxWhitelistIPs": services.MaxGlobalWhitelistIPs,
	})
}

func (h *Handler) SubscriptionPage(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "auth:subscription.html", map[string]interface{}{
		"Title": "Subscription",
	})
}

// Global Whitelist API handlers

func (h *Handler) APIGetGlobalWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	if h.globalWhitelist == nil {
		h.jsonError(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	entries, err := h.globalWhitelist.List(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"whitelist": entries,
		"maxIps":    services.MaxGlobalWhitelistIPs,
	})
}

func (h *Handler) APIAddGlobalWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Subscription required for product features
	if !subscription.FromRequest(r).Active {
		h.jsonError(w, "An active subscription is required", http.StatusForbidden)
		return
	}

	if h.globalWhitelist == nil {
		h.jsonError(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	var input struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	input.IP = strings.TrimSpace(input.IP)
	if input.IP == "" {
		h.jsonError(w, "IP address is required", http.StatusBadRequest)
		return
	}

	entry, err := h.globalWhitelist.Add(userID, input.IP)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"entry":   entry,
	})
}

func (h *Handler) APIRemoveGlobalWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Subscription required for product features
	if !subscription.FromRequest(r).Active {
		h.jsonError(w, "An active subscription is required", http.StatusForbidden)
		return
	}

	if h.globalWhitelist == nil {
		h.jsonError(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		h.jsonError(w, "ID is required", http.StatusBadRequest)
		return
	}

	if err := h.globalWhitelist.Remove(userID, id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)

	if input.Name == "" || input.Email == "" {
		h.jsonError(w, "Name and email are required", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateProfile(userID, input.Name, input.Email); err != nil {
		if err == services.ErrEmailExists {
			h.jsonError(w, "That email is already in use", http.StatusConflict)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(input.NewPassword) < 8 {
		h.jsonError(w, "New password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if err := h.service.ChangePassword(userID, input.CurrentPassword, input.NewPassword); err != nil {
		if err == services.ErrInvalidCredentials {
			h.jsonError(w, "Current password is incorrect", http.StatusForbidden)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// ChangePassword drops every session, this one included, so the browser is
	// holding a cookie that no longer resolves. Clear it and let the client
	// send the user to the login page.
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"reauthorize": true,
	})
}

// API handlers

func (h *Handler) APISignup(w http.ResponseWriter, r *http.Request) {
	var input models.SignupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Email == "" || input.Password == "" || input.Name == "" {
		h.jsonError(w, "Email, password and name are required", http.StatusBadRequest)
		return
	}

	if len(input.Password) < 8 {
		h.jsonError(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Capture referral code from query param
	refCode := r.URL.Query().Get("ref")

	user, err := h.service.SignupWithReferral(input, refCode)
	if err != nil {
		if err == services.ErrEmailExists {
			h.jsonError(w, "Email already registered", http.StatusConflict)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"user":    user,
	})
}

func (h *Handler) APILogin(w http.ResponseWriter, r *http.Request) {
	var input models.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, token, err := h.service.Login(input)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			h.jsonError(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user":    user,
		"token":   token,
	})
}

func (h *Handler) APILogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		h.service.Logout(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIMe(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	if user == nil {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"user": user})
}

// Middleware

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		// Check cookie first
		if cookie, err := r.Cookie("session"); err == nil {
			token = cookie.Value
		}

		// Check Authorization header
		if token == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if token == "" {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		user, err := h.service.ValidateSession(token)
		if err != nil {
			// Clear invalid cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "session",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Add user to context using shared ctx package
		ctxUser := &ctx.User{
			ID:      user.ID,
			Email:   user.Email,
			Name:    user.Name,
			Role:    user.Role,
			Balance: user.Balance,
		}
		newCtx := ctx.WithUser(r.Context(), ctxUser)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}

// AdminMiddleware requires admin role
func (h *Handler) AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := ctx.GetUser(r)
		if user == nil || !user.IsAdmin() {
			// Check if API request
			if strings.HasPrefix(r.URL.Path, "/admin/") && strings.Contains(r.URL.Path, "/api/") {
				h.jsonError(w, "Admin access required", http.StatusForbidden)
				return
			}
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		if cookie, err := r.Cookie("session"); err == nil {
			token = cookie.Value
		}

		if token == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if token != "" {
			if user, err := h.service.ValidateSession(token); err == nil {
				ctxUser := &ctx.User{
					ID:      user.ID,
					Email:   user.Email,
					Name:    user.Name,
					Role:    user.Role,
					Balance: user.Balance,
				}
				r = r.WithContext(ctx.WithUser(r.Context(), ctxUser))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// JSON helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}
