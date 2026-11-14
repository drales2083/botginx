package intel

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/botginx/botginx/modules/intel/handlers"
	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	contentService  *services.ContentService
	markdownService *services.MarkdownService
	publicHandler   *handlers.PublicHandler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"intel",
			"Intel",
			"File-based blog and content",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	// Find content directory
	contentDir := findContentDir()

	var err error
	m.contentService, err = services.NewContentService(contentDir)
	if err != nil {
		log.Printf("Intel: failed to load content from %s: %v", contentDir, err)
		// Create empty service if content dir doesn't exist
		m.contentService, _ = services.NewContentService("/dev/null")
	}

	m.markdownService = services.NewMarkdownService()

	m.publicHandler = handlers.NewPublicHandler(
		m.contentService, m.markdownService, deps.Templates,
	)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func findContentDir() string {
	// Try relative paths first
	paths := []string{
		"content/intel",
		"./content/intel",
		"../content/intel",
	}

	// Also try from executable location
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(exeDir, "content/intel"))
		paths = append(paths, filepath.Join(exeDir, "..", "content/intel"))
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

	// Fallback - will be empty
	return "content/intel"
}

func (m *Module) Migrate() error {
	// No database migrations needed for file-based content
	return nil
}

func (m *Module) Routes() chi.Router {
	return m.publicHandler.Routes()
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	// No admin routes - content is managed via git
	return nil
}

func (m *Module) MenuItems() []module.MenuItem {
	// No admin menu items
	return nil
}

func (m *Module) GetLatestArticles(limit int) ([]interface{}, error) {
	articles := m.contentService.GetLatestArticles(limit)
	result := make([]interface{}, len(articles))
	for i, a := range articles {
		result[i] = map[string]interface{}{
			"Title":        a.Title,
			"Slug":         a.Slug,
			"Excerpt":      a.Excerpt,
			"CoverImage":   a.CoverImage,
			"ReadingTime":  a.ReadingTime,
			"CategoryName": a.CategoryName,
		}
	}
	return result, nil
}

// Reload reloads content from disk (useful after git pull)
func (m *Module) Reload() error {
	return m.contentService.Reload()
}
