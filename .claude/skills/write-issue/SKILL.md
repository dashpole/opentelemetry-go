---
name: write-issue
description: Expand a maintainer's rough task idea into a complete spec-implementation issue draft using this repo's issue template. Use when a maintainer describes work to be filed as an issue, or asks to "write this up" / "file an issue for X".
---

# Issue Writer (maintainer-side)

Turn a one-or-two-sentence task idea into a draft issue that an unfamiliar
contributor's coding agent could complete. The template being filled is
`.github/ISSUE_TEMPLATE/spec-implementation.md` — produce every section.
**Always output a draft for the maintainer to review; never file the issue
or post it anywhere yourself.**

## Procedure

1. **Find the spec surface.** Search the OpenTelemetry specification
   (https://github.com/open-telemetry/opentelemetry-specification,
   `specification/` tree, latest release tag unless told otherwise) for the
   sections governing this task. Quote every normative sentence
   (MUST/SHOULD/MAY) that applies, with pinned links. If the task has no
   spec surface, state that explicitly in the draft.
2. **Localize.** Search this repository for the module, packages, files,
   types, and functions the change will touch. List expected touch points
   and, just as important, neighboring code that should NOT change.
3. **Size check — before drafting further.** If the work spans more than
   one module, or several packages with disjoint edits, stop and propose a
   split into multiple issues (with a suggested ordering and which one
   unblocks the others). Task completion rates collapse as changes spread
   across files; small issues are a feature, not an inconvenience.
4. **Derive acceptance criteria.** One checklist item per quoted normative
   sentence, plus items for error paths and concurrent use where relevant.
   Each must be objectively checkable.
5. **Draft acceptance tests.** Write the table of test cases (name, setup,
   input, expected outcome) covering the criteria. Where cheap, write the
   actual failing table-driven test.
6. **Draft the API sketch — clearly marked unapproved.** Propose signatures
   following CONTRIBUTING.md's configuration and interface patterns, under a
   header: `PROPOSED — awaiting maintainer sign-off`. If the design is
   genuinely open, write "DESIGN OPEN — do not start implementation" and
   list the open questions instead.
7. **Write non-goals.** Ask yourself what a capable agent would plausibly
   also do (refactor neighbors, add extra options, handle adjacent spec
   sections) and explicitly exclude what isn't wanted.

## Output

The complete issue body in a single markdown block, ready to paste, with
label `needs-design`, followed by a short note to the maintainer listing:
what needs their judgment (the API sketch, any spec-interpretation calls
made), anything that looked ambiguous in the spec, and the proposed split
if step 3 triggered.
