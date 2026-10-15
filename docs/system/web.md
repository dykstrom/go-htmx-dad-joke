# Web package

`web.New` in `internal/web/server.go` builds the app's `http.Handler`. It takes a `JokeSource`, an interface with `Random` and `Search` that `*dadjoke.Client` implements. Handler tests pass `fakeJokes` from `server_test.go` instead, so they need no network. `web.New` parses every template when the app starts, so a template error stops startup instead of failing on the first request.

Each page has its own template set. `parsePage` parses `layout.html`, one page file, and the partial files that the page uses. The page file defines the `main` block and can define `title`. A new page needs its own `parsePage` call and a route in `New`. Pages cannot share one template set, because every page defines `main`, and the last page parsed would replace the others without an error.

## The joke endpoint

`GET /joke?term=...` calls `Random` for a blank term and `Search` otherwise. A request with the header `HX-Request: true` gets only the `result` template. Any other request gets the full index page, with the term in the search field and the joke in `#result`. `New` parses `result.html` into the index page set and also on its own, so both answers render the joke with the same template. Every answer sets `Vary: HX-Request`. On any error from the `JokeSource`, the handler logs the error and the term, and answers 500 with plain text.

## Static files

`GET /static/` serves the embedded `static/` folder with `http.FileServerFS`. The `noDirListing` wrapper returns 404 for folder paths such as `/static/`, which `http.FileServerFS` would otherwise answer with a file listing.

`TestHTMXScript` compares the SHA-256 of `static/htmx.min.js` with the `htmxSHA256` constant in `server_test.go`. An update of HTMX must change both the file and the constant.
