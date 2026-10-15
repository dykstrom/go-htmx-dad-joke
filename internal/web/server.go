// Package web serves the dad joke web pages and their static files.
package web

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dykstrom/go-htmx-dad-joke/internal/dadjoke"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// JokeSource returns dad jokes. *dadjoke.Client implements it.
type JokeSource interface {
	Random(ctx context.Context) (dadjoke.Joke, error)
	Search(ctx context.Context, term string) (dadjoke.Joke, error)
}

// pageData is the data for the index page. Joke is nil until a search
// has found one.
type pageData struct {
	Term string
	Joke *dadjoke.Joke
}

// New returns the app's HTTP handler, which gets its jokes from jokes.
// It parses all templates up front, so a broken template fails here
// instead of on the first request.
func New(jokes JokeSource) (http.Handler, error) {
	index, err := parsePage("index.html", "result.html")
	if err != nil {
		return nil, err
	}
	result, err := template.ParseFS(templateFS, "templates/result.html")
	if err != nil {
		return nil, fmt.Errorf("parse result.html: %w", err)
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("open static files: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", pageHandler(index))
	mux.Handle("GET /joke", jokeHandler(jokes, index, result))
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

// parsePage parses the layout together with one page and the partials
// that the page uses. Each page gets its own template set, so pages can
// define the same blocks without clashing.
func parsePage(page string, partials ...string) (*template.Template, error) {
	files := []string{"templates/layout.html", "templates/" + page}
	for _, p := range partials {
		files = append(files, "templates/"+p)
	}
	tmpl, err := template.ParseFS(templateFS, files...)
	if err != nil {
		return nil, fmt.Errorf("parse page %s: %w", page, err)
	}
	return tmpl, nil
}

// pageHandler renders a page parsed by parsePage with empty page data.
func pageHandler(tmpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		render(w, tmpl, "layout", pageData{})
	})
}

// jokeHandler answers GET /joke?term=... with a joke. A blank term gets
// a random joke. An HTMX request gets only the result fragment, and any
// other request gets the full index page.
func jokeHandler(jokes JokeSource, index, result *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One URL returns either a fragment or a full page.
		w.Header().Add("Vary", "HX-Request")

		term := strings.TrimSpace(r.URL.Query().Get("term"))
		var joke dadjoke.Joke
		var err error
		if term == "" {
			joke, err = jokes.Random(r.Context())
		} else {
			joke, err = jokes.Search(r.Context(), term)
		}
		if err != nil {
			slog.Error("get joke", "term", term, "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if r.Header.Get("HX-Request") == "true" {
			render(w, result, "result", joke)
			return
		}
		render(w, index, "layout", pageData{Term: term, Joke: &joke})
	})
}

// render executes the named template into a buffer first, so a template
// error returns a 500 instead of a half-written page.
func render(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("render template", "name", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
