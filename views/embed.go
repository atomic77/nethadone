package views

import "embed"

//go:embed *.tpl
//go:embed **/*.tpl
var EmbedTemplates embed.FS
