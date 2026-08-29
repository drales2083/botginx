package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localesFS embed.FS

type contextKey string

const langKey contextKey = "lang"

// Translator handles internationalization
type Translator struct {
	mu          sync.RWMutex
	defaultLang string
	available   []string
	messages    map[string]map[string]string // lang -> key -> message
}

func New(defaultLang string, available []string) *Translator {
	return &Translator{
		defaultLang: defaultLang,
		available:   available,
		messages:    make(map[string]map[string]string),
	}
}

// LoadFromFS loads translation files from embedded filesystem
func (t *Translator) LoadFromFS() error {
	for _, lang := range t.available {
		data, err := localesFS.ReadFile("locales/" + lang + ".json")
		if err != nil {
			continue // Skip missing locale files
		}

		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			continue
		}

		t.mu.Lock()
		t.messages[lang] = messages
		t.mu.Unlock()
	}
	return nil
}

// Load translations from a map (for runtime additions)
func (t *Translator) Load(lang string, messages map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.messages[lang] == nil {
		t.messages[lang] = make(map[string]string)
	}
	for k, v := range messages {
		t.messages[lang][k] = v
	}
}

// T translates a key to the given language
func (t *Translator) T(lang, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try requested language
	if msgs, ok := t.messages[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}

	// Fall back to default
	if msgs, ok := t.messages[t.defaultLang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}

	// Return key as-is
	return key
}

// TCtx translates using the language from context
func (t *Translator) TCtx(ctx context.Context, key string) string {
	lang := t.defaultLang
	if l, ok := ctx.Value(langKey).(string); ok {
		lang = l
	}
	return t.T(lang, key)
}

// Available returns available languages
func (t *Translator) Available() []string {
	return t.available
}

// Default returns the default language
func (t *Translator) Default() string {
	return t.defaultLang
}

// Middleware sets the language from cookie, query param, or header
func (t *Translator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := t.detectLanguage(r)

		// Set language cookie if changed via query param
		if qLang := r.URL.Query().Get("lang"); qLang != "" && t.isAvailable(qLang) {
			http.SetCookie(w, &http.Cookie{
				Name:     "lang",
				Value:    qLang,
				Path:     "/",
				MaxAge:   86400 * 365, // 1 year
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			lang = qLang
		}

		ctx := context.WithValue(r.Context(), langKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (t *Translator) detectLanguage(r *http.Request) string {
	// 1. Query param
	if lang := r.URL.Query().Get("lang"); lang != "" && t.isAvailable(lang) {
		return lang
	}

	// 2. Cookie
	if cookie, err := r.Cookie("lang"); err == nil && t.isAvailable(cookie.Value) {
		return cookie.Value
	}

	// 3. Accept-Language header
	if accept := r.Header.Get("Accept-Language"); accept != "" {
		for _, part := range strings.Split(accept, ",") {
			lang := strings.TrimSpace(strings.Split(part, ";")[0])
			lang = strings.Split(lang, "-")[0] // en-US -> en
			if t.isAvailable(lang) {
				return lang
			}
		}
	}

	return t.defaultLang
}

func (t *Translator) isAvailable(lang string) bool {
	for _, l := range t.available {
		if l == lang {
			return true
		}
	}
	return false
}

// TemplateFuncs returns template functions for translations.
//
// These are bound at parse time and fall back to the default language. The
// request's actual language is not known until render, so RequestTemplateFuncs
// overrides "t" and "currentLang" per request; both names must exist here for
// templates that use them to parse.
func (t *Translator) TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"t": func(key string) string {
			return t.T(t.defaultLang, key)
		},
		"tLang": func(lang, key string) string {
			return t.T(lang, key)
		},
		"languages": func() []string {
			return t.available
		},
		"defaultLang": func() string {
			return t.defaultLang
		},
		"currentLang": func() string {
			return t.defaultLang
		},
	}
}

// RequestTemplateFuncs returns the language-dependent functions bound to one
// request, so a visitor who picked Russian gets Russian rather than the
// configured default.
func (t *Translator) RequestTemplateFuncs(r *http.Request) template.FuncMap {
	lang := t.defaultLang
	if r != nil {
		if l := GetLang(r.Context()); l != "" && t.isAvailable(l) {
			lang = l
		}
	}

	return template.FuncMap{
		"t": func(key string) string {
			return t.T(lang, key)
		},
		"currentLang": func() string {
			return lang
		},
	}
}

// GetLang extracts language from context
func GetLang(ctx context.Context) string {
	if lang, ok := ctx.Value(langKey).(string); ok {
		return lang
	}
	return "en"
}
