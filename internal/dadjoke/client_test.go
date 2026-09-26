package dadjoke

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// fakeAPI is a fake icanhazdadjoke.com. Its responses copy the JSON shapes
// that the live API returned on 2026-09-26.
type fakeAPI struct {
	t        *testing.T
	jokes    []string // jokes that match every search
	mu       sync.Mutex
	requests []*http.Request
	handler  http.HandlerFunc // replaces the default behaviour when set
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.mu.Unlock()
	if f.handler != nil {
		f.handler(w, r)
		return
	}
	f.serveDefault(w, r)
}

// serveDefault answers like the live API for the jokes in f.jokes.
func (f *fakeAPI) serveDefault(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/":
		fmt.Fprint(w, `{"id":"R1","joke":"A random joke.","status":200}`)
	case "/search":
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		start := (page - 1) * limit
		end := min(start+limit, len(f.jokes))
		results := "[]"
		if start < len(f.jokes) {
			results = "["
			for i := start; i < end; i++ {
				if i > start {
					results += ","
				}
				results += fmt.Sprintf(`{"id":"J%d","joke":%q}`, i, f.jokes[i])
			}
			results += "]"
		}
		fmt.Fprintf(w, `{"current_page":%d,"limit":%d,"results":%s,"search_term":%q,"status":200,"total_jokes":%d}`,
			page, limit, results, r.URL.Query().Get("term"), len(f.jokes))
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
		fake.jokes = append(fake.jokes, fmt.Sprintf("Joke number %d.", i))
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, New(srv.URL)
}

func TestRandom(t *testing.T) {
	fake, c := newFake(t, 0)

	joke, err := c.Random(context.Background())

	if err != nil {
		t.Fatalf("Random() error: %v", err)
	}
	if joke != (Joke{ID: "R1", Text: "A random joke."}) {
		t.Errorf("Random() = %+v", joke)
	}
	req := fake.requests[0]
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
	if got := req.Header.Get("User-Agent"); got != userAgent {
		t.Errorf("User-Agent = %q, want %q", got, userAgent)
	}
}

func TestSearchAllMatchesOnFirstPage(t *testing.T) {
	fake, c := newFake(t, 11)
	c.randIntN = func(n int) int {
		if n != 11 {
			t.Errorf("randIntN(%d), want randIntN(11)", n)
		}
		return 7
	}

	joke, err := c.Search(context.Background(), "cat")

	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if joke != (Joke{ID: "J7", Text: "Joke number 7."}) {
		t.Errorf("Search() = %+v, want J7", joke)
	}
	if got := fake.paths(); len(got) != 1 || got[0] != "/search?limit=30&page=1&term=cat" {
		t.Errorf("requests = %v, want one request for page 1", got)
	}
}

func TestSearchPicksFromLaterPage(t *testing.T) {
	fake, c := newFake(t, 75)
	c.randIntN = func(n int) int {
		if n != 75 {
			t.Errorf("randIntN(%d), want randIntN(75) over all matches", n)
		}
		return 64 // page 3, position 4
	}

	joke, err := c.Search(context.Background(), "a")

	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if joke != (Joke{ID: "J64", Text: "Joke number 64."}) {
		t.Errorf("Search() = %+v, want J64", joke)
	}
	want := []string{"/search?limit=30&page=1&term=a", "/search?limit=30&page=3&term=a"}
	if got := fake.paths(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

func TestSearchPickOnFirstPageNeedsNoSecondRequest(t *testing.T) {
	fake, c := newFake(t, 75)
	c.randIntN = func(int) int { return 12 }

	joke, err := c.Search(context.Background(), "a")

	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if joke.ID != "J12" {
		t.Errorf("Search() = %+v, want J12", joke)
	}
	if got := fake.paths(); len(got) != 1 {
		t.Errorf("requests = %v, want one", got)
	}
}

func TestSearchShortLaterPageFallsBackToFirstPage(t *testing.T) {
	fake, c := newFake(t, 75)
	calls := 0
	c.randIntN = func(n int) int {
		calls++
		if calls == 1 {
			return 64
		}
		return 2
	}
	// The later page comes back empty, as if the jokes changed.
	fake.handler = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "3" {
			fmt.Fprint(w, `{"results":[],"status":200,"total_jokes":75}`)
			return
		}
		fake.serveDefault(w, r)
	}

	joke, err := c.Search(context.Background(), "a")

	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if joke.ID != "J2" {
		t.Errorf("Search() = %+v, want J2 from page 1", joke)
	}
}

func TestSearchNoMatch(t *testing.T) {
	_, c := newFake(t, 0)

	_, err := c.Search(context.Background(), "xyzzyqqq")

	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("Search() error = %v, want ErrNoMatch", err)
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
			if joke.ID != "R1" {
				t.Errorf("Search() = %+v, want the random joke R1", joke)
			}
			if got := fake.paths(); len(got) != 1 || got[0] != "/" {
				t.Errorf("requests = %v, want [/]", got)
			}
		})
	}
}

func TestHTTPErrorStatus(t *testing.T) {
	fake, c := newFake(t, 0)
	fake.handler = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}

	_, err := c.Random(context.Background())

	var se *StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusInternalServerError {
		t.Errorf("Random() error = %v, want StatusError 500", err)
	}
}

func TestErrorStatusInBody(t *testing.T) {
	fake, c := newFake(t, 0)
	fake.handler = func(w http.ResponseWriter, r *http.Request) {
		// The live API answers like this with HTTP 200.
		fmt.Fprint(w, `{"message":"Joke with id \"nope\" not found","status":404}`)
	}

	_, err := c.Search(context.Background(), "cat")

	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("Search() error = %v, want StatusError", err)
	}
	if se.StatusCode != http.StatusNotFound || se.Message != `Joke with id "nope" not found` {
		t.Errorf("StatusError = %+v", se)
	}
}

func TestInvalidJSON(t *testing.T) {
	fake, c := newFake(t, 0)
	fake.handler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html>not json</html>`)
	}

	_, err := c.Random(context.Background())

	var se *StatusError
	if err == nil || errors.As(err, &se) {
		t.Errorf("Random() error = %v, want a decode error", err)
	}
}

func TestTimeout(t *testing.T) {
	fake, c := newFake(t, 0)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	fake.handler = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}
	c.httpClient.Timeout = 50 * time.Millisecond

	_, err := c.Random(context.Background())

	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Errorf("Random() error = %v, want a timeout", err)
	}
}

func TestCancelledContext(t *testing.T) {
	_, c := newFake(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Random(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Random() error = %v, want context.Canceled", err)
	}
}
