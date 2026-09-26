# AGENTS.md

## What is this

go-htmx-dad-joke is a web app that searches the icanhazdadjoke.com REST API for dad jokes and shows the result. The UI has one search field, a button, and an output area. An empty search returns a random joke. The app is under construction. `README.md` describes the finished behaviour.

## Stack

| Piece | Choice |
|-------|--------|
| Backend | Go |
| Frontend | HTMX 4, with HTML pages rendered on the server by Go's standard `html/template` package |
| Styling | Pico CSS, vendored in `internal/web/static/` (ADR 0002) |
| External API | icanhazdadjoke.com REST API |
| Package manager | Go modules |

## Directory index

| Path | What's there |
|------|-------------|
| `cmd/server/` | The app's `main` package. Reads `PORT`, starts the server, and stops it on Ctrl-C |
| `internal/web/` | HTTP handlers and routes. Embeds `templates/` and `static/` with `go:embed` (ADR 0001) |
| `internal/web/templates/` | `html/template` files. `layout.html` is the page skeleton |
| `internal/web/static/` | Files served under `/static/`, including vendored third-party files |
| `docs/` | Durable project context. Sub-folder layout below shows where each kind of doc goes. |

```
docs/
├── system/         ← what the code does today (updated as code changes)
├── architecture/   ← what the system must do (updated when rules change)
├── adr/            ← architecture decisions (immutable once shipped)
├── reference/      ← long-form rationale (append-only)
└── working-notes/  ← research; NOT authoritative — rules live in architecture/ + adr/
```

## Commands

| What | Command |
|------|---------|
| Run the app | `go run ./cmd/server` (listens on `PORT`, default `8080`) |
