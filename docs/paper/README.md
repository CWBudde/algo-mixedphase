# Paper

This directory contains the source of the revised English paper accompanying the
repository. The Typst sources are canonical; the PDF is a generated build artifact.

## Build

Install Typst 0.15.0 and run:

```bash
just paper
```

The result is `docs/paper/mixed-phase-filter-design-en.pdf`. It is ignored by Git because
CI builds and uploads the same PDF as an artifact. During editing, use:

```bash
just paper-watch
```

`just fmt` formats `.typ` sources with `typstyle`; CI uses the same formatter.

## Layout

- `paper.typ` — the single paper entry point and current English draft.
- `style.typ` — page, typography, and reusable callout definitions.
- `references.bib` — bibliography metadata.
- `figures/` — generated figures derived from committed Phase 3 benchmark data.

The original German contribution, [“Gemischtphasige Filter”][original], remains the
historical source. It is linked rather than copied here until its redistribution terms have
been confirmed.

## Keeping paper and code in step

The point of this repository is that the paper's claims are re-runnable. When a figure or
table corresponds to a measurement here, the reproducibility map in `paper.typ` records the
implementation, evidence, configuration budget, and command that reproduces it.
`just compare` regenerates the comparison CSVs, and the package tests carry the assertions
behind quoted numbers.

Normal paper builds consume committed data and never rerun timing benchmarks. Phase 3 will
add the CSV-to-figure refresh command once the common benchmark schema is stable.

[original]: https://pub.dega-akustik.de/DAGA_2012/data/articles/000281.pdf
