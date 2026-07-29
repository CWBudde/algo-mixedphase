import assert from "node:assert/strict";
import test from "node:test";

import {
  LOWPASS_TARGET,
  PRESETS,
  RequestGate,
  TARGETS,
  copyDesign,
  decodeExperiment,
  designRequest,
  encodeExperiment,
  normaliseExperiment,
  presetExperiment,
  swapDesigns,
  usesCutoff,
} from "./lab-state.mjs";

test("URL state round-trips the complete experiment", () => {
  const experiment = presetExperiment("low-delay");
  const restored = decodeExperiment(encodeExperiment(experiment));

  assert.deepEqual(restored, experiment);
});

test("invalid URL values are bounded and replaced deterministically", () => {
  const restored = decodeExperiment(
    "?length=999&cutoff=nope&target=nope&aMethod=unknown&aDelay=-8&bTolerance=99",
  );

  assert.equal(restored.length, 257);
  assert.equal(restored.cutoff, 0.08);
  assert.equal(restored.target, LOWPASS_TARGET);
  assert.equal(restored.a.method, "iterative");
  assert.equal(restored.a.delay, 0);
  assert.equal(restored.b.tolerance, 12);
});

test("every benchmark target round-trips through the URL", () => {
  for (const target of Object.keys(TARGETS)) {
    const restored = decodeExperiment(
      encodeExperiment(normaliseExperiment({ target })),
    );

    assert.equal(restored.target, target, `target ${target} did not survive`);
    assert.equal(designRequest(restored, "a").target, target);
  }
});

test("only the adjustable low-pass is shaped by the cutoff", () => {
  assert.equal(usesCutoff(LOWPASS_TARGET), true);

  for (const target of Object.keys(TARGETS).filter((name) => name !== LOWPASS_TARGET)) {
    assert.equal(usesCutoff(target), false, `${target} should ignore the cutoff`);
  }
});

test("every preset names a published target", () => {
  for (const preset of PRESETS) {
    const experiment = presetExperiment(preset.id);

    assert.ok(
      Object.hasOwn(TARGETS, experiment.target),
      `preset ${preset.id} resolved to unknown target ${experiment.target}`,
    );
    // A preset whose target silently fell back to the default would still look
    // healthy here, so pin the declared value too.
    assert.equal(experiment.target, preset.experiment.target);
    assert.equal(experiment.preset, preset.id);
  }
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
