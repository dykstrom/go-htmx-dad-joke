# Web package

`web.New` in `internal/web/server.go` builds the app's `http.Handler`. It parses every template when the app starts, so a template error stops startup instead of failing on the first request.

Each page has its own template set. `parsePage` parses `layout.html` together with one page file. The page file defines the `main` block and can define `title`. A new page needs its own `parsePage` call and a route in `New`. Pages cannot share one template set, because every page defines `main`, and the last page parsed would replace the others without an error.

`GET /static/` serves the embedded `static/` folder with `http.FileServerFS`. The `noDirListing` wrapper returns 404 for folder paths such as `/static/`, which `http.FileServerFS` would otherwise answer with a file listing.
