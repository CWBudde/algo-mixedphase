# AGENTS.md

Guidance for Claude Code and other agents working in this repository.

## Project Overview

`algo-mixedphase` designs mixed-phase FIR filters and compares the methods that produce
them. It is the software companion to Christian-W. Budde's DAGA 2012 paper on fixed-support
mixed-phase filter design.

**Module**: `github.com/cwbudde/algo-mixedphase`

Two packages, both at the repository root:

- `mixedphase` — alternating minimum/linear-phase factorisation (DAGA 2012), phase
  interpolation, weighted complex least squares with Lawson minimax refinement, direct
  low-group-delay optimisation (Wu–Gao–Teo), two minimum-phase reconstructions, and
  comparison metrics.
- `graphiceq` — hybrid IIR/FIR low-latency octave graphic EQ (DAFx 2022). A
  structure-specific comparison, deliberately _not_ part of the mixed-phase API.

**Upstream**: [`algo-dsp`](https://github.com/cwbudde/algo-dsp) supplies windows,
convolution, spectrum utilities, the biquad runtime and the biquad designers;
[`algo-fft`](https://github.com/cwbudde/algo-fft) supplies transforms. Depend on their
**public** API only — nothing from either library is duplicated here. If something is
missing upstream, add it upstream rather than reimplementing it here.

## Development Commands

```bash
just test            # go test ./...
just test-race       # go test -race ./...
just lint            # golangci-lint (v2 config, incl. wsl_v5)
just fmt             # treefmt
just bench           # benchmarks
just compare         # regenerate the comparison CSVs quoted in the docs
just compare-check   # prove those CSVs are byte-reproducible
just compare-timings # refresh the machine-local timings (not reproducible)
just check-coverage  # enforce the 90% floor on the two public packages
just ci              # everything CI runs
just web-demo        # build and serve the Mixed Phase Lab on :8787
```

## What makes a change acceptable here

This is a comparison repository, so the standard is reproducibility rather than API reach:

1. **Every number in a doc comment or in `docs/` must be reproducible** by a test or by
   `just compare`. State the budget that produced it — iteration counts and tolerances are
   dials, not converged values. Nothing machine-dependent may enter a committed
   artifact that `just compare` regenerates: timings belong in
   `docs/reference-timings.csv`, which is written only by `just compare-timings`
   and is never diff-gated.
2. **Measure against the structure that actually runs.** Metrics computed on the design
   grid are not proof; verify against the realised impulse response (see
   `TestImpulseResponseMatchesMetrics` in `graphiceq`).
3. **Document failure modes as prominently as successes.** Each method here has an input
   class it handles badly — the zigzag target for `graphiceq`, the linear-phase start for
   `DesignLowGroupDelay`, zero-weight bins for the least-squares path. Those belong in the
   package docs.
4. **New methods are additive.** Adding a design must not change the output of an existing
   one; the example CSVs are the regression test.
5. **Deterministic behaviour**: same input + config = same output.
6. Errors are sentinel values wrapped with `fmt.Errorf("%w: …")`; zero-valued config fields
   mean "default", negative ones are rejected.
7. Public types and functions need doc comments; design entry points need runnable examples.

## Testing

- Table-driven tests for validation and edge cases.
- Property/invariant tests for algorithmic claims (a halved latency, a monotone taper).
- Golden CSVs via the examples for cross-method comparison.
- Coverage target: ≥ 90% for both packages.

## Conventions

- Conventional commits.
- Go: the version in `go.mod` (1.25) only. `algo-dsp` and `algo-fft` both
  declare `go 1.25.0`, so a "previous stable" job cannot build this module —
  it would just download 1.25 through `GOTOOLCHAIN` and test the same thing
  twice. Widen the matrix again once upstream lowers its floor.
- Semantic versioning; `v0.x` until the API settles.
