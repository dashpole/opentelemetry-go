---
name: benchmark-review
description: Compare benchmark performance of a PR branch against its base and produce a formatted benchstat report. Use when a PR claims a performance improvement, touches SDK hot paths, or a maintainer asks for a performance check.
---

# Benchmark Comparison Review

Produce a statistically honest before/after benchmark comparison for a
change. The output is a report the contributor can paste into the PR's
Evidence section, or the reviewer can use directly.

## Non-negotiable method

- **Baseline via a separate worktree — never switch branches in place.**
  `git stash` / `git checkout` dances lose state and invalidate caches:

  ```sh
  git worktree add /tmp/bench-base $(git merge-base HEAD origin/main)
  ```

  (Use the PR's actual base branch if it is not `main`.)

  Build and run baseline benchmarks there; run PR benchmarks in the main
  tree. Remove the worktree (`git worktree remove /tmp/bench-base`) when done.
- **Scope to affected packages.** Identify packages the diff touches (plus
  their direct dependents inside the repo) and benchmark only those.
  Repo-wide `-bench=.` runs are slow and drown the signal.
- **Repetition, then statistics.** Single runs are noise. Use `-count=10`
  and let `benchstat` judge significance:

  ```sh
  go test -run=^$ -bench=. -benchmem -count=10 ./<pkg>/... > /tmp/new.txt
  # same command in the baseline worktree > /tmp/old.txt
  benchstat /tmp/old.txt /tmp/new.txt
  ```

- **Trust benchstat's p-values, not raw deltas.** Report a regression or
  improvement only when benchstat marks the delta significant. A large but
  insignificant delta on a noisy machine is reported as "no reliable
  difference," never as a finding. Note the environment (shared/virtualized
  machines are noisy); if results look unstable, raise `-count` or rerun
  interleaved rather than cherry-picking a clean run.

## What to flag

1. **Allocation deltas on hot paths** (span start/end, metric record,
   context propagation): any significant `allocs/op` increase is a blocking
   finding regardless of ns/op — cite `sdk/AGENTS.md`.
2. **Significant time regressions** in touched packages, with the specific
   benchmark names.
3. **Claims without coverage:** the PR claims a speedup but no benchmark
   exercises the changed code — say which benchmark is missing and sketch it.
4. **Benchmark quality:** new benchmarks that don't use `b.ReportAllocs()`
   or reset timers around setup, benchmarks measuring the wrong unit of work.

## Report format

Start with the verdict in one sentence (improves / regresses / no reliable
change / claims unsubstantiated). Then the raw `benchstat` table in a code
block, environment note (machine, `-count`), and findings. Do not editorialize
beyond what the statistics support.
