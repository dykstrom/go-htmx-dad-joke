package dadjoke

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// TestLive calls the real icanhazdadjoke.com API. It runs only when
// DADJOKE_LIVE=1 is set, so normal test runs and CI need no network.
func TestLive(t *testing.T) {
	if os.Getenv("DADJOKE_LIVE") != "1" {
		t.Skip("set DADJOKE_LIVE=1 to call the real API")
	}
	c := New(DefaultBaseURL)
	ctx := context.Background()

	t.Run("Random", func(t *testing.T) {
		joke, err := c.Random(ctx)
		if err != nil || joke.ID == "" || joke.Text == "" {
			t.Fatalf("Random() = %+v, %v", joke, err)
		}
		t.Logf("random: %s", joke.Text)
	})

	t.Run("SearchEmpty", func(t *testing.T) {
		joke, err := c.Search(ctx, "")
		if err != nil || joke.Text == "" {
			t.Fatalf("Search(\"\") = %+v, %v", joke, err)
		}
		t.Logf("empty search: %s", joke.Text)
	})

	t.Run("SearchCat", func(t *testing.T) {
		joke, err := c.Search(ctx, "cat")
		if err != nil {
			t.Fatalf("Search(cat) error: %v", err)
		}
		if !strings.Contains(strings.ToLower(joke.Text), "cat") {
			t.Errorf("Search(cat) = %q, want a joke that contains cat", joke.Text)
		}
		t.Logf("cat: %s", joke.Text)
	})

	t.Run("SearchNoMatch", func(t *testing.T) {
		_, err := c.Search(ctx, "xyzzyqqq")
		if !errors.Is(err, ErrNoMatch) {
			t.Errorf("Search(xyzzyqqq) error = %v, want ErrNoMatch", err)
		}
	})
}
