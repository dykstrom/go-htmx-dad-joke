# 0000. Record architecture decisions

*2026-09-26*

## Context

This project is built with AI coding agents, and an agent works from what it can read in the repo. The reason behind a structural choice is rarely visible in the code, so a later contributor or agent can undo the choice without knowing why it was made. Commit messages hold some reasons, but they are hard to find later. A single design document is easier to find, but edits over time replace the reasoning as it stood when the decision was made.

## Decision

We will record each architecturally significant decision as a short ADR in `docs/adr/`, with the sections Context, Decision, and Consequences. We will write ADRs with the TRACE skill `/trace:adr` and follow the rules in `docs/adr/README.md`.

## Consequences

The reason behind each structural choice has one place to look, and the files are numbered in the order the decisions were made. An ADR does not change after it ships. A change of course is a new ADR that supersedes the old one. Decisions below the ADR bar go to `docs/system/` or `docs/architecture/` through `/trace:distil` instead.
