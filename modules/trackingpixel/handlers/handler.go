package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/botginx/botginx/modules/trackingpixel/core"
	"github.com/botginx/botginx/modules/trackingpixel/models"
	"github.com/botginx/botginx/modules/trackingpixel/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
)

const (
	freePixels = 5
	pixelPrice = 2.0
)

// Domain is the minimal domain shape the handler needs.
type Domain struct {
	Name       string
	Verified   bool
	SSLEnabled bool
	HasServer  bool
}

// Deployer handles pixel subdomain deployment to Deploy VPS.
type Deployer interface {
	SetupPixelSubdomain(domain string) error
	CheckPixelSubdomainExists(domain string) (bool, error)
}

// Handler serves the module's pages and the public pixel.
type Handler struct {
	deps        *module.Dependencies
	store       *models.Store
	listDomains func(userID string) ([]Domain, error)
	ipSalt      []byte
	retention   time.Duration
	deployer    Deployer
}

func New(deps *module.Dependencies, store *models.Store,
	listDomains func(string) ([]Domain, error), ipSalt []byte, retention time.Duration) *Handler {
	return &Handler{deps: deps, store: store, listDomains: listDomains, ipSalt: ipSalt, retention: retention}
}

// SetDeployer sets the deploy service for pixel subdomain deployment.
func (h *Handler) SetDeployer(d Deployer) {
	h.deployer = d
}

// pixelURL generates the tracking pixel URL for a given token and domain.
func pixelURL(domain, token, format string) string {
	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}
	return fmt.Sprintf("https://px.%s/t/%s.%s", baseDomain, token, format)
}

// deployPixelSubdomainIfNeeded deploys nginx config for px.{domain} if not already done.
func (h *Handler) deployPixelSubdomainIfNeeded(domain string) {
	if h.deployer == nil {
		return
	}
	exists, err := h.deployer.CheckPixelSubdomainExists(domain)
	if err != nil {
		log.Printf("tracking-pixel: check subdomain: %v", err)
		return
	}
	if exists {
		return
	}
	if err := h.deployer.SetupPixelSubdomain(domain); err != nil {
		log.Printf("tracking-pixel: deploy subdomain px.%s: %v", services.GetBaseDomain(domain), err)
	} else {
		log.Printf("tracking-pixel: deployed subdomain px.%s", services.GetBaseDomain(domain))
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.index)
	r.Post("/create", h.create)
	r.Post("/{id}/delete", h.delete)
	return r
}

func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{token}", h.serve) // token may include ".gif"/".png"
	r.Head("/{token}", h.serve)
	return r
}

// TrackingRoutes returns routes for /t/{token} used by px.{domain} subdomains.
func (h *Handler) TrackingRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{token}", h.serve)
	r.Head("/{token}", h.serve)
	return r
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	pixels, err := h.store.ListByUser(user.ID)
	if err != nil {
		log.Printf("tracking-pixel: list pixels: %v", err)
	}
	doms, err := h.listDomains(user.ID)
	if err != nil {
		log.Printf("tracking-pixel: list domains: %v", err)
	}
	count, err := h.store.CountByUser(user.ID)
	if err != nil {
		log.Printf("tracking-pixel: count pixels: %v", err)
	}
	module.RenderUserSection(w, r, h.deps.Templates, "tracking-pixel:index.html",
		map[string]interface{}{
			"Title": "Tracking Pixel", "Pixels": pixels, "Domains": doms,
			"Balance": user.Balance, "Price": pixelPrice, "Free": freePixels,
			"NextIsFree": count < freePixels,
		})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	label := strings.TrimSpace(r.FormValue("label"))
	domain := strings.TrimSpace(r.FormValue("domain"))
	format := r.FormValue("format")
	if format != "png" {
		format = "gif"
	}
	if label == "" || domain == "" || len(label) > 200 {
		http.Error(w, "label (max 200 chars) and domain are required", http.StatusBadRequest)
		return
	}
	// Domain must be one of the user's verified domains.
	doms, err := h.listDomains(user.ID)
	canonical, ok := matchDomain(domain, doms)
	if err != nil || !ok {
		http.Error(w, "domain not available", http.StatusBadRequest)
		return
	}
	domain = canonical
	token, err := core.NewToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Billing + insert, atomic.
	tx, err := h.store.DB.Beginx()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Serialise concurrent creates per user so the free-tier count and the
	// balance debit cannot race.
	var lock string
	if err := tx.Get(&lock, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var count int
	if err := tx.Get(&count, `SELECT count(*) FROM tracking_pixels WHERE user_id=$1`, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count >= freePixels {
		res, err := tx.Exec(`UPDATE users SET balance = balance - $1, updated_at = NOW()
			WHERE id=$2 AND balance >= $1`, pixelPrice, user.ID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "insufficient balance", http.StatusPaymentRequired)
			return
		}
		if _, err := tx.Exec(`INSERT INTO balance_transactions (id,user_id,amount,type,description,created_at)
			VALUES (gen_random_uuid(),$1,$2,'debit',$3,NOW())`, user.ID, -pixelPrice, "Tracking pixel creation"); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	p := &models.Pixel{UserID: user.ID, Label: label, Domain: domain, Format: format, Token: token}
	if err := models.InsertPixelTx(tx, p); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Deploy px.{domain} subdomain nginx config in background
	go h.deployPixelSubdomainIfNeeded(domain)

	http.Redirect(w, r, "/user/tracking-pixel", http.StatusSeeOther)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = h.store.DeleteOwned(chi.URLParam(r, "id"), user.ID)
	http.Redirect(w, r, "/user/tracking-pixel", http.StatusSeeOther)
}

// serve is public: it always returns the image, logs best-effort.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "token")
	token, format := splitToken(raw)

	// Image first: the mail client must never wait on the DB.
	w.Header().Set("Content-Type", core.ContentType(format))
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(core.ImageFor(format))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	pixel, err := h.store.GetByToken(token)
	if err != nil || pixel == nil {
		return // unknown token: blank image already served, nothing logged
	}
	// Log unless the owner's balance is exhausted.
	if ownerBalanceOK(h, pixel.UserID) {
		class := core.Classify(r.UserAgent(), r.Method).String()
		_ = h.store.RecordOpen(pixel.ID, h.hashIP(clientIP(r)), r.UserAgent(), class)
	}
}

// matchDomain returns the canonical name of the verified domain matching domain.
func matchDomain(domain string, doms []Domain) (string, bool) {
	for _, d := range doms {
		if d.Verified && strings.EqualFold(d.Name, domain) {
			return d.Name, true
		}
	}
	return "", false
}

// ownerBalanceOK reports whether opens should be logged for this pixel's owner.
func ownerBalanceOK(h *Handler, userID string) bool {
	var bal float64
	if err := h.store.DB.Get(&bal, `SELECT balance FROM users WHERE id=$1`, userID); err != nil {
		return false
	}
	return bal > 0
}

func (h *Handler) hashIP(ip string) string {
	mac := hmac.New(sha256.New, h.ipSalt)
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

func splitToken(raw string) (token, format string) {
	if i := strings.LastIndexByte(raw, '.'); i >= 0 {
		ext := raw[i+1:]
		if ext == "png" || ext == "gif" {
			return raw[:i], ext
		}
	}
	return raw, "gif"
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
