import assert from "node:assert/strict";
import test from "node:test";

import {
  RequestGate,
  copyDesign,
  decodeExperiment,
  encodeExperiment,
  normaliseExperiment,
  presetExperiment,
  swapDesigns,
} from "./lab-state.mjs";

test("URL state round-trips the complete experiment", () => {
  const experiment = presetExperiment("low-delay");
  const restored = decodeExperiment(encodeExperiment(experiment));

  assert.deepEqual(restored, experiment);
});

test("invalid URL values are bounded and replaced deterministically", () => {
  const restored = decodeExperiment(
    "?length=999&cutoff=nope&aMethod=unknown&aDelay=-8&bTolerance=99",
  );

  assert.equal(restored.length, 257);
  assert.equal(restored.cutoff, 0.08);
  assert.equal(restored.a.method, "iterative");
  assert.equal(restored.a.delay, 0);
  assert.equal(restored.b.tolerance, 12);
});

test("A/B designs can be copied and swapped without aliasing", () => {
  const original = presetExperiment("low-delay");
  const copied = copyDesign(original, "a", "b");
  const swapped = swapDesigns(original);

  assert.deepEqual(copied.a, copied.b);
  assert.notStrictEqual(copied.a, copied.b);
  assert.deepEqual(swapped.a, original.b);
  assert.deepEqual(swapped.b, original.a);
});

test("stale worker responses are rejected per slot", () => {
  const gate = new RequestGate();
  const firstA = gate.issue("a");
  const firstB = gate.issue("b");
  const secondA = gate.issue("a");

  assert.equal(gate.isCurrent("a", firstA), false);
  assert.equal(gate.isCurrent("a", secondA), true);
  assert.equal(gate.isCurrent("b", firstB), true);
});

test("tap lengths stay on the supported odd grid", () => {
  assert.equal(normaliseExperiment({ length: 34 }).length, 33);
  assert.equal(normaliseExperiment({ length: 130 }).length, 129);
});
