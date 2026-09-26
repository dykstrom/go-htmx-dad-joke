# HTMX 4 for the search page

> **Working note — not authoritative.** Binding rules live in `architecture/` and `adr/`. Nothing here is a rule until it's promoted — however settled it reads.

Status: Stabilizing

This note collects what the search page needs to know about HTMX 4 before any markup exists. The answers come from the HTMX 4.0.0 docs and from a throwaway test page run in Firefox on 2026-09-26. The test page loaded the pinned `htmx.min.js`, sent four requests, and logged every request header and HTMX event.

## Resolved

### 1. Current release and where to download it

HTMX `4.0.0` is the current HTMX 4 release. npm published it on 2026-08-28, after six alpha and six beta releases.

The npm `latest` tag still points to `2.0.11`. The `next` tag points to `4.0.0`. A URL or install command without an explicit version therefore gets HTMX 2. Always write the full version.

Download URL:

```text
https://cdn.jsdelivr.net/npm/htmx.org@4.0.0/dist/htmx.min.js
```

The file is 36716 bytes, and its SHA-256 is `e484d9171a9db30a39c8f16e3d709d4137f3211c659f8e6125816635033d593f`. The file has no header comment with the version. The version is in the code as `this.version="4.0.0"`, and the test page read `htmx.version` as `4.0.0`.

Sources: the npm registry (`https://registry.npmjs.org/htmx.org`), the jsDelivr response header `x-jsd-version: 4.0.0`, and the test page.

### 2. Changes from HTMX 2 that matter to a search form

HTMX 4 is a rewrite that uses `fetch()` instead of `XMLHttpRequest`. These changes touch a form with one text field, a button, and a result area:

| Change | HTMX 2 | HTMX 4 |
|--------|--------|--------|
| Attribute inheritance | Children inherit `hx-target`, `hx-indicator`, and others from parents | Only with the `:inherited` suffix, for example `hx-target:inherited` |
| Disable elements during a request | `hx-disabled-elt` | `hx-disable` |
| Skip HTMX processing | `hx-disable` | `hx-ignore` |
| 4xx and 5xx responses | Not swapped | Swapped, see question 3 |
| Request timeout | None | 60 seconds (`defaultTimeout` is `60000`) |
| Event names | `htmx:afterSwap`, `htmx:responseError` | `htmx:after:swap`, `htmx:response:error` |
| Request header for the source element | `HX-Trigger` with the element id | `HX-Source` as `tag#id`, for example `button#slowbtn` |
| Request header for the target | `HX-Target` with the id | `HX-Target` as `tag#id`, for example `div#out200` |
| New request header | None | `HX-Request-Type` is `partial` or `full` |
| `Accept` header | Browser default | `text/html` |

`hx-disable` changed meaning between versions. An HTMX 2 example with `hx-disable` does the opposite of what it seems to do in HTMX 4. One context7 search result said `hx-disable` only works as a boolean. That result came from the HTMX 2 source on `master`. The test showed that `hx-disable="this"` works in 4.0.0.

What did not change: `hx-get`, `hx-target`, `hx-swap` with `innerHTML` as the default, `hx-indicator`, and the `HX-Request: true` header. The server can still check `HX-Request` to decide between a fragment and a full page. An `hx-get` on a `<form>` still sends the form's fields as query parameters. The test form sent `GET /case/200?term=cat`.

The HTMX 4 docs warn that an `hx-get` on a button inside a form does not include the form's fields. Put `hx-get` on the `<form>` itself.

Sources: "What's New in htmx 4" (`www/src/content/docs/whats-new-in-htmx-4.md` at tag `v4.0.0`), the upgrade guide (`dist/skills/htmx-upgrade-from-htmx2.md` at tag `v4.0.0`), and the test page's server log.

### 3. 4xx and 5xx responses

HTMX 4 swaps every response into the target except `204` and `304`. The test confirmed it. The `404` and the `500` fragments both replaced the target's content.

For both status codes, the browser fired `htmx:response:error` and then `htmx:before:swap` and `htmx:after:swap`. So the error event does not stop the swap.

This means the server can return a friendly message with a real error status, and HTMX shows it without extra settings. To stop a swap for some codes, HTMX 4 offers `hx-status`, for example `hx-status:5xx="swap:none"`, or the global `htmx.config.noSwap`. The old `htmx.config.responseHandling` is removed.

Sources: "What's New in htmx 4", section "Error responses swap", and the test page with the event log below.

```text
GET /case/404  htmx:response:error status=404, htmx:after:swap status=404
GET /case/500  htmx:response:error status=500, htmx:after:swap status=500
```

### 4. Loading indicator

The indicator works the same way as in HTMX 2. HTMX adds the class `htmx-request` to the element that sends the request, or to the elements that `hx-indicator` selects. An element with the class `htmx-indicator` is hidden until it or a parent has `htmx-request`.

HTMX 4 injects the CSS for this by default, because `includeIndicatorCSS` defaults to `true`. The page needs no CSS of its own for a basic indicator. The built-in CSS uses `opacity` and `visibility`.

The test used `hx-indicator="#spinner"` on a button and `hx-disable="this"` for a 2-second request. The browser logged this state every 250 milliseconds:

```text
During:  spinner opacity=1 visibility=visible classes=[htmx-indicator htmx-request] button.disabled=true
After:   spinner opacity=0 visibility=hidden classes=[htmx-indicator] button.disabled=false
```

Sources: the `hx-indicator` and `includeIndicatorCSS` reference pages at tag `v4.0.0`, and the test page.

## Open questions

- Which status code the app returns for "no joke matched". HTMX swaps any 4xx, so the choice is free. Ticket 005 decides.
- Whether the server timeout for icanhazdadjoke.com stays well under the HTMX default of 60 seconds. Ticket 005 decides.
- Whether `hx-disable="find button"` on the form works. The docs list the `find` form, but the test only tried `hx-disable="this"`.

## For ticket 004

Pin HTMX `4.0.0` from the download URL in question 1, and check the SHA-256 after the download.

The search form needs these attributes:

| Attribute | Where | Purpose |
|-----------|-------|---------|
| `hx-get="/joke"` | `<form>` | Sends the form's fields as `GET /joke?term=...` |
| `action="/joke"` and `method="get"` | `<form>` | Native fallback when JavaScript is off |
| `hx-target="#result"` | `<form>` | Swaps the fragment into the result area |
| `hx-indicator="#spinner"` | `<form>` | Shows the element with class `htmx-indicator` during the request |
| `hx-disable` | `<form>` | Disables the button during the request, see the open question on `find button` |

Write every attribute on the element that sends the request. HTMX 4 does not inherit attributes from parent elements unless they have the `:inherited` suffix.

On the server, check the `HX-Request: true` header to return only the fragment, and return the full page otherwise.
