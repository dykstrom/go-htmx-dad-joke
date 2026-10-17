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

// userAgent names the app, as the API documentation asks.
const userAgent = "go-htmx-dad-joke (https://github.com/dykstrom/go-htmx-dad-joke)"

// ErrNoMatch is returned by Search when no joke matches the term.
var ErrNoMatch = errors.New("dadjoke: no joke matches the search term")

// Client calls the icanhazdadjoke.com API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New returns a client for the API at baseURL, usually DefaultBaseURL.
// Each request times out after 5 seconds.
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

// Search returns a random joke that matches term, or a random joke about
// anything if term is blank. It returns ErrNoMatch when no joke matches.
func (c *Client) Search(ctx context.Context, term string) (string, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		var body struct {
			Joke string `json:"joke"`
		}
		err := c.get(ctx, "/", &body)
		return body.Joke, err
	}

	// The first request finds out how many jokes match. With limit=1,
	// page n holds joke n, so the second request fetches a random one.
	first, err := c.searchPage(ctx, term, 1)
	if err != nil {
		return "", err
	}
	if first.TotalJokes == 0 {
		return "", ErrNoMatch
	}
	page, err := c.searchPage(ctx, term, rand.IntN(first.TotalJokes)+1)
	if err != nil {
		return "", err
	}
	// The page is empty if the jokes changed between the two requests.
	if len(page.Results) == 0 {
		return "", ErrNoMatch
	}
	return page.Results[0].Joke, nil
}

type searchResponse struct {
	TotalJokes int `json:"total_jokes"`
	Results    []struct {
		Joke string `json:"joke"`
	} `json:"results"`
}

func (c *Client) searchPage(ctx context.Context, term string, page int) (searchResponse, error) {
	query := url.Values{"term": {term}, "limit": {"1"}, "page": {strconv.Itoa(page)}}
	var body searchResponse
	err := c.get(ctx, "/search?"+query.Encode(), &body)
	return body, err
}

// get sends a GET request and decodes the JSON body into dst.
func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dadjoke: GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("dadjoke: decode response from %s: %w", path, err)
	}
	return nil
}
