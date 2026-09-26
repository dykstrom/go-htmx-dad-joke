// Package web serves the dad joke web pages and their static files.
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// New returns the app's HTTP handler. It parses all templates up front,
// so a broken template fails here instead of on the first request.
func New() (http.Handler, error) {
	index, err := parsePage("index.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("open static files: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", pageHandler(index))
	mux.Handle("GET /static/", http.StripPrefix("/static/", noDirListing(http.FileServerFS(static))))
	return mux, nil
}

// noDirListing returns 404 for folder paths, which http.FileServerFS
// would otherwise answer with a listing of the folder.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parsePage parses the layout together with one page. Each page gets its
// own template set, so pages can define the same blocks without clashing.
func parsePage(page string) (*template.Template, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+page)
	if err != nil {
		return nil, fmt.Errorf("parse page %s: %w", page, err)
	}
	return tmpl, nil
}

// pageHandler renders a page parsed by parsePage. It renders into a buffer
// first, so a template error returns a 500 instead of a half-written page.
func pageHandler(tmpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "layout", nil); err != nil {
			slog.Error("render page", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		buf.WriteTo(w)
	})
}
