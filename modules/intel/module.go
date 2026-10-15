package intel

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/intel/handlers"
	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	categoryService *services.CategoryService
	tagService      *services.TagService
	articleService  *services.ArticleService
	markdownService *services.MarkdownService
	adminHandler    *handlers.AdminHandler
	publicHandler   *handlers.PublicHandler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"intel",
			"Intel",
			"Blog and content management",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.categoryService = services.NewCategoryService(deps.DB)
	m.tagService = services.NewTagService(deps.DB)
	m.articleService = services.NewArticleService(deps.DB, m.tagService)
	m.markdownService = services.NewMarkdownService()

	m.adminHandler = handlers.NewAdminHandler(
		m.articleService, m.categoryService, m.tagService, m.markdownService, deps.Templates,
	)
	m.publicHandler = handlers.NewPublicHandler(
		m.articleService, m.categoryService, m.tagService, m.markdownService, deps.Templates,
	)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_intel_tables.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

func (m *Module) Routes() chi.Router {
	return m.publicHandler.Routes()
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		return m.adminHandler.Routes()
	}
	return nil
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Intel",
			Icon:    "bi-journal-text",
			Path:    "/admin/intel",
			Order:   50,
			Section: module.MenuSectionAdmin,
			Group:   "Content",
		},
	}
}

func (m *Module) GetLatestArticles(limit int) ([]interface{}, error) {
	articles, err := m.articleService.GetLatest(limit)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, len(articles))
	for i, a := range articles {
		result[i] = a
	}
	return result, nil
}
