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

	t.Run("SearchEmpty", func(t *testing.T) {
		joke, err := c.Search(ctx, "")
		if err != nil || joke == "" {
			t.Fatalf("Search(\"\") = %q, %v", joke, err)
		}
		t.Logf("empty search: %s", joke)
	})

	t.Run("SearchCat", func(t *testing.T) {
		joke, err := c.Search(ctx, "cat")
		if err != nil {
			t.Fatalf("Search(cat) error: %v", err)
		}
		if !strings.Contains(strings.ToLower(joke), "cat") {
			t.Errorf("Search(cat) = %q, want a joke that contains cat", joke)
		}
		t.Logf("cat: %s", joke)
	})

	t.Run("SearchNoMatch", func(t *testing.T) {
		_, err := c.Search(ctx, "xyzzyqqq")
		if !errors.Is(err, ErrNoMatch) {
			t.Errorf("Search(xyzzyqqq) error = %v, want ErrNoMatch", err)
		}
	})
}
