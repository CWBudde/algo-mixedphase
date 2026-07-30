export const METHODS = Object.freeze({
  iterative: "Alternating factorisation",
  adaptive: "Alternating, selected delay",
  interpolation: "Phase interpolation",
  minimax: "Least squares + minimax",
  lowdelay: "Magnitude-constrained low delay",
});

// Methods that choose their own delay, so the delay slider is meaningless for
// them and the realised value is reported back instead.
export const CHOOSES_DELAY = Object.freeze(["adaptive", "lowdelay"]);

export function usesDelayControl(method) {
  return !CHOOSES_DELAY.includes(method);
}

// Methods whose phase is a prescribed curve rather than a support split. Their
// slider spans the whole continuum — minimum phase, through linear phase at the
// midpoint, to maximum phase at the end — so it reaches twice as far as the
// factorisation's, which stops once the linear factor has consumed the filter.
// Mirrors labresponse.PrescribesPhase; a mismatch would silently clamp.
export const PRESCRIBES_PHASE = Object.freeze(["interpolation", "minimax"]);

export function maximumDelayFor(method, length) {
  const linearPhase = (length - 1) / 2;

  return PRESCRIBES_PHASE.includes(method) ? 2 * linearPhase : linearPhase;
}

// LOWPASS_TARGET is the lab's own fixture and the only one whose shape the
// cutoff slider controls. Every other entry is a fixed comparison target taken
// from the published benchmark, so selecting one at the published tap and delay
// budget reproduces a row of docs/reference-results.csv.
export const LOWPASS_TARGET = "lowpass";

// TARGETS must list exactly what the WebAssembly engine accepts. The page needs
// its own copy because a shared URL is validated before the engine has loaded;
// the browser smoke test compares this list against mixedphaseLab.targets so
// the two cannot drift apart silently.
export const TARGETS = Object.freeze({
  [LOWPASS_TARGET]: "Adjustable low-pass",
  "low-pass": "Benchmark: first-order low-pass, 1 kHz",
  "parametric-eq": "Benchmark: parametric EQ, +9 dB at 3 kHz",
  crossover: "Benchmark: LR4 crossover, 2 kHz",
  "deep-notch": "Benchmark: −60 dB notch, 6 kHz",
  "room-correction": "Benchmark: room correction",
  "steep-crossover": "Benchmark: LR8 crossover, 800 Hz",
});

// The benchmark fixtures are 257-tap prototypes sampled at 48 kHz, so a
// normalised frequency f on the plots is f × 48 kHz. Only the adjustable
// low-pass is rebuilt from the tap budget and cutoff.
export const TARGET_NOTE = Object.freeze({
  [LOWPASS_TARGET]: "Hann-windowed sinc, rebuilt from the tap budget and cutoff.",
  default:
    "Fixed benchmark prototype: 257 taps at 48 kHz. The cutoff does not apply.",
});

export function targetNote(target) {
  return TARGET_NOTE[target] ?? TARGET_NOTE.default;
}

export function usesCutoff(target) {
  return target === LOWPASS_TARGET;
}

export const DEFAULT_EXPERIMENT = Object.freeze({
  preset: "daga-interpolation",
  target: LOWPASS_TARGET,
  length: 129,
  cutoff: 0.08,
  a: Object.freeze({
    method: "iterative",
    delay: 8,
    tolerance: 1,
    iterations: 12,
  }),
  b: Object.freeze({
    method: "interpolation",
    delay: 8,
    tolerance: 1,
    iterations: 12,
  }),
});

export const PRESETS = Object.freeze([
  {
    id: "daga-interpolation",
    label: "DAGA vs interpolation",
    description:
      "The paper method against the direct phase baseline at eight samples.",
    experiment: DEFAULT_EXPERIMENT,
  },
  {
    id: "delay-extremes",
    label: "Minimum vs linear phase",
    description:
      "The same magnitude response at opposite ends of the delay budget.",
    experiment: {
      preset: "delay-extremes",
      target: LOWPASS_TARGET,
      length: 129,
      cutoff: 0.08,
      a: { method: "interpolation", delay: 0, tolerance: 1, iterations: 12 },
      b: { method: "interpolation", delay: 64, tolerance: 1, iterations: 12 },
    },
  },
  {
    id: "minimax-budget",
    label: "Minimax iteration budget",
    description:
      "One Lawson pass against a larger peak-error refinement budget.",
    experiment: {
      preset: "minimax-budget",
      target: LOWPASS_TARGET,
      length: 129,
      cutoff: 0.08,
      a: { method: "minimax", delay: 8, tolerance: 1, iterations: 1 },
      b: { method: "minimax", delay: 8, tolerance: 1, iterations: 24 },
    },
  },
  {
    id: "low-delay",
    label: "Low-delay challenge",
    description: "Prescribed delay against a magnitude-constrained optimiser.",
    experiment: {
      preset: "low-delay",
      target: LOWPASS_TARGET,
      length: 129,
      cutoff: 0.08,
      a: { method: "iterative", delay: 8, tolerance: 1, iterations: 12 },
      b: { method: "lowdelay", delay: 8, tolerance: 1, iterations: 60 },
    },
  },
  {
    id: "support-starved",
    label: "Support-starved crossover",
    description:
      "The one benchmark target whose minimum-phase factor does not fit its tap share. At these settings both designs reproduce their published rows.",
    experiment: {
      preset: "support-starved",
      target: "steep-crossover",
      length: 129,
      cutoff: 0.08,
      a: { method: "iterative", delay: 16, tolerance: 1, iterations: 12 },
      b: { method: "interpolation", delay: 16, tolerance: 1, iterations: 12 },
    },
  },
  {
    id: "degeneracy",
    label: "Degeneracy check",
    description:
      "The same method with and without its delay budget. On a target the minimum-phase factor already fits, the two coincide up to that delay — which is what the budget bought.",
    experiment: {
      preset: "degeneracy",
      target: "crossover",
      length: 129,
      cutoff: 0.08,
      a: { method: "iterative", delay: 16, tolerance: 1, iterations: 12 },
      b: { method: "iterative", delay: 0, tolerance: 1, iterations: 12 },
    },
  },
]);

const LIMITS = Object.freeze({
  length: [33, 257],
  cutoff: [0.02, 0.4],
  // The upper bound is the longest support's maximum-phase end, 2*(257-1)/2.
  // The per-method ceiling in normaliseDesign is the one that actually binds;
  // this only rejects nonsense from a hand-edited URL.
  delay: [0, 256],
  tolerance: [0.1, 12],
  iterations: [1, 120],
});

function numberWithin(value, fallback, [minimum, maximum], integer = false) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }

  const clamped = Math.min(Math.max(parsed, minimum), maximum);
  return integer ? Math.round(clamped) : clamped;
}

function normaliseDesign(candidate, fallback, length) {
  const method = Object.hasOwn(METHODS, candidate?.method)
    ? candidate.method
    : fallback.method;

  return {
    method,
    // The ceiling depends on the method, because the prescribed-phase designs
    // read the slider as a position on the phase continuum and so run twice as
    // far as the factorisation's support split.
    delay: Math.min(
      numberWithin(candidate?.delay, fallback.delay, LIMITS.delay, true),
      maximumDelayFor(method, length),
    ),
    tolerance: numberWithin(
      candidate?.tolerance,
      fallback.tolerance,
      LIMITS.tolerance,
    ),
    iterations: numberWithin(
      candidate?.iterations,
      fallback.iterations,
      LIMITS.iterations,
      true,
    ),
  };
}

export function normaliseExperiment(candidate = {}) {
  let length = numberWithin(
    candidate.length,
    DEFAULT_EXPERIMENT.length,
    LIMITS.length,
    true,
  );
  length = 33 + Math.round((length - 33) / 8) * 8;

  return {
    preset: PRESETS.some(({ id }) => id === candidate.preset)
      ? candidate.preset
      : "custom",
    target: Object.hasOwn(TARGETS, candidate.target)
      ? candidate.target
      : DEFAULT_EXPERIMENT.target,
    length,
    cutoff: numberWithin(
      candidate.cutoff,
      DEFAULT_EXPERIMENT.cutoff,
      LIMITS.cutoff,
    ),
    a: normaliseDesign(candidate.a, DEFAULT_EXPERIMENT.a, length),
    b: normaliseDesign(candidate.b, DEFAULT_EXPERIMENT.b, length),
  };
}

export function presetExperiment(id) {
  const preset = PRESETS.find((candidate) => candidate.id === id) ?? PRESETS[0];
  return normaliseExperiment(preset.experiment);
}

export function decodeExperiment(search) {
  const parameters = new URLSearchParams(search);
  if (![...parameters.keys()].length) {
    return normaliseExperiment(DEFAULT_EXPERIMENT);
  }

  return normaliseExperiment({
    preset: parameters.get("preset") ?? "custom",
    target: parameters.get("target"),
    length: parameters.get("length"),
    cutoff: parameters.get("cutoff"),
    a: {
      method: parameters.get("aMethod"),
      delay: parameters.get("aDelay"),
      tolerance: parameters.get("aTolerance"),
      iterations: parameters.get("aIterations"),
    },
    b: {
      method: parameters.get("bMethod"),
      delay: parameters.get("bDelay"),
      tolerance: parameters.get("bTolerance"),
      iterations: parameters.get("bIterations"),
    },
  });
}

export function encodeExperiment(experiment) {
  const state = normaliseExperiment(experiment);
  const parameters = new URLSearchParams({
    preset: state.preset,
    target: state.target,
    length: String(state.length),
    cutoff: state.cutoff.toFixed(3).replace(/0+$/, "").replace(/\.$/, ""),
    aMethod: state.a.method,
    aDelay: String(state.a.delay),
    aTolerance: String(state.a.tolerance),
    aIterations: String(state.a.iterations),
    bMethod: state.b.method,
    bDelay: String(state.b.delay),
    bTolerance: String(state.b.tolerance),
    bIterations: String(state.b.iterations),
  });

  return `?${parameters.toString()}`;
}

export function swapDesigns(experiment) {
  return normaliseExperiment({
    ...experiment,
    preset: "custom",
    a: { ...experiment.b },
    b: { ...experiment.a },
  });
}

export function copyDesign(experiment, from, to) {
  if (!["a", "b"].includes(from) || !["a", "b"].includes(to)) {
    throw new TypeError("design slots must be a or b");
  }

  return normaliseExperiment({
    ...experiment,
    preset: "custom",
    [to]: { ...experiment[from] },
  });
}

export function designRequest(experiment, slot) {
  const state = normaliseExperiment(experiment);
  const design = state[slot];

  return {
    method: design.method,
    target: state.target,
    length: state.length,
    cutoff: state.cutoff,
    delay: design.delay,
    toleranceDB: design.tolerance,
    iterations: design.iterations,
  };
}

export class RequestGate {
  constructor() {
    this.sequences = { a: 0, b: 0 };
  }

  issue(slot) {
    this.sequences[slot] += 1;
    return this.sequences[slot];
  }

  isCurrent(slot, identifier) {
    return this.sequences[slot] === identifier;
  }
}
