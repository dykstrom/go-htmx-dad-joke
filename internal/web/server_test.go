package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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

// fakeJokes is a JokeSource that returns joke and err and records each
// call, so tests need no network.
type fakeJokes struct {
	joke dadjoke.Joke
	err  error

	mu       sync.Mutex
	randoms  int
	searches []string // the term of each Search call
}

func (f *fakeJokes) Random(context.Context) (dadjoke.Joke, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.randoms++
	return f.joke, f.err
}

func (f *fakeJokes) Search(_ context.Context, term string) (dadjoke.Joke, error) {
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

// newTestHandler returns the app's handler and the buffer that its log
// goes to.
func newTestHandler(t *testing.T, jokes JokeSource) (http.Handler, *logBuffer) {
	t.Helper()
	logs := &logBuffer{}
	handler, err := New(jokes, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return handler, logs
}

func newTestServer(t *testing.T, jokes JokeSource) (*httptest.Server, *logBuffer) {
	t.Helper()
	handler, logs := newTestHandler(t, jokes)
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

	for _, path := range []string{"/nope", "/static/nope.css", "/static/"} {
		resp, _ := get(t, srv, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusNotFound)
		}
	}
}

func TestJokeSearchFragment(t *testing.T) {
	jokes := &fakeJokes{joke: dadjoke.Joke{ID: "J1", Text: "A cat joke."}}
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
	if len(jokes.searches) != 1 || jokes.searches[0] != "cat" || jokes.randoms != 0 {
		t.Errorf("searches = %q, randoms = %d, want one Search(cat)", jokes.searches, jokes.randoms)
	}
}

func TestJokeBlankTermIsRandom(t *testing.T) {
	for _, path := range []string{"/joke", "/joke?term=", "/joke?term=%20%20"} {
		t.Run(path, func(t *testing.T) {
			jokes := &fakeJokes{joke: dadjoke.Joke{ID: "R1", Text: "A random joke."}}
			srv, _ := newTestServer(t, jokes)

			resp, body := getHTMX(t, srv, path)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if !bytes.Contains(body, []byte("A random joke.")) {
				t.Errorf("body = %q, want the random joke", body)
			}
			if jokes.randoms != 1 || len(jokes.searches) != 0 {
				t.Errorf("randoms = %d, searches = %q, want one Random call", jokes.randoms, jokes.searches)
			}
		})
	}
}

func TestJokeFullPage(t *testing.T) {
	jokes := &fakeJokes{joke: dadjoke.Joke{ID: "J1", Text: "A cat joke."}}
	srv, _ := newTestServer(t, jokes)

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

// secret is in the text of every fake error. A visitor must never see it.
const secret = "secret-upstream-detail"

// timeoutError is the kind of error that http.Client wraps when its
// timeout runs out.
type timeoutError struct{}

func (timeoutError) Error() string   { return secret + ": Client.Timeout exceeded" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// jokeErrorCase is an error from the JokeSource and the answer it gets.
type jokeErrorCase struct {
	name    string
	err     error
	status  int
	message string   // the message in the body, as html/template escapes it
	log     []string // text that the log must contain
}

// jokeErrorCases returns one case per kind of error, in the shapes that
// dadjoke.Client returns.
func jokeErrorCases() []jokeErrorCase {
	syntaxErr := json.Unmarshal([]byte("<html>"), new(any))
	return []jokeErrorCase{
		{
			name:    "no match",
			err:     dadjoke.ErrNoMatch,
			status:  http.StatusNotFound,
			message: "No joke matches &#34;cat&#34;. Try another word, or leave the field empty for a random joke.",
			log:     []string{`level=INFO msg="no joke matched" term=cat`},
		},
		{
			name:    "API error status",
			err:     &dadjoke.StatusError{StatusCode: http.StatusServiceUnavailable, Message: secret},
			status:  http.StatusBadGateway,
			message: "The joke service is having trouble right now. Please try again in a moment.",
			log:     []string{`level=ERROR msg="get joke" term=cat kind=api_error status=503 error=`, secret},
		},
		{
			name:    "invalid JSON",
			err:     fmt.Errorf("dadjoke: decode response from /search (%s): %w", secret, syntaxErr),
			status:  http.StatusBadGateway,
			message: "The joke service is having trouble right now. Please try again in a moment.",
			log:     []string{`level=ERROR msg="get joke" term=cat kind=api_error error=`, secret},
		},
		{
			name: "network failure",
			err: fmt.Errorf("dadjoke: GET /search: %w", &url.Error{
				Op:  "Get",
				URL: "https://" + secret + "/search",
				Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			}),
			status:  http.StatusBadGateway,
			message: "Could not reach the joke service. Please try again in a moment.",
			log:     []string{`level=ERROR msg="get joke" term=cat kind=network error=`, secret},
		},
		{
			name:    "timeout",
			err:     fmt.Errorf("dadjoke: GET /search: %w", &url.Error{Op: "Get", URL: "https://example.com/search", Err: timeoutError{}}),
			status:  http.StatusGatewayTimeout,
			message: "The joke service took too long to answer. Please try again.",
			log:     []string{`level=ERROR msg="get joke" term=cat kind=timeout error=`, secret},
		},
	}
}

func TestJokeErrorFragment(t *testing.T) {
	for _, tc := range jokeErrorCases() {
		t.Run(tc.name, func(t *testing.T) {
			srv, logs := newTestServer(t, &fakeJokes{err: tc.err})

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
			checkLog(t, logs.String(), tc.status != http.StatusNotFound, tc.log)
		})
	}
}

func TestJokeErrorFullPage(t *testing.T) {
	for _, tc := range jokeErrorCases() {
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
			if bytes.Contains(body, []byte(secret)) {
				t.Errorf("body contains the error text %q", secret)
			}
			if vary := resp.Header.Values("Vary"); !slices.Contains(vary, "HX-Request") {
				t.Errorf("Vary = %q, want HX-Request", vary)
			}
		})
	}
}

// checkLog checks that log has one ERROR line if wantError is set and none
// otherwise, and that it contains each string in want.
func checkLog(t *testing.T, log string, wantError bool, want []string) {
	t.Helper()
	errorLines := strings.Count(log, "level=ERROR")
	if wantError && errorLines != 1 || !wantError && errorLines != 0 {
		t.Errorf("log has %d ERROR lines, want error = %v:\n%s", errorLines, wantError, log)
	}
	for _, w := range want {
		if !strings.Contains(log, w) {
			t.Errorf("log does not contain %q:\n%s", w, log)
		}
	}
}

func TestJokeNoMatchEscapesTerm(t *testing.T) {
	srv, _ := newTestServer(t, &fakeJokes{err: dadjoke.ErrNoMatch})

	_, body := getHTMX(t, srv, "/joke?term="+url.QueryEscape("<b>"))

	if !bytes.Contains(body, []byte("No joke matches &#34;&lt;b&gt;&#34;.")) || bytes.Contains(body, []byte("<b>")) {
		t.Errorf("body = %q, want the term escaped", body)
	}
}

func TestJokeCancelledRequest(t *testing.T) {
	handler, logs := newTestHandler(t, &fakeJokes{err: context.Canceled})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/joke?term=cat", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none", rec.Body)
	}
	checkLog(t, logs.String(), false, []string{`level=INFO msg="request cancelled" term=cat`})
}
