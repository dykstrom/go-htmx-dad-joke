package web

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string) (*http.Response, []byte) {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + path)
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
	srv := newTestServer(t)

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
		`<h1>Dad jokes</h1>`,
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestPicoStylesheet(t *testing.T) {
	srv := newTestServer(t)
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

func TestUnknownPathIsNotFound(t *testing.T) {
	srv := newTestServer(t)

	for _, path := range []string{"/nope", "/static/nope.css", "/static/"} {
		resp, _ := get(t, srv, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusNotFound)
		}
	}
}
