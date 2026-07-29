# Paper

This directory contains the source of the revised English paper accompanying the
repository. The Typst sources are canonical; the PDF is a generated build artifact.

## Build

Install Typst 0.15.0 and run:

```bash
just paper
```

The result is `docs/paper/mixed-phase-filter-design-en.pdf`. It is ignored by Git because
CI builds and uploads the same PDF as an artifact. A `v*` tag also creates a GitHub release
and attaches that exact CI artifact as
[`mixed-phase-filter-design-en.pdf`][paper-pdf]; the binary is never maintained by hand.
During editing, use:

```bash
just paper-watch
```

To regenerate the committed benchmark CSVs and then rebuild every data-backed chart:

```bash
just paper-refresh
```

`just fmt` formats `.typ` sources with `typstyle`; CI uses the same formatter.

## Layout

- `paper.typ` — the single paper entry point and current English draft.
- `style.typ` — page, typography, and reusable callout definitions.
- `charts.typ` — dependency-free SVG charts generated from the committed CSVs, plus the
  native signal-flow diagram redrawn from the original paper.
- `references.bib` — bibliography metadata.
- `figures/` — reserved for externally rendered figures if a future plot cannot be
  expressed by the native chart helpers.

The original German contribution, [“Gemischtphasige Filter”][original], remains the
historical source. It is linked rather than copied here until its redistribution terms have
been confirmed.

## Editorial provenance

The English manuscript is a revision, not a translation. It attributes the latency-budget
motivation, two-factor minimum/linear-phase cascade, truncation compensation, and
alternating correction to the 2012 paper. Exact convolution-length accounting, regularised
division, concrete reconstruction and stopping policies, comparison methods, failure
analysis, and benchmark results are explicitly identified as later repository work.

The minimum/maximum-phase split, frequency-dependent weighting, and three-factor
decomposition mentioned in the original are reported as proposed extensions, not as
implemented or evaluated 2012 results.

## Keeping paper and code in step

The point of this repository is that the paper's claims are re-runnable. When a figure or
table corresponds to a measurement here, the reproducibility appendix in `paper.typ`
records the implementation, evidence, configuration budget, and command that reproduces it.
`just compare` regenerates the comparison CSVs, and the package tests carry the assertions
behind quoted numbers.

Normal paper builds consume committed data and never rerun timing benchmarks. The charts
read `docs/reference-results.csv`, `docs/reference-response.csv`,
`docs/reference-impulse.csv`, and `docs/graphiceq-results.csv` directly. The response
artifact contains the realised parametric-EQ designs used for the magnitude and weighted
group-delay figures. The impulse artifact uses the first-order 1 kHz low-pass target so the
peak-aligned plot visibly exercises the alternating design's pre-peak support. Only the
explicit `just paper-refresh` command reruns the benchmark generators.

The reproducibility appendix maps every public design method, numbered equation, figure,
and table to its implementation, named tests, exact reference budget, artifact, and
regeneration command. Its notation table records the correspondence between paper symbols
and public Go configuration/result fields. The representative response CSVs and summary
CSVs are byte-compared against fresh generator output by the reference tests. The committed
quality data deliberately contain no wall-clock values; those live separately in
`docs/reference-timings.csv`.

The paper gives failure cases their own numbered table rather than leaving them as scattered
caveats. Correction-loop instability, optimiser initialisation sensitivity, zero-weight
bins, stopband group-delay masking, and the hybrid graphic-EQ target-class limit each map to
a named regression test in the reproducibility appendix.

The completed editorial audit is recorded in
[`TECHNICAL_REVIEW.md`](TECHNICAL_REVIEW.md). It checks historical attribution against the
German original, API and algorithm statements against the Go implementation, and every
new result against the committed comparison artifacts.

## Layout reference

The document uses an AES-informed author-manuscript layout: a full-width title and abstract,
a two-column body, numbered sections and figures, bracketed numbered references, and chart
symbols or hatch patterns that remain meaningful in black and white. The references are the
[AES Convention and Conference Author Guidelines][aes-convention] and the
[Journal Author Guidelines][aes-journal].

It is deliberately not a reproduction of the protected AES cover/header or publication
template. The AES guidelines reserve that graphical format, and the
[AES Publications Policy][aes-policy] states that preprints should not present themselves
in the published AES style.

[original]: https://pub.dega-akustik.de/DAGA_2012/data/articles/000281.pdf
[aes-convention]: https://secure.aes.org/authors/guidelines/
[aes-journal]: https://aes.org/publications/journal-of-the-audio-engineering-society/journal-author-guidelines/
[aes-policy]: https://aes.org/publications/publications-policy/
[paper-pdf]: https://github.com/cwbudde/algo-mixedphase/releases/latest/download/mixed-phase-filter-design-en.pdf
