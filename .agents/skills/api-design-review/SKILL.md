---
name: api-design-review
description: Review a PR or proposed API sketch for Go API design quality against this repo's conventions. Use when a change adds or modifies exported identifiers, or when a maintainer asks for design review of signatures proposed on an issue.
---

# Go API Design Review

Judgment-level review of public API surface. Assume the deterministic
checks (linters, API-compatibility tooling) already ran — do not repeat
what they catch (breaking changes, formatting, missing docs). Your scope
is everything a compatibility tool cannot see: whether the API *should*
exist and whether it is shaped right. Output goes to the reviewing
maintainer — do not post PR comments unless explicitly asked.

## Checklist

Work through the diff's exported surface (`gorelease`-style: every added or
changed exported identifier). For each, in order:

1. **Necessity.** Is this identifier required by the linked issue's approved
   API design? Anything exported beyond the approved sketch is a finding —
   the default for new identifiers is unexported or `internal/`. No
   approved sketch on the issue + new public API = blocking finding, full
   stop.
2. **Permanent-commitment test.** Would we be comfortable maintaining this
   exact name and signature forever? Flag: stutter (`trace.TraceOption`),
   abbreviations, `Get*` prefixes, booleans where options belong, concrete
   types where the caller only needs an interface (and vice versa —
   accept interfaces, return concrete types).
3. **Extensibility.** Can this grow without breaking? Configuration must use
   the repo's options pattern (unexported `config`, sealed `Option`
   interface with `apply`, exported `With*`/`Without*` helpers —
   CONTRIBUTING.md "Configuration"); required params precede variadic
   options. New exported interfaces must follow CONTRIBUTING.md "Interface
   Stability": spec-defined interfaces carry the "methods may be added in
   minor releases" warning and use the package's embedding mechanism so
   external implementations don't break; all other stable interfaces are
   frozen, so extension happens via a new interface plus type assertion
   ("How to Change Other Interfaces"). Exported structs should not promise
   comparability or field layout the spec doesn't require.
4. **Consistency.** Does it match the naming and shape of the nearest
   analogous API in this repo (trace vs. metric vs. propagation)? Divergence
   between signal APIs for the same concept is a finding even when both
   designs are individually fine.
5. **Go conventions.** `context.Context` first where the operation can
   block or is request-scoped; errors returned not logged; no exported
   fields that create hidden coupling; interface method parameters named
   (CONTRIBUTING.md "Interface Type").
6. **Cost when disabled.** For API-package additions: does the no-op path
   allocate or force work on callers that have telemetry off?

## Report format

Findings ranked by severity, each with: identifier, the problem in one
sentence, and a concrete alternative signature (not just "reconsider").
Then a short section listing surface that looks right — the maintainer
needs to know what was checked, not only what failed. End with a verdict:
approve surface as-is / approve with renames / needs design round on the
issue.
