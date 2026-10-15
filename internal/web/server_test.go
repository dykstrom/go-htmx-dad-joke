package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

func newTestServer(t *testing.T, jokes JokeSource) *httptest.Server {
	t.Helper()
	handler, err := New(jokes)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
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
	srv := newTestServer(t, &fakeJokes{})

	resp, body := get(t, srv, "/")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	for _, want := range []string{
		`<!doctype html>`,
		`<link rel="stylesheet" href="/static/pico.min.css">`,
		`<script src="/static/htmx.min.js"></script>`,
		`<h1>Dad jokes</h1>`,
		`<form action="/joke" method="get" hx-get="/joke" hx-target="#result">`,
		`<input type="search" id="term" name="term" value="">`,
		`<button type="submit">Search</button>`,
		`<div id="result"></div>`,
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestPicoStylesheet(t *testing.T) {
	srv := newTestServer(t, &fakeJokes{})
	want, err := staticFS.ReadFile("static/pico.min.css")
	if err != nil {
		t.Fatalf("read embedded file: %v", err)
	}

	resp, body := get(t, srv, "/static/pico.min.css")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
	if !bytes.Equal(body, want) {
		t.Errorf("body differs from the embedded file (%d bytes, want %d)", len(body), len(want))
	}
}

func TestHTMXScript(t *testing.T) {
	srv := newTestServer(t, &fakeJokes{})
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
	srv := newTestServer(t, &fakeJokes{})

	for _, path := range []string{"/nope", "/static/nope.css", "/static/"} {
		resp, _ := get(t, srv, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusNotFound)
		}
	}
}

func TestJokeSearchFragment(t *testing.T) {
	jokes := &fakeJokes{joke: dadjoke.Joke{ID: "J1", Text: "A cat joke."}}
	srv := newTestServer(t, jokes)

	resp, body := getHTMX(t, srv, "/joke?term=cat")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := string(body); got != "<p>A cat joke.</p>" {
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
			srv := newTestServer(t, jokes)

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
	srv := newTestServer(t, jokes)

	resp, body := get(t, srv, "/joke?term=cat")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	for _, want := range []string{
		`<!doctype html>`,
		`<input type="search" id="term" name="term" value="cat">`,
		`<div id="result"><p>A cat joke.</p></div>`,
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestJokeErrorIs500(t *testing.T) {
	for _, err := range []error{dadjoke.ErrNoMatch, errors.New("boom")} {
		t.Run(err.Error(), func(t *testing.T) {
			srv := newTestServer(t, &fakeJokes{err: err})

			resp, _ := getHTMX(t, srv, "/joke?term=cat")

			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
			}
		})
	}
}
