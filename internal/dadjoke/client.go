// Package dadjoke is a client for the icanhazdadjoke.com REST API.
package dadjoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the address of the public icanhazdadjoke.com API.
const DefaultBaseURL = "https://icanhazdadjoke.com"

const (
	// userAgent names the app, as the API documentation asks.
	userAgent = "go-htmx-dad-joke (https://github.com/dykstrom/go-htmx-dad-joke)"
	timeout   = 5 * time.Second
	// pageSize is the largest limit the API accepts. It caps larger
	// values to 30 without an error.
	pageSize = 30
)

// ErrNoMatch is returned by Search when no joke matches the term.
var ErrNoMatch = errors.New("dadjoke: no joke matches the search term")

// Joke is one dad joke.
type Joke struct {
	ID   string
	Text string
}

// StatusError is returned when the API answers with a status other than
// 200, either as the HTTP status or as the status field in the body.
type StatusError struct {
	StatusCode int
	Message    string
}

func (e *StatusError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("dadjoke: API returned status %d", e.StatusCode)
	}
	return fmt.Sprintf("dadjoke: API returned status %d: %s", e.StatusCode, e.Message)
}

// Client calls the icanhazdadjoke.com API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	// randIntN returns a random number in [0, n). Tests replace it.
	randIntN func(n int) int
}

// New returns a client for the API at baseURL, usually DefaultBaseURL.
// Each request times out after 5 seconds.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		randIntN:   rand.IntN,
	}
}

// Random returns a random joke.
func (c *Client) Random(ctx context.Context) (Joke, error) {
	var body struct {
		envelope
		ID   string `json:"id"`
		Joke string `json:"joke"`
	}
	if err := c.get(ctx, "/", nil, &body); err != nil {
		return Joke{}, err
	}
	return Joke{ID: body.ID, Text: body.Joke}, nil
}

// Search returns a random joke from all jokes that match term. For an
// empty or blank term it returns a random joke, like Random. It returns
// ErrNoMatch when no joke matches.
func (c *Client) Search(ctx context.Context, term string) (Joke, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return c.Random(ctx)
	}

	first, err := c.searchPage(ctx, term, 1)
	if err != nil {
		return Joke{}, err
	}
	if first.TotalJokes == 0 || len(first.Results) == 0 {
		return Joke{}, ErrNoMatch
	}
	if first.TotalJokes <= len(first.Results) {
		return c.pick(first.Results), nil
	}

	// Pick from all matches, and fetch the page that holds the pick.
	i := c.randIntN(first.TotalJokes)
	if i < len(first.Results) {
		return first.Results[i].joke(), nil
	}
	page, err := c.searchPage(ctx, term, i/pageSize+1)
	if err != nil {
		return Joke{}, err
	}
	if pos := i % pageSize; pos < len(page.Results) {
		return page.Results[pos].joke(), nil
	}
	// The page came back shorter than expected, for example because the
	// jokes changed between the two requests.
	return c.pick(first.Results), nil
}

type searchResult struct {
	ID   string `json:"id"`
	Joke string `json:"joke"`
}

func (r searchResult) joke() Joke { return Joke{ID: r.ID, Text: r.Joke} }

type searchResponse struct {
	envelope
	Results    []searchResult `json:"results"`
	TotalJokes int            `json:"total_jokes"`
}

func (c *Client) searchPage(ctx context.Context, term string, page int) (searchResponse, error) {
	query := url.Values{
		"term":  {term},
		"limit": {strconv.Itoa(pageSize)},
		"page":  {strconv.Itoa(page)},
	}
	var body searchResponse
	err := c.get(ctx, "/search", query, &body)
	return body, err
}

func (c *Client) pick(results []searchResult) Joke {
	return results[c.randIntN(len(results))].joke()
}

// envelope holds the fields that every API response body has.
type envelope struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e *envelope) fields() *envelope { return e }

// responseBody is a response struct that embeds envelope.
type responseBody interface{ fields() *envelope }

// get sends a GET request and decodes the JSON body into dst. The API can
// report an error as HTTP 200 with a status field in the body, so get
// checks that field after decoding.
func (c *Client) get(ctx context.Context, path string, query url.Values, dst responseBody) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("dadjoke: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dadjoke: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &StatusError{StatusCode: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("dadjoke: decode response from %s: %w", path, err)
	}
	if env := dst.fields(); env.Status != http.StatusOK {
		return &StatusError{StatusCode: env.Status, Message: env.Message}
	}
	return nil
}
