# Web — templates, static files, and styling

## Assets

- Templates MUST live in `internal/web/templates/` and static files in `internal/web/static/`, so that `go:embed` includes them in the binary.
- The app MUST NOT load scripts or stylesheets from a CDN, and MUST NOT read templates or static files from disk at runtime.
- Each third-party file MUST be vendored from a URL with an exact version, for example `htmx.org@4.0.0`. An unversioned HTMX URL gets HTMX 2.
- Source: [ADR 0001](../adr/0001-vendor-and-embed-assets.md).

## Styling

- Pages MUST load `pico.min.css` from `/static/`.
- Custom CSS MUST go on top of Pico and MUST NOT replace it.
- Source: [ADR 0002](../adr/0002-use-pico-css.md).

## HTMX endpoints

- An endpoint that HTMX calls MUST return only the HTML fragment when the request has `HX-Request: true`, and the full page otherwise.
- Such an endpoint MUST set `Vary: HX-Request` on every response.
- A form that sends an HTMX request MUST also have `action` and `method`, so that it works without JavaScript.
- Source: ticket 004 of this project, decided on 2026-10-15, so that a bookmarked URL or a browser without JavaScript still gets a working page.

## Error responses

- When an endpoint that HTMX calls cannot return its normal content, it MUST answer with an HTML fragment that holds a message for the visitor.
- The status MUST match the failure: 4xx when the request cannot succeed, such as a search with no match, and 5xx when a service the app calls fails.
- A response body MUST NOT contain the text of a Go error or of an answer from an upstream service. The server log MUST record those details instead.
- Source: ticket 005 of this project, decided on 2026-10-15. HTMX 4 swaps 4xx and 5xx responses into the target, so the visitor sees the message while logs and tools still see the failure.
