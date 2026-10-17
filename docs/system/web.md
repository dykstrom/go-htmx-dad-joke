# Web package

`web.New` in `internal/web/server.go` builds the app's `http.Handler`. It takes a `JokeSource`, an interface with the one method `Search` that `*dadjoke.Client` implements, and a `*slog.Logger`. The handlers log through that logger, not through the global `slog` functions. Handler tests pass `fakeJokes` from `server_test.go`, so they need no network. `newTestServer` also returns a `logBuffer`, so tests can check log lines. `web.New` parses every template when the app starts, so a template error stops startup instead of failing on the first request.

All templates are parsed into one template set. `layout.html` is the page skeleton and calls the `main` template, which `index.html` defines. `result.html` defines `result`. Every template gets the same flat `page` struct, with `Term`, `Joke` and `Message`. One set works because the app has one page. A second page would need its own set, because both pages would define `main`.

## The joke endpoint

`GET /joke?term=...` passes the term to `Search`, which handles a blank term. A request with the header `HX-Request: true` gets only the `result` template. Any other request gets the full index page, with the term in the search field and the result in `#result`. Every answer sets `Vary: HX-Request`.

When `Search` returns an error, the `result` template renders a message in place of a joke. The visitor never sees the error text.

| Error from `Search` | Status | Log |
|---------------------|--------|-----|
| `dadjoke.ErrNoMatch` | 404 | None |
| Any other error | 502 | ERROR `get joke` with `term` and `error` |

The client times out after 5 seconds, and HTMX 4 gives up after 60 seconds. A longer client timeout must stay below 60 seconds, or the visitor gets no message at all.

## The search form

During a request, the form shows `#spinner` through `hx-indicator` and disables the Search button through `hx-disable="#search"`. The form uses an id selector because nobody has tested the `find button` form of `hx-disable` in HTMX 4.0.0.

When a visitor clicks Search, the button is disabled, and the browser drops focus to the page body. The form's `hx-on:htmx:finally:request="this.elements.term.focus()"` puts focus back in the search field. HTMX 4.0.0 fires `htmx:finally:request` after every request, also when `fetch()` fails. `htmx:after:request` fires only when a response arrives. The field also has `autofocus`, so a visitor can type as soon as the page loads.

`#result` has `aria-live="polite"`, so screen readers announce each new joke or message. Nobody has tested this with a screen reader yet.

## Styling

`static/app.css` holds the app's own styles. The layout loads it after `pico.min.css`, so its rules win. It sets no colours of its own and uses Pico variables such as `--pico-muted-color` instead. Pico switches those variables with the system light or dark setting, so the page follows that setting with no theme switch.

`result.html` puts a joke in `<article class="joke">`, which Pico draws as a card, and a message in `<p class="message">`. `app.css` styles both classes. New content in `#result` fades in, except when the visitor's system asks for reduced motion.

## Static files

`GET /static/` serves the embedded `static/` folder with `http.FileServerFS`. `/static/` itself answers with a listing of the files, which are all public anyway.

`TestHTMXScript` compares the SHA-256 of `static/htmx.min.js` with the `htmxSHA256` constant in `server_test.go`. An update of HTMX must change both the file and the constant.
