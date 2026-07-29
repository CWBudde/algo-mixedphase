# External paper figures

The benchmark, magnitude-response, group-delay, and peak-aligned impulse charts are
generated inside `charts.typ` directly from committed CSV artifacts, so a normal paper
build does not write files here. This directory is reserved for future plots that cannot be
expressed by the native Typst chart helpers.

Do not edit generated plots by hand. Each future generator must document its input CSV,
configuration budget, and regeneration command.
