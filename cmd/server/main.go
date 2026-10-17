// Command server runs the dad joke web app.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/dykstrom/go-htmx-dad-joke/internal/dadjoke"
	"github.com/dykstrom/go-htmx-dad-joke/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	handler, err := web.New(dadjoke.New(dadjoke.DefaultBaseURL), slog.Default())
	if err != nil {
		return err
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("server started", "addr", "http://localhost:"+port)
	return http.ListenAndServe(":"+port, handler)
}
