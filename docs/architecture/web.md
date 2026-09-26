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
