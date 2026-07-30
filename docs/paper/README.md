# Paper

This directory contains the source of the English paper accompanying the repository, which
presents fixed-support FIR design parameterised by requested group delay, and a short
companion account of the same results. The Typst sources are canonical; the PDFs are
generated build artifacts.

## Build

Install Typst 0.15.0 and run:

```bash
just paper
```

The result is `docs/paper/mixed-phase-filter-design-en.pdf`. The short companion account
builds separately with `just paper-brief`, producing
`docs/paper/mixed-phase-filter-design-brief-en.pdf`. Both are ignored by Git because
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

- `paper.typ` — the full paper: the delay-parameterised design, its measurements, and the
  reproducibility appendix.
- `paper_brief.typ` — a three-page account of the same results, reading the same artifact so
  the two cannot disagree on a number. Built by `just paper-brief` and format-checked in CI
  alongside the full paper.
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

The English manuscript is neither a revision nor a translation of the 2012 contribution: it
presents a different parameterisation and a reformulated sub-floor design, and treats the
2012 construction as one of the methods it compares against. It attributes the latency-budget
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
`just compare` regenerates the comparison CSVs, and named package tests carry the assertions
behind quoted numbers.

Normal paper builds consume committed data and never rerun timing benchmarks. The full paper
reads `docs/reference-continuum.csv`, `docs/reference-continuum-impulse.csv`,
`docs/reference-results.csv` and `docs/reference-phase-regimes.csv`; the brief reads the
continuum artifact alone. The continuum artifact carries one requested group delay per row
across both branches of the reachable window, with `predicted_delay` beside
`mean_group_delay` so the residual of the affine delay law is itself committed. The impulse
artifact records the low-pass at six positions along the continuum, which is what makes the
reversal into maximum phase visible. Only the explicit `just paper-refresh` command reruns
the generators.

Both numeric tables in the paper are computed from the CSVs at build time rather than typed
in, so no quoted figure in a table can drift from the artifact the figures draw. Numbers in
running prose are pinned by the named tests listed in the reproducibility appendix.

The reproducibility appendix maps every public design method, numbered equation, figure,
and table to its implementation, named tests, exact reference budget, artifact, and
regeneration command. Its notation table records the correspondence between paper symbols
and public Go configuration/result fields. The representative response CSVs and summary
CSVs are byte-compared against fresh generator output by the reference tests. The committed
quality data deliberately contain no wall-clock values; those live separately in
`docs/reference-timings.csv`.

The paper gives failure cases their own numbered table rather than leaving them as scattered
caveats: support-starved targets losing the accuracy structure, the approximate affine
inverse, phase unwrapping in the interior of the continuum, the local-optimum character of
the sub-floor solve, ripple growth below the floor, the alternating budget being a
magnitude control, and stopband group-delay masking. Each maps to a named regression test in
the reproducibility appendix.

The editorial audit of the superseded draft is recorded in
[`TECHNICAL_REVIEW.md`](TECHNICAL_REVIEW.md), which is marked as such and awaits a re-run
against the current manuscript. It checks historical attribution against the
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
