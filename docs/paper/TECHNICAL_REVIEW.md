# Paper technical review

Review completed 2026-07-29 for the English Typst manuscript and the current
Phase 3 reference suite. This record separates evidence that can be checked in
the repository from the publication check that can only happen after a version
tag exists.

## Historical attribution

The manuscript was checked against both pages of Christian-W. Budde's 2012 DAGA
paper, [“Gemischtphasige Filter”][original].

| Manuscript statement                                                                                                                            | Original evidence                                               | Review outcome                                                    |
| ----------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- | ----------------------------------------------------------------- |
| A delay or pre-ringing budget motivates an intermediate phase response.                                                                         | “Motivation”, page 103                                          | Retained as a 2012 contribution.                                  |
| The developed structure cascades minimum- and linear-phase FIR parts.                                                                           | Figure 3 and “Zerlegung eines Filters”, page 104                | Retained as the developed construction.                           |
| The residual is obtained through the spectral difference, equivalently complex division, and is forced to the selected phase before truncation. | Steps 1–8, page 104                                             | Restated in English with present notation.                        |
| Alternating residual updates compensate the two windowed factors.                                                                               | “Probleme bei der Zerlegung”, page 104                          | Retained as the distinguishing iterative step.                    |
| Response difference and changing window parameters are possible stopping or update policies.                                                    | “Probleme bei der Zerlegung”, page 104                          | Reported as suggestions, not as a fully specified 2012 algorithm. |
| Minimum/maximum-phase, frequency-dependent weighting, and a three-factor split are extensions.                                                  | “Zerlegung eines Filters” and “Weitere Möglichkeiten”, page 104 | Kept outside the implemented 2012 result.                         |

Two historical statements deliberately needed qualification. The original
describes the combined length as the sum of both factor lengths; the revision
uses the exact finite-convolution relation `NA + NB - 1 = N` and labels that
choice as later repository work. The original also describes the resulting
support as nearly or optimally short without a proof or controlled comparison;
the revision treats this as motivation rather than a demonstrated optimum.

## Implementation and API audit

The algorithm steps, notation table, and reproducibility appendix were checked
against these executable paths:

- alternating factorisation: `mixedphase/iterative.go` and
  `IterativeConfig`;
- minimum-phase reconstruction: `mixedphase/minimum.go` and
  `MinimumPhaseConfig`;
- phase interpolation and weighted complex fitting:
  `mixedphase/interpolate.go` and `mixedphase/complexls.go`;
- direct low-group-delay optimisation: `mixedphase/lowdelay.go`,
  `mixedphase/lbfgs.go`, and `LowGroupDelayConfig`;
- hybrid graphic equalisation: `graphiceq/design.go` and `graphiceq.Config`;
  and
- common realised-response analysis and budgets: `internal/reference/`.

The review confirmed that configuration limits are described as budgets rather
than convergence claims, the final support is measured from realised taps, and
the six documented failure modes have named regression tests. Public upstream
dependencies are used through their exported APIs.

## Quantitative evidence audit

The paper's result table, five cross-target summary counts, and all six
quantitative charts read the committed CSVs directly. The additional
signal-flow figure is a qualitative redrawing of Figures 2 and 3 in the
original paper; it adds the revision's exact convolution-support notation but
no measured value. The manuscript stores configuration budgets and
mathematical constants, but no measured result value. The evidence paths are:

- `docs/reference-results.csv` for cross-target scalar metrics;
- `docs/reference-response.csv` for the representative parametric-EQ frequency
  responses;
- `docs/reference-impulse.csv` for the fourth-order Linkwitz–Riley 2 kHz
  low-pass crossover impulse responses, including regression checks that the
  alternating result uses meaningful, distributed pre-peak energy; and
- `docs/graphiceq-results.csv` for the structure-specific latency comparison.

`just compare-check` regenerates these files and requires a byte-identical
working tree. `internal/reference/reference_test.go` checks their schemas,
common budgets, realised-response coverage, and committed contents. Runtime
measurements are excluded from the paper's deterministic claims.

The cross-target conclusion was checked against every row rather than inferred
from one representative plot. It reports the objectives conditionally:
alternating correction leads the fixed-support relative-magnitude comparison,
while direct low-delay optimisation leads the mean-delay comparison under its
separate magnitude constraint. The manuscript does not turn those observations
into a universal ranking.

## Build and publication audit

The paper workflow starts from an Actions checkout, installs the pinned Typst
compiler and formatter, checks formatting, runs `just paper`, and uploads the
result as a CI artifact. On a `v*` tag, a dependent release job downloads that
same artifact and attaches `mixed-phase-filter-design-en.pdf` to the GitHub
release. Repository, implementation-note, and Mixed Phase Lab links use the
stable `releases/latest/download/` asset URL, so they do not depend on a
`main`-branch source path when viewed from a tag.

There is no version tag yet. The workflow contract is reviewed, but the final
HTTP download check remains a Phase 6 release-time gate and must not be marked
complete before the first tagged release exists.

[original]: https://pub.dega-akustik.de/DAGA_2012/data/articles/000281.pdf
