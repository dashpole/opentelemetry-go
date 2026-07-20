# Principles for High-Quality Contributions in the Agent Era

Most incoming pull requests are now written wholly or partly by AI coding
agents. That is not a problem to filter out — it is a change in economics to
design for: generating a PR is nearly free, while reviewing one still costs
real maintainer time. Projects that ignore this asymmetry drown their
maintainers; projects that ban AI outright forfeit contributions and can't
enforce the ban anyway. This document takes a third position: **make the
repository itself the expert**, so that an unfamiliar contributor pointing an
agent at one of our issues produces a mergeable PR — and so that maintainer
time concentrates where human judgment is irreplaceable.

These principles are written for opentelemetry-go but are intended to
generalize across OpenTelemetry SIGs. Concrete artifacts implementing them in
this repository: the "Working from issues" section of `AGENTS.md`, the
`spec-implementation` issue template, the PR template, and the skills under
`.agents/skills/` — building on the agent guidance
(`AGENTS.md`, `.github/copilot-instructions.md`) and conventions
(`CONTRIBUTING.md`, `VERSIONING.md`) the repository already has.

The skills use the [Agent Skills](https://github.com/agentskills/agentskills)
open format (a directory containing a `SKILL.md` with YAML front matter),
which is read by multiple coding agents, not just one vendor's. Canonical
copies live in the vendor-neutral `.agents/skills/` directory (the
project-level location the format's ecosystem converged on, alongside
`AGENTS.md`); `.claude/skills/` holds relative symlinks for tools that only
discover their vendor-specific path.

## The principles

### 1. The issue is the prompt

Assume every issue will be pasted verbatim into a coding agent by someone
with zero project context. Anything not stated in the issue — or linked with
a specific section anchor — effectively does not exist. A well-specified
issue contains: the requirements (for spec work, the quoted normative
sentences with pinned links), the affected packages and types, the non-goals,
an API sketch or an explicit "design open" marker, and acceptance criteria.
Under-specified issues are the root cause of low-quality agent PRs: the agent
fills every gap with a plausible-looking guess.

### 2. Separate design from implementation — and only advertise the latter

Spec interpretation and API design are the two most expensive things to
review, and the worst place to review them is a finished PR. Issues are born
`needs-design`; a maintainer settles the API sketch and spec reading *on the
issue*, then relabels it `ready-to-implement` (optionally `help wanted`).
Reviewing a sketch costs minutes; reviewing a PR that embodies a wrong design
costs hours and usually ends in abandonment. An issue with settled signatures
is also precisely the kind of task agents execute well.

### 3. Keep tasks small

The strongest empirical predictor of agent task failure is the size and
spread of the required change — completion rates collapse as edits span more
files and more disjoint hunks. Decomposition is part of issue authoring: a
`ready-to-implement` issue should be confined to one module and ideally a
handful of files. A perfectly specified 500-line issue is still a bad issue;
split it.

### 4. The definition of done must be executable

Phrase acceptance criteria as things a machine or a reviewing agent can
check: "default is 512 per spec §X (quoted)", "no significant `allocs/op`
delta in `benchstat`", "test T fails before, passes after". The strongest
form is a failing acceptance test included in the issue itself. Vague
criteria produce vague PRs.

### 5. Evidence, not assertions

Agents are excellent at producing evidence when the template demands it and
excellent at producing confident prose when it doesn't. PRs must show their
work: spec sections quoted, test output pasted, `benchstat` for any
performance claim, and an explicit "out of scope" statement. This converts
review from "re-derive whether this is correct" into "check the evidence."
The enforcement rule, borrowed from FastAPI's contributor policy: **if the
effort invested in a PR is evidently less than the effort required to review
it, the PR may be closed on that basis alone.** This rule needs no AI
detection and no debate about tooling — it prices the externality directly.

### 6. Push every rule down the enforcement ladder

Anything a maintainer says twice in review should move as far down this
ladder as it can:

1. **Deterministic CI check** — linters, API-compatibility diffing,
   changelog checks, spec-default assertions. Cheapest, zero noise budget.
2. **Written rule an agent can cite** — CONTRIBUTING.md, AGENTS.md. Free to
   apply, requires the reader to comply.
3. **Review skill** — packaged judgment for recurring review types (spec
   compliance, API design, benchmarks). For what tools can't check.
4. **Human attention** — reserved for what's left: taste, tradeoffs, spec
   interpretation calls.

A good maintenance habit: periodically skim your own review comments and ask
which rung each should have been caught on.

### 7. Gate by blast radius, not contributor identity

Anyone with an agent is now a contributor; vetting people doesn't scale, but
gating change types does. Additions to public API of a released module
require a design-approved issue — no exceptions. Internal changes, tests,
and docs flow freely. Maintainer attention concentrates where mistakes are
permanent.

### 8. Agent guidance files are written by hand and kept minimal

`AGENTS.md` earns its place line by line: every line must state something an
agent cannot infer from the codebase (commands, hard rules, workflow gates).
Auto-generated context files are actively harmful — evaluations have found
they increase inference cost without improving success rates. This
repository's existing `AGENTS.md` and `.github/copilot-instructions.md`
already follow this principle: hand-written and convention-dense; keep them
that way. Corollaries:
state rules affirmatively ("use the options pattern") rather than as
negations, which keep the forbidden pattern salient; and if a linter can
enforce a rule, the guidance file says only "run the linter."

### 9. Review automation serves the reviewer, not the thread

Automated review comments are a known failure mode — projects that let AI
reviewers post directly have found the noise costs maintainers more than the
bad PRs did. Review skills here produce reports *to the reviewing
maintainer*, who decides what reaches the PR. Pilot any review automation on
a couple dozen PRs and tune before it is allowed to speak publicly, and
never run it with write access to secrets on fork PRs.

## The deterministic floor (roadmap)

The ladder's first rung, in adoption order. Some of it already exists here —
a per-module `make gorelease` target, golangci-lint v2, and continuous
benchmarking of `main` — so each item below closes a specific remaining gap,
and each removes a class of review comment permanently:

1. **API-compatibility gating in PR CI**: wire the existing per-module
   `make gorelease` (or the `apidiff` GitHub Action) into pull request
   checks — report-only first, then hard-fail for released modules. This is
   a multi-module repo: the check must run per module against each module's
   own baseline.
2. **Differential linting**: enable golangci-lint's
   `issues.new-from-merge-base` so strict new rules apply to new code
   without a legacy cleanup.
3. **Project-idiom rules**: a small, hand-written ruleguard bundle encoding
   CONTRIBUTING.md's patterns (options pattern shape, no anonymously
   embedded locks in exported types), distributed as a Go module other
   OTel Go repositories can import.
4. **Spec-default assertions**: the specification's environment-variable and
   default-value tables are semi-structured; a small check asserting our
   defaults, names, and units against them catches the silent-noncompliance
   class (wrong unit, wrong default) that has produced multi-year bugs in
   other SIGs.

## Adopting this beyond opentelemetry-go

The language-agnostic pieces — this document, the issue lifecycle and
template, the PR evidence template, the spec-compliance review skill, and
the spec-default checker — could live in a community repository and be
adopted per-SIG. The API-design skill and idiom rules are inherently
per-language and need each SIG's maintainers to encode their own
conventions. The spec-compliance surface is the best build-once candidate:
the specification and its compliance matrix are shared by every SIG, and
today every SIG verifies compliance by hand.

## References

All sources below were verified against primary material; claims from
earlier research drafts that could not be verified were excluded from this
document.

**Policies referenced by principle 5 (evidence, not assertions):**

- FastAPI contributing guidelines, "Automated Code and AI":
  <https://tiangolo.com/open-source/contributing/#automated-code-and-ai> —
  "If the human effort put in a PR, e.g. writing LLM prompts, is less than
  the effort we would need to put to review it, please don't submit the PR."
- curl's experience with unstructured AI submissions — bug bounty ended
  January 2026 after a flood of fabricated AI reports:
  <https://daniel.haxx.se/blog/2026/01/26/the-end-of-the-curl-bug-bounty/>

**Policy referenced by principle 9 (reviewer-facing automation):**

- Django, "Submitting contributions":
  <https://docs.djangoproject.com/en/dev/internals/contributing/writing-code/submitting-patches/>
  — requires disclosure of AI tool use and prohibits requesting automated
  AI reviews on Django PRs because they "do not replace human review and
  often generate noise."

**Evidence for principle 3 (keep tasks small):**

- OpenAI, "Introducing SWE-bench Verified":
  <https://openai.com/index/introducing-swe-bench-verified/> — difficulty
  annotations: 38.8% of tasks ≤15 min, 52.2% 15 min–1 hr, 9% ≥1 hr.
- Analyses of success by difficulty and file spread (J. Ganhotra):
  <https://jatinganhotra.dev/blog/swe-agents/2025/06/05/swe-bench-verified-discriminative-subsets.html>
  — top agents resolve 84–86% of easy tasks but ~42% of hard tasks and
  ~10% of multi-file problems.

**Evidence for principle 1's localization guidance:**

- Liang, Garg, Zilouchian Moghaddam, "The SWE-Bench Illusion: When
  State-of-the-Art LLMs Remember Instead of Reason":
  <https://arxiv.org/abs/2506.12286> — models identify buggy file paths
  from issue text alone at up to 76% on SWE-bench repositories vs. ~53% on
  repositories outside the benchmark, i.e. agents cannot be assumed to
  "know" a codebase's layout; issues must localize.

**Evidence for principle 8 (hand-written, minimal guidance files):**

- Gloaguen, Mündler, Müller, Raychev, Vechev (ETH Zurich), "Evaluating
  AGENTS.md: Are Repository-Level Context Files Helpful for Coding
  Agents?": <https://arxiv.org/abs/2602.11988> — context files do not
  generally improve task success while increasing inference cost by over
  20%; LLM-generated files slightly *reduce* resolution rates, while
  minimal developer-written files give a marginal gain.

**The AGENTS.md convention:**

- <https://agents.md/> — donated to the Linux Foundation's Agentic AI
  Foundation (Dec 2025), adopted by 60,000+ projects; read by Copilot,
  Cursor, Codex, Gemini CLI, and others:
  <https://www.linuxfoundation.org/press/linux-foundation-announces-the-formation-of-the-agentic-ai-foundation>

**Tooling for the deterministic floor:**

- `apidiff` as a GitHub Action: <https://github.com/joelanford/go-apidiff>
  (used in CI by kubebuilder and operator-framework).
- golangci-lint `issues.new-from-merge-base` (report only issues new
  relative to the merge base):
  <https://golangci-lint.run/docs/configuration/file/>
- `gorelease`: <https://pkg.go.dev/golang.org/x/exp/cmd/gorelease>
- OpenTelemetry spec compliance matrix:
  <https://github.com/open-telemetry/opentelemetry-specification/blob/main/spec-compliance-matrix.md>
