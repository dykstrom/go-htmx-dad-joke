// Package web serves the dad joke web pages and their static files.
package web

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/dykstrom/go-htmx-dad-joke/internal/dadjoke"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// JokeSource returns dad jokes. *dadjoke.Client implements it.
type JokeSource interface {
	Search(ctx context.Context, term string) (string, error)
}

// page is the data for the templates. Joke and Message are empty until a
// search has run.
type page struct {
	Term    string
	Joke    string
	Message string
}

// New returns the app's HTTP handler, which gets its jokes from jokes and
// writes its log to logger. It parses all templates up front, so a broken
// template fails here instead of on the first request.
func New(jokes JokeSource, logger *slog.Logger) (http.Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("open static files: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, logger, tmpl, "layout", http.StatusOK, page{})
	})
	mux.Handle("GET /joke", jokeHandler(jokes, logger, tmpl))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return mux, nil
}

// jokeHandler answers GET /joke?term=... with a joke, or with a message for
// the visitor when no joke comes back. An HTMX request gets only the result
// fragment, and any other request gets the full index page.
func jokeHandler(jokes JokeSource, logger *slog.Logger, tmpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One URL returns either a fragment or a full page.
		w.Header().Add("Vary", "HX-Request")

		data := page{Term: r.URL.Query().Get("term")}
		status := http.StatusOK
		joke, err := jokes.Search(r.Context(), data.Term)
		switch {
		case err == nil:
			data.Joke = joke
		case errors.Is(err, dadjoke.ErrNoMatch):
			status = http.StatusNotFound
			data.Message = fmt.Sprintf(`No joke matches "%s". Try another word, or leave the field empty for a random joke.`, data.Term)
		default:
			// The visitor never sees the error text itself.
			logger.Error("get joke", "term", data.Term, "error", err)
			status = http.StatusBadGateway
			data.Message = "The joke service is having trouble right now. Please try again in a moment."
		}

		name := "layout"
		if r.Header.Get("HX-Request") == "true" {
			name = "result"
		}
		render(w, logger, tmpl, name, status, data)
	})
}

// render executes the named template into a buffer first, so a template
// error returns a 500 instead of a half-written page. Otherwise it answers
// with status.
func render(w http.ResponseWriter, logger *slog.Logger, tmpl *template.Template, name string, status int, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		logger.Error("render template", "name", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}
