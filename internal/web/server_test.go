package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/dykstrom/go-htmx-dad-joke/internal/dadjoke"
)

// htmxSHA256 is the SHA-256 of htmx.min.js from htmx.org@4.0.0, as
// recorded in docs/working-notes/htmx-4.md.
const htmxSHA256 = "e484d9171a9db30a39c8f16e3d709d4137f3211c659f8e6125816635033d593f"

// fakeJokes is a JokeSource that returns joke and err and records the term
// of each call, so tests need no network.
type fakeJokes struct {
	joke string
	err  error

	mu       sync.Mutex
	searches []string
}

func (f *fakeJokes) Search(_ context.Context, term string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches = append(f.searches, term)
	return f.joke, f.err
}

// logBuffer collects log output. The server writes to it from its own
// goroutines, so it locks around each access.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newTestServer starts the app with jokes and returns the server and the
// buffer that its log goes to.
func newTestServer(t *testing.T, jokes JokeSource) (*httptest.Server, *logBuffer) {
	t.Helper()
	logs := &logBuffer{}
	handler, err := New(jokes, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, logs
}

func get(t *testing.T, srv *httptest.Server, path string) (*http.Response, []byte) {
	t.Helper()
	return getWithHeader(t, srv, path, nil)
}

// getHTMX sends a GET request with the header that HTMX sends.
func getHTMX(t *testing.T, srv *httptest.Server, path string) (*http.Response, []byte) {
	t.Helper()
	return getWithHeader(t, srv, path, http.Header{"HX-Request": {"true"}})
}

func getWithHeader(t *testing.T, srv *httptest.Server, path string, header http.Header) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("build request for %s: %v", path, err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body of %s: %v", path, err)
	}
	return resp, body
}

func TestIndexPage(t *testing.T) {
	srv, _ := newTestServer(t, &fakeJokes{})

	resp, body := get(t, srv, "/")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	for _, want := range []string{
		`<!doctype html>`,
		`<meta name="color-scheme" content="light dark">`,
		// app.css comes after Pico, so its rules win.
		"<link rel=\"stylesheet\" href=\"/static/pico.min.css\">\n    <link rel=\"stylesheet\" href=\"/static/app.css\">",
		`<script src="/static/htmx.min.js"></script>`,
		`<h1>Dad jokes</h1>`,
		"<search>\n<form ",
		`<form action="/joke" method="get" hx-get="/joke" hx-target="#result" hx-indicator="#spinner" hx-disable="#search" hx-on:htmx:finally:request="this.elements.term.focus()">`,
		`<label for="term">Search word</label>`,
		`<div role="group">`,
		`<input type="search" id="term" name="term" value="" autofocus>`,
		`<button type="submit" id="search">Search</button>`,
		"</form>\n</search>",
		`<p id="spinner" class="htmx-indicator" aria-busy="true">Fetching a joke…</p>`,
		`<div id="result" aria-live="polite"></div>`,
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestStylesheets(t *testing.T) {
	for _, name := range []string{"pico.min.css", "app.css"} {
		t.Run(name, func(t *testing.T) {
			srv, _ := newTestServer(t, &fakeJokes{})
			want, err := staticFS.ReadFile("static/" + name)
			if err != nil {
				t.Fatalf("read embedded file: %v", err)
			}

			resp, body := get(t, srv, "/static/"+name)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
				t.Errorf("Content-Type = %q, want text/css", ct)
			}
			if !bytes.Equal(body, want) {
				t.Errorf("body differs from the embedded file (%d bytes, want %d)", len(body), len(want))
			}
		})
	}
}

func TestHTMXScript(t *testing.T) {
	srv, _ := newTestServer(t, &fakeJokes{})
	want, err := staticFS.ReadFile("static/htmx.min.js")
	if err != nil {
		t.Fatalf("read embedded file: %v", err)
	}

	resp, body := get(t, srv, "/static/htmx.min.js")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	if !bytes.Equal(body, want) {
		t.Errorf("body differs from the embedded file (%d bytes, want %d)", len(body), len(want))
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != htmxSHA256 {
		t.Errorf("SHA-256 = %s, want %s (htmx.org@4.0.0)", got, htmxSHA256)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	srv, _ := newTestServer(t, &fakeJokes{})

	for _, path := range []string{"/nope", "/static/nope.css"} {
		resp, _ := get(t, srv, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusNotFound)
		}
	}
}

func TestJokeSearchFragment(t *testing.T) {
	jokes := &fakeJokes{joke: "A cat joke."}
	srv, _ := newTestServer(t, jokes)

	resp, body := getHTMX(t, srv, "/joke?term=cat")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := string(body); got != `<article class="joke"><p>A cat joke.</p></article>` {
		t.Errorf("body = %q, want only the result fragment", got)
	}
	if vary := resp.Header.Values("Vary"); !slices.Contains(vary, "HX-Request") {
		t.Errorf("Vary = %q, want HX-Request", vary)
	}
	if len(jokes.searches) != 1 || jokes.searches[0] != "cat" {
		t.Errorf("searches = %q, want one Search(cat)", jokes.searches)
	}
}

func TestJokeFullPage(t *testing.T) {
	srv, _ := newTestServer(t, &fakeJokes{joke: "A cat joke."})

	resp, body := get(t, srv, "/joke?term=cat")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	for _, want := range []string{
		`<!doctype html>`,
		`<input type="search" id="term" name="term" value="cat" autofocus>`,
		`<div id="result" aria-live="polite"><article class="joke"><p>A cat joke.</p></article></div>`,
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

// secret is in the text of the fake upstream error. A visitor must never
// see it.
const secret = "secret-upstream-detail"

// jokeErrorCase is an error from the JokeSource and the answer it gets.
type jokeErrorCase struct {
	name    string
	err     error
	status  int
	message string // the message in the body, as html/template escapes it
}

var jokeErrorCases = []jokeErrorCase{
	{
		name:    "no match",
		err:     dadjoke.ErrNoMatch,
		status:  http.StatusNotFound,
		message: "No joke matches &#34;cat&#34;. Try another word, or leave the field empty for a random joke.",
	},
	{
		name:    "upstream failure",
		err:     errors.New("dadjoke: GET /search: " + secret),
		status:  http.StatusBadGateway,
		message: "The joke service is having trouble right now. Please try again in a moment.",
	},
}

func TestJokeErrorFragment(t *testing.T) {
	for _, tc := range jokeErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newTestServer(t, &fakeJokes{err: tc.err})

			resp, body := getHTMX(t, srv, "/joke?term=cat")

			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			if want := `<p class="message">` + tc.message + `</p>`; string(body) != want {
				t.Errorf("body = %q, want only %q", body, want)
			}
			if vary := resp.Header.Values("Vary"); !slices.Contains(vary, "HX-Request") {
				t.Errorf("Vary = %q, want HX-Request", vary)
			}
		})
	}
}

func TestJokeErrorFullPage(t *testing.T) {
	for _, tc := range jokeErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newTestServer(t, &fakeJokes{err: tc.err})

			resp, body := get(t, srv, "/joke?term=cat")

			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			for _, want := range []string{
				`<!doctype html>`,
				`<input type="search" id="term" name="term" value="cat" autofocus>`,
				`<div id="result" aria-live="polite"><p class="message">` + tc.message + `</p></div>`,
			} {
				if !bytes.Contains(body, []byte(want)) {
					t.Errorf("body does not contain %q", want)
				}
			}
			if vary := resp.Header.Values("Vary"); !slices.Contains(vary, "HX-Request") {
				t.Errorf("Vary = %q, want HX-Request", vary)
			}
		})
	}
}

func TestJokeUpstreamFailureIsLoggedNotShown(t *testing.T) {
	srv, logs := newTestServer(t, &fakeJokes{err: jokeErrorCases[1].err})

	_, body := getHTMX(t, srv, "/joke?term=cat")

	if bytes.Contains(body, []byte(secret)) {
		t.Errorf("body contains the error text %q", secret)
	}
	if log := logs.String(); !strings.Contains(log, `level=ERROR msg="get joke" term=cat error=`) || !strings.Contains(log, secret) {
		t.Errorf("log = %q, want an ERROR line with the term and the error text", log)
	}
}

func TestJokeNoMatchEscapesTerm(t *testing.T) {
	srv, _ := newTestServer(t, &fakeJokes{err: dadjoke.ErrNoMatch})

	_, body := getHTMX(t, srv, "/joke?term="+url.QueryEscape("<b>"))

	if !bytes.Contains(body, []byte("No joke matches &#34;&lt;b&gt;&#34;.")) || bytes.Contains(body, []byte("<b>")) {
		t.Errorf("body = %q, want the term escaped", body)
	}
}
