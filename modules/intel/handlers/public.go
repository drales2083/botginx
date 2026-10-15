package handlers

import (
	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type PublicHandler struct{}

func NewPublicHandler(
	articles *services.ArticleService,
	categories *services.CategoryService,
	tags *services.TagService,
	markdown *services.MarkdownService,
	templates *module.TemplateEngine,
) *PublicHandler {
	return &PublicHandler{}
}

func (h *PublicHandler) Routes() chi.Router {
	return chi.NewRouter()
}
