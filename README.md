# go-htmx-dad-joke

[![CI](https://github.com/dykstrom/go-htmx-dad-joke/actions/workflows/ci.yml/badge.svg)](https://github.com/dykstrom/go-htmx-dad-joke/actions/workflows/ci.yml)

A Go/HTMX REST client for icanhazdadjoke.com

## Overview

This section describes the application in present tense, even though it
has not yet been built. What it really describes is how it will work and
look when it is finished.

go-htmx-dad-joke is an application that lets a user search for dad jokes
on icanhazdadjoke.com. The enters a search word and clicks a search button.
The application retrieves a joke using the icanhazdadjoke.com REST API,
and displays it to the user. The user can also search for a random joke
by leaving the search field empty. If no joke can be found or if an error
occurs, a suitable message is displayed to the user instead.

The entire UI consists of a search field, a button, and an output text area.

The backend is built in Go, and the frontend in HTMX 4, using a template
language that works well with Go.

The application is built using best practices in Go and HTMX, and uses a
modern look and feel in the UI.
