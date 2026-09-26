# 0002. Use Pico CSS for styling

*2026-09-26*

## Context

The README asks for a modern look and feel, and the whole UI is a search field, a button, and a result area. One alternative was a small hand-written stylesheet with CSS custom properties and no dependency. Another was Tailwind CSS, which needs the Tailwind CLI as a build step before `go run` and adds Node tooling to a Go repo. Pico CSS styles plain HTML elements, so the page looks finished with little custom CSS. Pico ships a classless build and a standard build that adds a few layout classes such as `.container` and `.grid`.

## Decision

We will use Pico CSS 2.1.1, standard build `pico.min.css`, vendored as ADR 0001 describes. Custom styles go on top of Pico, not in place of it.

## Consequences

Plain semantic HTML gets a clean look with no classes, and the layout classes are available for the page design. Pico's defaults set the look, so a different design means overriding them instead of writing styles from nothing. The page loads an 83 KB stylesheet for three elements, which does not matter for an app that runs locally. An update of Pico is a new vendored file, as for HTMX.
