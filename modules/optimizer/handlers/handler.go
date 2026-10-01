package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/botginx/botginx/modules/optimizer/core"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
)

const (
	maxUpload = 25 << 20
	tokenTTL  = 7 * 24 * time.Hour
)

// Handler serves the module's pages and endpoints.
type Handler struct {
	deps    *module.Dependencies
	signKey []byte
	// wrapBase is the absolute public URL of the redirector. Empty disables
	// wrap mode (requests are downgraded to neutralize).
	wrapBase string
}

// New constructs a Handler. wrapBase is the absolute public redirector URL
// (e.g. "https://app.example.com/optimizer/r"), or "" to disable wrap mode.
func New(deps *module.Dependencies, signKey []byte, wrapBase string) *Handler {
	return &Handler{deps: deps, signKey: signKey, wrapBase: wrapBase}
}

// Routes returns the auth-gated chi router mounted at /user/optimizer. The
// redirector is intentionally absent; see PublicRoutes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.index)
	r.Post("/process", h.process)
	return r
}

// PublicRoutes returns the unauthenticated redirector router (GET /{token}).
// The host must mount it outside auth middleware.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{token}", h.redirect)
	return r
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if ctx.GetUser(r) == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	module.RenderUserSection(w, r, h.deps.Templates, "optimizer:index.html",
		map[string]interface{}{"Title": "Attachment Optimizer"})
}

func (h *Handler) process(w http.ResponseWriter, r *http.Request) {
	if ctx.GetUser(r) == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "file too large or malformed", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	in, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	level := core.LevelAggressive
	mode := core.LinkWrap
	switch r.FormValue("level") {
	case "standard":
		level, mode = core.LevelStandard, core.LinkNeutralize
	case "maximum":
		level = core.LevelMaximum
	}
	if mode == core.LinkWrap && !core.IsHTTPURL(h.wrapBase) {
		mode = core.LinkNeutralize
	}

	res, err := core.Optimize(in, hdr.Header.Get("Content-Type"), core.Options{
		Level:    level,
		LinkMode: mode,
		WrapBase: h.wrapBase,
		SignKey:  h.signKey,
		TokenTTL: tokenTTL,
		MaxBytes: maxUpload,
	})
	if err != nil {
		if errors.Is(err, core.ErrTooLarge) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not process file", http.StatusInternalServerError)
		return
	}

	rep := res.Report
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Opt-Detected-Type", rep.DetectedType)
	w.Header().Set("X-Opt-Bytes-In", strconv.FormatInt(rep.BytesIn, 10))
	w.Header().Set("X-Opt-Bytes-Out", strconv.FormatInt(rep.BytesOut, 10))
	w.Header().Set("X-Opt-Trackers-Removed", strconv.Itoa(rep.TrackersRemoved))
	w.Header().Set("X-Opt-Scripts-Removed", strconv.Itoa(rep.ScriptsRemoved))
	w.Header().Set("X-Opt-Links-Handled", strconv.Itoa(rep.LinksHandled))
	w.Header().Set("X-Opt-PDF-JS-Removed", strconv.Itoa(rep.PDFJSRemoved))
	w.Header().Set("X-Opt-PDF-Embedded-Removed", strconv.Itoa(rep.PDFEmbeddedRemoved))
	w.Header().Set("X-Opt-Degraded", strconv.FormatBool(rep.Degraded))
	if rep.Degraded {
		w.Header().Set("X-Opt-Reason", reasonCode(rep.Reason))
	}
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", "optimized-"+hdr.Filename))
	w.Header().Set("Access-Control-Expose-Headers", "X-Opt-Detected-Type,X-Opt-Bytes-In,X-Opt-Bytes-Out,X-Opt-Trackers-Removed,X-Opt-Scripts-Removed,X-Opt-Links-Handled,X-Opt-PDF-JS-Removed,X-Opt-PDF-Embedded-Removed,X-Opt-Degraded,X-Opt-Reason")
	_, _ = w.Write(res.Output)
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	Redirect(h.signKey, w, r)
}

// Redirect verifies the {token} chi URL param against signKey and 302s to the
// target, or responds 410. It is a plain function so it has no router
// dependency; the request must be routed through a chi route with {token}.
func Redirect(signKey []byte, w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	url, err := core.VerifyToken(signKey, token)
	if err != nil || !core.IsHTTPURL(url) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte("This link has expired or is invalid."))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, url, http.StatusFound)
}

// reasonCode maps internal degradation text to a stable, non-leaking code.
func reasonCode(reason string) string {
	switch {
	case strings.HasPrefix(reason, "unsupported type"):
		return "unsupported_type"
	case strings.HasPrefix(reason, "panic"):
		return "panic"
	case strings.Contains(reason, "parse failed"):
		return "parse_failed"
	default:
		return "processing_failed"
	}
}
