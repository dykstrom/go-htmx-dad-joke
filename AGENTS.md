# AGENTS.md

## What is this

go-htmx-dad-joke is a web app that searches the icanhazdadjoke.com REST API for dad jokes and shows the result. The UI has one search field, a button, and an output area. An empty search returns a random joke. The repo has no code yet. `README.md` describes the planned behaviour.

## Stack

| Piece | Choice |
|-------|--------|
| Backend | Go |
| Frontend | HTMX 4, with HTML pages rendered on the server by Go's standard `html/template` package |
| External API | icanhazdadjoke.com REST API |
| Package manager | Go modules |

## Directory index

| Path | What's there |
|------|-------------|
| `docs/` | Durable project context. Sub-folder layout below shows where each kind of doc goes. |

```
docs/
├── system/         ← what the code does today (updated as code changes)
├── architecture/   ← what the system must do (updated when rules change)
├── adr/            ← architecture decisions (immutable once shipped)
├── reference/      ← long-form rationale (append-only)
└── working-notes/  ← research; NOT authoritative — rules live in architecture/ + adr/
```
