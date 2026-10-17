# Dad joke API client

`internal/dadjoke` calls the icanhazdadjoke.com REST API. Every request sends `Accept: application/json` and a `User-Agent` that names the app. Each request times out after 5 seconds.

`Search` returns a random joke from `/` for an empty or blank term. Otherwise it picks a random joke from all matches with two requests. It fetches page 1 of `/search` with `limit=1` to read `total_jokes`. Then it picks a random index and fetches that page, which holds exactly that joke.

`Search` returns `ErrNoMatch` when `total_jokes` is 0. It also returns `ErrNoMatch` when the picked page comes back empty, which happens if the jokes change between the two requests. Any other error means the API failed: a network error, a timeout, an HTTP status other than 200, or a body that is not valid JSON.

The API reports an unknown joke id as HTTP 200 with `"status":404` in the body. That only happens on `/j/{id}`, which the client does not call, so the client checks only the HTTP status.

The tests in `client_test.go` use a fake server that copies the API's JSON shapes.
