# Dad joke API — icanhazdadjoke.com

## Requests

- Every request to icanhazdadjoke.com MUST send a `User-Agent` with the app's name and the repo URL, for example `go-htmx-dad-joke (https://github.com/dykstrom/go-htmx-dad-joke)`.
- Every request MUST send `Accept: application/json`. Without it, the API answers with `text/plain`.
- Source: [the API documentation](https://icanhazdadjoke.com/api), sections "Custom user agent" and "API response format".

## Tests

- Tests that run with `go test ./...` MUST NOT call the real API. They MUST use a fake server from `net/http/httptest`.
- A test that calls the real API MUST skip itself unless `DADJOKE_LIVE=1` is set.
- Source: ticket 003 of this project, decided on 2026-09-26, so that tests and CI need no network access to icanhazdadjoke.com.
