package dadjoke

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

// fakeAPI is a fake icanhazdadjoke.com. Its responses copy the JSON shapes
// that the live API returned on 2026-09-26.
type fakeAPI struct {
	t        *testing.T
	jokes    []string // jokes that match every search
	mu       sync.Mutex
	requests []*http.Request
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/":
		fmt.Fprint(w, `{"id":"R1","joke":"A random joke.","status":200}`)
	case "/search":
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		results := "[]"
		if page >= 1 && page <= len(f.jokes) {
			results = fmt.Sprintf(`[{"id":"J%d","joke":%q}]`, page, f.jokes[page-1])
		}
		fmt.Fprintf(w, `{"current_page":%d,"limit":1,"results":%s,"search_term":%q,"status":200,"total_jokes":%d}`,
			page, results, r.URL.Query().Get("term"), len(f.jokes))
	default:
		f.t.Errorf("unexpected request %s", r.URL)
		http.NotFound(w, r)
	}
}

func (f *fakeAPI) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var paths []string
	for _, r := range f.requests {
		paths = append(paths, r.URL.RequestURI())
	}
	return paths
}

func newFake(t *testing.T, matches int) (*fakeAPI, *Client) {
	t.Helper()
	fake := &fakeAPI{t: t}
	for i := range matches {
		fake.jokes = append(fake.jokes, fmt.Sprintf("Joke number %d.", i+1))
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, New(srv.URL)
}

func TestSearch(t *testing.T) {
	fake, c := newFake(t, 75)

	joke, err := c.Search(context.Background(), "cat")

	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	paths := fake.paths()
	if len(paths) != 2 || paths[0] != "/search?limit=1&page=1&term=cat" {
		t.Fatalf("requests = %v, want page 1 and then a random page", paths)
	}
	page := fake.requests[1].URL.Query().Get("page")
	if want := "Joke number " + page + "."; joke != want {
		t.Errorf("Search() = %q, want %q from the second request", joke, want)
	}
	for _, req := range fake.requests {
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if got := req.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
	}
}

func TestSearchBlankTermReturnsRandomJoke(t *testing.T) {
	for _, term := range []string{"", "   "} {
		t.Run(strconv.Quote(term), func(t *testing.T) {
			fake, c := newFake(t, 5)

			joke, err := c.Search(context.Background(), term)

			if err != nil {
				t.Fatalf("Search() error: %v", err)
			}
			if joke != "A random joke." {
				t.Errorf("Search() = %q, want the random joke", joke)
			}
			if got := fake.paths(); len(got) != 1 || got[0] != "/" {
				t.Errorf("requests = %v, want [/]", got)
			}
		})
	}
}

func TestSearchNoMatch(t *testing.T) {
	_, c := newFake(t, 0)

	_, err := c.Search(context.Background(), "xyzzyqqq")

	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("Search() error = %v, want ErrNoMatch", err)
	}
}

func TestSearchEmptyPage(t *testing.T) {
	// The jokes change between the two requests, so the picked page is empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[],"status":200,"total_jokes":5}`)
	}))
	t.Cleanup(srv.Close)

	_, err := New(srv.URL).Search(context.Background(), "cat")

	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("Search() error = %v, want ErrNoMatch", err)
	}
}

func TestSearchFails(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"HTTP error status": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
		"invalid JSON": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<html>not json</html>`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			for _, term := range []string{"", "cat"} {
				_, err := New(srv.URL).Search(context.Background(), term)
				if err == nil || errors.Is(err, ErrNoMatch) {
					t.Errorf("Search(%q) error = %v, want an API error", term, err)
				}
			}
		})
	}
}
