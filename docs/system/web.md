# Web package

`web.New` in `internal/web/server.go` builds the app's `http.Handler`. It takes a `JokeSource`, an interface with `Random` and `Search` that `*dadjoke.Client` implements, and a `*slog.Logger`. The handlers log through that logger, not through the global `slog` functions. Handler tests pass `fakeJokes` from `server_test.go`, so they need no network. `newTestServer` also returns a `logBuffer`, so tests can check log lines. `web.New` parses every template when the app starts, so a template error stops startup instead of failing on the first request.

Each page has its own template set. `parsePage` parses `layout.html`, one page file, and the partial files that the page uses. The page file defines the `main` block and can define `title`. A new page needs its own `parsePage` call and a route in `New`. Pages cannot share one template set, because every page defines `main`, and the last page parsed would replace the others without an error.

## The joke endpoint

`GET /joke?term=...` calls `Random` for a blank term and `Search` otherwise. A request with the header `HX-Request: true` gets only the `result` template. Any other request gets the full index page, with the term in the search field and the result in `#result`. `New` parses `result.html` into the index page set and also on its own, so both answers render the result with the same template. Every answer sets `Vary: HX-Request`.

When the `JokeSource` returns an error, `classify` picks the status and the message. The `result` template renders the message in place of a joke. The visitor never sees the error text.

| Error from the `JokeSource` | Status | Log |
|-----------------------------|--------|-----|
| `dadjoke.ErrNoMatch` | 404 | INFO `no joke matched` with `term` |
| A timeout | 504 | ERROR `get joke` with `kind=timeout` |
| Any other `*url.Error` | 502 | ERROR `get joke` with `kind=network` |
| `*dadjoke.StatusError`, or any other error such as invalid JSON | 502 | ERROR `get joke` with `kind=api_error`, plus `status` for a `*dadjoke.StatusError` |

Every ERROR line also has `term` and `error`. `classify` checks for a timeout before it checks for `*url.Error`, because `http.Client` wraps a timeout in a `*url.Error` too. If the visitor's request context is done, the handler logs `request cancelled` at INFO and writes no body.

The client times out after 5 seconds, and HTMX 4 gives up after 60 seconds. A longer client timeout must stay below 60 seconds, or the visitor gets no message at all.

## The search form

During a request, the form shows `#spinner` through `hx-indicator` and disables the Search button through `hx-disable="#search"`. The form uses an id selector because nobody has tested the `find button` form of `hx-disable` in HTMX 4.0.0.

## Static files

`GET /static/` serves the embedded `static/` folder with `http.FileServerFS`. The `noDirListing` wrapper returns 404 for folder paths such as `/static/`, which `http.FileServerFS` would otherwise answer with a file listing.

`TestHTMXScript` compares the SHA-256 of `static/htmx.min.js` with the `htmxSHA256` constant in `server_test.go`. An update of HTMX must change both the file and the constant.
