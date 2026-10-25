package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web
var embeddedWebFS embed.FS

// WebFS returns an http.FileSystem serving the embedded web application files.
func WebFS() http.FileSystem {
	sub, _ := fs.Sub(embeddedWebFS, "web")
	return http.FS(sub)
}
