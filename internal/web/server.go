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
	"net"
	"net/http"
	"net/url"
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

// pageData is the data for the index page. Result is nil until a search
// has run.
type pageData struct {
	Term   string
	Result *result
}

// result is the data for the result template. It holds either a joke or
// a message for the visitor.
type result struct {
	Joke    *dadjoke.Joke
	Message string
}

// New returns the app's HTTP handler, which gets its jokes from jokes and
// writes its log to logger. It parses all templates up front, so a broken
// template fails here instead of on the first request.
func New(jokes JokeSource, logger *slog.Logger) (http.Handler, error) {
	index, err := parsePage("index.html", "result.html")
	if err != nil {
		return nil, err
	}
	resultTmpl, err := template.ParseFS(templateFS, "templates/result.html")
	if err != nil {
		return nil, fmt.Errorf("parse result.html: %w", err)
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("open static files: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", pageHandler(logger, index))
	mux.Handle("GET /joke", jokeHandler(jokes, logger, index, resultTmpl))
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
func pageHandler(logger *slog.Logger, tmpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		render(w, logger, tmpl, "layout", http.StatusOK, pageData{})
	})
}

// jokeHandler answers GET /joke?term=... with a joke. A blank term gets
// a random joke. When no joke comes back, it answers with a message for
// the visitor and a status from failure. An HTMX request gets only the
// result fragment, and any other request gets the full index page.
func jokeHandler(jokes JokeSource, logger *slog.Logger, index, resultTmpl *template.Template) http.Handler {
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

		status, res := http.StatusOK, &result{Joke: &joke}
		if err != nil {
			// Nobody is left to read an answer to a cancelled request.
			if r.Context().Err() != nil {
				logger.Info("request cancelled", "term", term, "error", err)
				return
			}
			f := classify(err, term)
			if f.kind == "" {
				logger.Info("no joke matched", "term", term)
			} else {
				attrs := []any{"term", term, "kind", f.kind}
				var se *dadjoke.StatusError
				if errors.As(err, &se) {
					attrs = append(attrs, "status", se.StatusCode)
				}
				logger.Error("get joke", append(attrs, "error", err)...)
			}
			status, res = f.status, &result{Message: f.message}
		}

		if r.Header.Get("HX-Request") == "true" {
			render(w, logger, resultTmpl, "result", status, res)
			return
		}
		render(w, logger, index, "layout", status, pageData{Term: term, Result: res})
	})
}

// failure describes how the joke endpoint answers an error from the
// JokeSource.
type failure struct {
	status int
	// kind names the failure in the log. It is empty for no match, which
	// is not an API failure.
	kind    string
	message string
}

// classify maps an error from the JokeSource for a search for term to the
// answer for the visitor. The visitor never sees the error text itself.
func classify(err error, term string) failure {
	var netErr net.Error
	var urlErr *url.Error
	switch {
	case errors.Is(err, dadjoke.ErrNoMatch):
		return failure{http.StatusNotFound, "",
			fmt.Sprintf(`No joke matches "%s". Try another word, or leave the field empty for a random joke.`, term)}
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		return failure{http.StatusGatewayTimeout, "timeout",
			"The joke service took too long to answer. Please try again."}
	case errors.As(err, &urlErr):
		return failure{http.StatusBadGateway, "network",
			"Could not reach the joke service. Please try again in a moment."}
	default:
		// A *dadjoke.StatusError, or a response body that is not valid JSON.
		return failure{http.StatusBadGateway, "api_error",
			"The joke service is having trouble right now. Please try again in a moment."}
	}
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
