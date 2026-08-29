package handlers

import (
	"encoding/json"
	"net/http"
	"time"

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

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	users, _ := h.service.List()

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	module.Render(w, r, h.templates, "users:list.html", map[string]interface{}{
		"Title":         "Users",
		"Users":         users,
		"Subscriptions": h.subscriptions.StatesFor(ids),
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

// Helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}
