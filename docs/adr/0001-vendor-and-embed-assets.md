# 0001. Vendor and embed all assets in the binary

*2026-09-26*

## Context

The app is a Go server that renders HTML with `html/template` and needs HTMX 4, Pico CSS, and its own templates at runtime. It runs only on the developer's machine for demos and is never deployed. One alternative was to load HTMX from a CDN such as jsDelivr. That keeps less in the repo, but a URL without an exact version gets a different release, and npm's `latest` tag still points to HTMX 2. The other alternative was to read templates and static files from disk at runtime. That allows edit and refresh without a restart, but the app must then start from the repo root.

## Decision

We will keep pinned copies of all third-party files in the repo and embed them, together with the templates, in the binary with `go:embed`. The files live under `internal/web/templates/` and `internal/web/static/`.

## Consequences

The app starts from any folder, and every page gets exactly the versions in the repo. The app needs no network access for its own files. A change to a template or a stylesheet needs a server restart. An update of HTMX or Pico means a download of the new file into `internal/web/static/` and a commit. New templates and static files must go under `internal/web/`, because `go:embed` cannot reach files outside the package folder.
