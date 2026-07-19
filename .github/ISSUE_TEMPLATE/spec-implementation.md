---
name: Specification implementation task
about: A fully specified implementation task, ready for a contributor (human or AI-assisted) to pick up. Authored by maintainers.
labels: needs-design
---

<!-- Template bodies use section headings; an h1 would render as a title
inside every filed issue. -->
<!-- markdownlint-disable MD041 -->

<!--
MAINTAINER GUIDE (delete this comment before submitting)

This template produces an issue that an unfamiliar contributor — very likely
driving an AI coding agent — can complete without extra context. Assume the
issue body will be pasted verbatim into an agent as its prompt: anything not
written or linked here effectively does not exist.

Lifecycle: the issue is born `needs-design`. Only after a maintainer fills in
and approves the "API design" section, replace the label with
`ready-to-implement` (and optionally `help wanted`). Never advertise a
needs-design issue to contributors.

Sizing: tasks confined to a single package succeed; sprawling ones fail.
Evidence from agent coding benchmarks shows completion rates collapse as
changes span more files and disjoint edits. If you cannot fill in "Where"
with one module and a short list of files, split this into multiple issues.
-->

## Summary

<!-- Two or three sentences: what capability is added or fixed, and why. -->

## Specification references

<!--
Link the EXACT sections, pinned to a spec version tag (not main), and quote
the normative sentences (MUST/SHOULD/MAY) this task implements. The quoted
lines are the requirements; the acceptance criteria below must cover each
one. Example:

- https://github.com/open-telemetry/opentelemetry-specification/blob/v1.x.0/specification/trace/sdk.md#batching-processor
  > "batchSize - the maximum batch size of every export. It must be smaller or equal to maxQueueSize. The default value is 512."

Reminder for implementers: this project follows "capabilities, not structure
compliance" (see CONTRIBUTING.md) — match the spec's behavior, not its
object hierarchy.
-->

## Required behavior

<!--
Acceptance criteria as a checklist. Each item should be objectively
checkable by a test or a reviewer, and each quoted normative spec line above
should map to at least one item.
-->

- [ ]
- [ ]

## Non-goals

<!--
What this task deliberately does NOT include. This is the highest-leverage
section for agent-driven contributions — it prevents plausible-looking scope
creep. Include known adjacent work and any spec requirements intentionally
deferred or deviated from (with a link to where that decision was made).
-->

## Where

<!--
Localization anchors. Name the module, packages, and (when known) the files,
types, and functions expected to change. Agents' ability to find the right
code collapses without this. Also state what must NOT be modified.

- Module/package:
- Expected touch points:
- Do not modify:
-->

## API design

<!--
One of the following two states. The issue stays `needs-design` until a
maintainer approves a sketch.

STATE A — settled (required before `ready-to-implement`):
Paste the approved signatures — new types, functions, options — following the
configuration and interface patterns in CONTRIBUTING.md.

STATE B — open:
Write "DESIGN OPEN — do not start implementation." List the open questions.
Contributors are welcome to propose signatures in comments.
-->

## Acceptance tests

<!--
The executable definition of done. Best: paste a failing test (table-driven)
that must pass when the task is complete. Minimum: a table of test cases —
name, setup/input, expected outcome — covering each Required-behavior item,
including error and concurrent paths where relevant.
-->

## Verification

<!-- Adjust as needed. These commands must pass before a PR is opened. -->

- [ ] `make precommit`
- [ ] New/changed tests pass with `-race`
- [ ] `CHANGELOG.md` entry under `## [Unreleased]` (module reference and PR
      number, per the changelog conventions in `AGENTS.md`)
<!-- For performance-sensitive paths, also require:
- [ ] `benchstat` comparison against the base branch included in the PR
-->
