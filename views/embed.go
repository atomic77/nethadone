package views

import "embed"

//go:generate go tool templ generate

// Static holds the vendored Pico.css, HTMX and the favicon, served under /static/.
//
//go:embed static
var Static embed.FS
