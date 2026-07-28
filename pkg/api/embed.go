package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web
var embeddedWebFS embed.FS

// WebFS returns an http.FileSystem serving the embedded web application files.
func WebFS() (http.FileSystem, error) {
	sub, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}
