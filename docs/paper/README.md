# Paper

This directory holds the paper this repository accompanies.

## What belongs here

- `daga-2012-mixed-phase.pdf` — the original DAGA 2012 contribution (German).
- `mixed-phase-filter-design-en.pdf` — the revised English version.

Drop the PDFs in and they are picked up by the links below; `treefmt.toml` already excludes
`**/*.pdf` from formatting, and nothing in CI touches them.

## Where they are referenced

Once a PDF is present, link it from:

- `README.md` — the "software companion to …" sentence in the intro.
- `web/index.html` — a link in the Mixed Phase Lab header, so the demo and the text sit
  next to each other.
- `docs/MIXED_PHASE_FILTER_DESIGN.md` — at the sections that implement a numbered method
  from the paper.

## Keeping paper and code in step

The point of this repository is that the paper's claims are re-runnable. When a figure or
table in the paper corresponds to a measurement here, note the command that reproduces it
next to the reference — `just compare` regenerates both example CSVs, and the package tests
carry the assertions behind the quoted numbers.
