export const METHODS = Object.freeze({
  iterative: "Alternating factorisation",
  interpolation: "Phase interpolation",
  minimax: "Least squares + minimax",
  lowdelay: "Magnitude-constrained low delay",
});

export const DEFAULT_EXPERIMENT = Object.freeze({
  preset: "daga-interpolation",
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
      length: 129,
      cutoff: 0.08,
      a: { method: "iterative", delay: 8, tolerance: 1, iterations: 12 },
      b: { method: "lowdelay", delay: 8, tolerance: 1, iterations: 60 },
    },
  },
]);

const LIMITS = Object.freeze({
  length: [33, 257],
  cutoff: [0.02, 0.4],
  delay: [0, 128],
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

function normaliseDesign(candidate, fallback, maximumDelay) {
  const method = Object.hasOwn(METHODS, candidate?.method)
    ? candidate.method
    : fallback.method;

  return {
    method,
    delay: Math.min(
      numberWithin(candidate?.delay, fallback.delay, LIMITS.delay, true),
      maximumDelay,
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

  const maximumDelay = (length - 1) / 2;

  return {
    preset: PRESETS.some(({ id }) => id === candidate.preset)
      ? candidate.preset
      : "custom",
    length,
    cutoff: numberWithin(
      candidate.cutoff,
      DEFAULT_EXPERIMENT.cutoff,
      LIMITS.cutoff,
    ),
    a: normaliseDesign(candidate.a, DEFAULT_EXPERIMENT.a, maximumDelay),
    b: normaliseDesign(candidate.b, DEFAULT_EXPERIMENT.b, maximumDelay),
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
