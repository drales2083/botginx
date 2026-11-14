// Package agenticweb exposes WebMCP tools, llms.txt, and ard.json for AI browsing agents.
package agenticweb

import (
	"github.com/botginx/botginx/modules/agenticweb/handlers"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Module struct {
	*module.BaseModule
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"agenticweb",
			"Agentic Web",
			"WebMCP tools and AI discovery for browsing agents",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)
	m.handler = handlers.NewHandler(deps.DB)
	return nil
}

func (m *Module) Migrate() error {
	return nil // No database tables needed
}

// PublicRoutes returns routes that don't require authentication
func (m *Module) PublicRoutes() chi.Router {
	r := chi.NewRouter()

	// Discovery endpoints
	r.Get("/llms.txt", m.handler.LLMsTxt)
	r.Get("/llms-full.txt", m.handler.LLMsFullTxt)
	r.Get("/ard.json", m.handler.ARDJson)

	return r
}

// WellKnownRoutes returns routes for /.well-known/
func (m *Module) WellKnownRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ard.json", m.handler.ARDJson)
	return r
}

// WebMCPRoutes returns the WebMCP API endpoints (require auth)
func (m *Module) WebMCPRoutes() chi.Router {
	r := chi.NewRouter()

	// Read operations
	r.Get("/domains", m.handler.WebMCPDomains)
	r.Get("/links", m.handler.WebMCPLinks)
	r.Get("/link-stats", m.handler.WebMCPLinkStats)
	r.Get("/search", m.handler.WebMCPSearch)

	// Write operations
	r.Post("/links", m.handler.WebMCPCreateLink)
	r.Put("/links", m.handler.WebMCPUpdateLink)
	r.Delete("/links", m.handler.WebMCPDeleteLink)

	return r
}

func (m *Module) Routes() chi.Router {
	return m.PublicRoutes()
}
