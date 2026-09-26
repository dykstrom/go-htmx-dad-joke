# Dad joke API client

`internal/dadjoke` calls the icanhazdadjoke.com REST API. Every request sends `Accept: application/json` and a `User-Agent` that names the app. Each request times out after 5 seconds.

The API can report an error as HTTP 200 with a body such as `{"message":"...","status":404}`. The client decodes every body and checks its `status` field as well as the HTTP status. Either one other than 200 gives a `*StatusError`.

`Search` returns `Random` for an empty or blank term. Otherwise it picks a random joke from all matches. It first fetches page 1 with `limit=30`, the largest limit the API accepts. If `total_jokes` is larger than page 1, it picks an index over all matches and fetches the page that holds it.

Callers tell the results apart like this:

| Result | Meaning |
|--------|---------|
| `ErrNoMatch` | The search matched nothing. The API answers this with HTTP 200 and an empty list |
| `*StatusError` | The API answered with a status other than 200 |
| An error where `net.Error` reports `Timeout()` | The API did not answer within 5 seconds |
| An error that matches `context.Canceled` | The caller's request was cancelled |

The tests in `client_test.go` use a fake server that copies the API's JSON shapes. They replace the client's unexported `randIntN` field to get a fixed pick.
