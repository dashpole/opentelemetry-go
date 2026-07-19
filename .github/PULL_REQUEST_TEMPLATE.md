<!--
Keep it factual and short. Reviewers check evidence, not prose: a PR that
shows its work gets reviewed faster. PRs whose review cost exceeds the
effort evidently invested in them may be closed — see
docs/ai-contribution-principles.md.
-->

## What & why

<!-- One or two sentences. Link the issue this implements: Fixes #NNN.
If there is no ready-to-implement issue for this change and it touches
public API, open one first — API changes without an approved design are
not reviewed. -->

Fixes #

## Specification references

<!-- Quote the normative spec sentence(s) this change implements, with
pinned links — usually copied from the issue. Write "n/a" for changes with
no spec surface. -->

## Evidence

<!-- Show, don't tell. Paste the commands you ran and their relevant output:
- test run for the new/changed behavior (with -race where applicable)
- for performance claims or hot-path changes: benchstat old vs. new
Do not paste generated summaries of the diff; the diff speaks for itself. -->

```console
$ make precommit
```

## Out of scope

<!-- What you intentionally did NOT do, and where that work is tracked. -->

## Checklist

- [ ] `make precommit` passes locally
- [ ] Tests assert the issue's acceptance criteria (not implementation details)
- [ ] `CHANGELOG.md` updated under `## [Unreleased]` with module reference and
      PR number (or change is not user-visible)
- [ ] No new exported identifiers beyond the issue's approved API design
