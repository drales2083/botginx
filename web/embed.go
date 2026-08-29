package web

import "embed"

//go:embed static
var StaticFS embed.FS

//go:embed templates/layouts/*.html
var LayoutsFS embed.FS

//go:embed templates/errors/*.html
var ErrorsFS embed.FS
