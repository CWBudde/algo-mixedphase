import { TransientAudition } from "./audio.mjs";
import {
  METHODS,
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
  targetNote,
  usesCutoff,
} from "./lab-state.mjs";
import { ComparisonPlots } from "./plots.mjs";

const elements = {
  preset: document.getElementById("preset"),
  presetDescription: document.getElementById("presetDescription"),
  target: document.getElementById("target"),
  targetNote: document.getElementById("targetNote"),
  cutoffControl: document.querySelector('[data-common-control="cutoff"]'),
  length: document.getElementById("length"),
  cutoff: document.getElementById("cutoff"),
  lengthValue: document.getElementById("lengthValue"),
  cutoffValue: document.getElementById("cutoffValue"),
  swap: document.getElementById("swapDesigns"),
  metrics: document.getElementById("metricsBody"),
  globalStatus: document.getElementById("globalStatus"),
  copyLink: document.getElementById("copyLink"),
  exportData: document.getElementById("exportData"),
  exportCSV: document.getElementById("exportCSV"),
};

const slots = Object.fromEntries(
  ["a", "b"].map((slot) => [
    slot,
    {
      method: document.getElementById(`${slot}Method`),
      delay: document.getElementById(`${slot}Delay`),
      tolerance: document.getElementById(`${slot}Tolerance`),
      iterations: document.getElementById(`${slot}Iterations`),
      delayValue: document.getElementById(`${slot}DelayValue`),
      toleranceValue: document.getElementById(`${slot}ToleranceValue`),
      iterationsValue: document.getElementById(`${slot}IterationsValue`),
      delayControl: document.querySelector(
        `[data-slot-control="${slot}-delay"]`,
      ),
      toleranceControl: document.querySelector(
        `[data-slot-control="${slot}-tolerance"]`,
      ),
      name: document.getElementById(`${slot}Name`),
      status: document.getElementById(`${slot}Status`),
    },
  ]),
);

const style = getComputedStyle(document.documentElement);
const cssColour = (name, fallback) =>
  style.getPropertyValue(name).trim() || fallback;
const plots = new ComparisonPlots({
  canvases: {
    magnitude: document.getElementById("magnitude"),
    groupDelay: document.getElementById("groupDelay"),
    impulse: document.getElementById("impulse"),
  },
  readouts: {
    magnitude: document.getElementById("magnitudeReadout"),
    delay: document.getElementById("delayReadout"),
    impulse: document.getElementById("impulseReadout"),
  },
  colours: {
    target: cssColour("--target", "#a4aaa5"),
    a: cssColour("--a", "#59d8d1"),
    b: cssColour("--b", "#ffb45f"),
    grid: cssColour("--grid", "#29312d"),
    muted: cssColour("--muted", "#969d96"),
    faint: cssColour("--faint", "#69716b"),
    crosshair: cssColour("--rule-strong", "#465149"),
    prering: cssColour("--a-soft", "rgba(89, 216, 209, 0.08)"),
  },
});

const audio = new TransientAudition({
  playButton: document.getElementById("playAudio"),
  pathButtons: [...document.querySelectorAll("[data-audition]")],
  status: document.getElementById("audioStatus"),
});

const metricDefinitions = [
  {
    label: "RMS magnitude error",
    value: (result) => result.rmsErrorDB,
    format: (value) => `${value.toFixed(3)} dB`,
    delta: (value) => `${signed(value, 3)} dB`,
  },
  {
    label: "Peak magnitude error",
    value: (result) => result.maxErrorDB,
    format: (value) => `${value.toFixed(3)} dB`,
    delta: (value) => `${signed(value, 3)} dB`,
  },
  {
    label: "Energy centroid",
    value: (result) => result.energyCentroid,
    format: (value) => `${value.toFixed(2)} smp`,
    delta: (value) => `${signed(value, 2)} smp`,
  },
  {
    label: "Energy before peak",
    value: (result) => result.prePeakEnergy * 100,
    format: (value) => `${value.toFixed(2)}%`,
    delta: (value) => `${signed(value, 2)} pp`,
  },
  {
    label: "Iterations run",
    value: (result) => result.iterations,
    format: (value) => `${value}`,
    delta: (value) => signed(value, 0),
  },
  {
    label: "Browser runtime",
    value: (result) => result.runtimeMS,
    format: (value) => `${formatRuntime(value)} ms`,
    delta: (value) => `${signed(value, value < 10 ? 1 : 0)} ms`,
  },
];

let experiment = decodeExperiment(window.location.search);
let results = { a: null, b: null };
let engineReady = false;
let engineTargets = [];
const pending = new Set();
const timers = { a: null, b: null };
const gate = new RequestGate();
const worker = new Worker("design-worker.js");

function signed(value, digits) {
  if (!Number.isFinite(value)) {
    return "–";
  }

  return `${value >= 0 ? "+" : ""}${value.toFixed(digits)}`;
}

function formatRuntime(value) {
  if (value < 10) {
    return value.toFixed(1);
  }
  if (value < 100) {
    return value.toFixed(0);
  }
  return Math.round(value).toLocaleString();
}

function populateOptions() {
  elements.preset.replaceChildren(
    ...PRESETS.map((preset) => new Option(preset.label, preset.id)),
    new Option("Custom experiment", "custom"),
  );

  elements.target.replaceChildren(
    ...Object.entries(TARGETS).map(([value, label]) => new Option(label, value)),
  );

  for (const controls of Object.values(slots)) {
    controls.method.replaceChildren(
      ...Object.entries(METHODS).map(
        ([value, label]) => new Option(label, value),
      ),
    );
  }
}

function presetDescription() {
  return (
    PRESETS.find(({ id }) => id === experiment.preset)?.description ??
    "Custom settings are encoded in the URL as you work."
  );
}

function renderControls() {
  elements.preset.value = experiment.preset;
  elements.presetDescription.textContent = presetDescription();
  elements.target.value = experiment.target;
  elements.targetNote.textContent = targetNote(experiment.target);
  elements.length.value = experiment.length;
  elements.lengthValue.textContent = `${experiment.length} taps`;
  elements.cutoff.value = experiment.cutoff;
  elements.cutoffValue.textContent = experiment.cutoff.toFixed(2);
  // Only the adjustable low-pass is rebuilt from the cutoff; leaving a live
  // slider on a fixed benchmark prototype would invite the reading that it
  // moved the target.
  elements.cutoffControl.hidden = !usesCutoff(experiment.target);

  const maximumDelay = (experiment.length - 1) / 2;
  for (const [slot, controls] of Object.entries(slots)) {
    const design = experiment[slot];
    controls.method.value = design.method;
    controls.delay.max = String(maximumDelay);
    controls.delay.value = design.delay;
    controls.tolerance.value = design.tolerance;
    controls.iterations.value = design.iterations;
    controls.delayValue.textContent = `${design.delay} samples`;
    controls.toleranceValue.textContent = `${design.tolerance.toFixed(1)} dB`;
    controls.iterationsValue.textContent = design.iterations;
    controls.delayControl.hidden = design.method === "lowdelay";
    controls.toleranceControl.hidden = design.method !== "lowdelay";
    controls.name.textContent = METHODS[design.method];
  }
}

function setStatus(slot, state, message) {
  slots[slot].status.dataset.state = state;
  slots[slot].status.textContent = message;
}

function setGlobalStatus(state, message) {
  elements.globalStatus.dataset.state = state;
  elements.globalStatus.textContent = message;
}

function updateURL() {
  const query = encodeExperiment(experiment);
  window.history.replaceState(null, "", `${window.location.pathname}${query}`);
}

function invalidate(slot) {
  results = { ...results, [slot]: null };
  audio.invalidate();
  elements.exportData.disabled = true;
  elements.exportCSV.disabled = true;
  document.body.dataset.ready = "false";
  plots.update(results, experiment.length);
  renderMetrics();
}

function scheduleDesign(slot, immediate = false) {
  window.clearTimeout(timers[slot]);
  const identifier = gate.issue(slot);
  invalidate(slot);
  setStatus(slot, "computing", engineReady ? "Computing" : "Queued");
  setGlobalStatus("computing", engineReady ? "Updating" : "Starting engine");

  const send = () => {
    if (!engineReady) {
      pending.add(slot);
      return;
    }

    worker.postMessage({
      type: "design",
      slot,
      identifier,
      request: designRequest(experiment, slot),
    });
  };

  timers[slot] = window.setTimeout(send, immediate ? 0 : 90);
}

function scheduleBoth(immediate = false) {
  scheduleDesign("a", immediate);
  scheduleDesign("b", immediate);
}

function allResultsReady() {
  return Boolean(results.a && results.b);
}

function applyResult(slot, result, runtimeMS) {
  if (result.error) {
    setStatus(slot, "error", "Design failed");
    setGlobalStatus("error", "Check design settings");
    slots[slot].status.title = result.error;
    return;
  }

  const next = { ...result, runtimeMS };
  results = { ...results, [slot]: next };
  setStatus(slot, "ready", "Ready");
  slots[slot].status.title =
    `${result.iterations} iteration${result.iterations === 1 ? "" : "s"} run`;
  plots.update(results, experiment.length);
  renderMetrics();

  if (allResultsReady()) {
    setGlobalStatus("ready", "Results current");
    audio.setTaps(results.a.taps, results.b.taps);
    elements.exportData.disabled = false;
    elements.exportCSV.disabled = false;
    document.body.dataset.ready = "true";
  }
}

function renderMetrics() {
  const rows = metricDefinitions.map((metric) => {
    const a = results.a ? metric.value(results.a) : null;
    const b = results.b ? metric.value(results.b) : null;
    const difference = a !== null && b !== null ? b - a : null;
    const meaningful = difference !== null && Math.abs(difference) > 1e-12;

    const row = document.createElement("tr");
    const label = document.createElement("td");
    label.textContent = metric.label;
    const valueA = document.createElement("td");
    valueA.className = "metric-a";
    valueA.textContent = a === null ? "…" : metric.format(a);
    const valueB = document.createElement("td");
    valueB.className = "metric-b";
    valueB.textContent = b === null ? "…" : metric.format(b);
    const delta = document.createElement("td");
    delta.textContent = difference === null ? "…" : metric.delta(difference);

    if (meaningful) {
      valueA.dataset.better = String(a < b);
      valueB.dataset.better = String(b < a);
    }

    row.append(label, valueA, valueB, delta);
    return row;
  });

  elements.metrics.replaceChildren(...rows);
}

function mutateExperiment(next, affected = ["a", "b"], immediate = false) {
  experiment = normaliseExperiment(next);
  renderControls();
  updateURL();
  affected.forEach((slot) => scheduleDesign(slot, immediate));
}

function bindControls() {
  elements.preset.addEventListener("change", () => {
    if (elements.preset.value === "custom") {
      experiment = { ...experiment, preset: "custom" };
      renderControls();
      updateURL();
      return;
    }

    mutateExperiment(presetExperiment(elements.preset.value), ["a", "b"], true);
  });

  for (const [field, control] of [
    ["length", elements.length],
    ["cutoff", elements.cutoff],
  ]) {
    control.addEventListener("input", () => {
      mutateExperiment({
        ...experiment,
        preset: "custom",
        [field]: Number(control.value),
      });
    });
  }

  elements.target.addEventListener("change", () => {
    mutateExperiment(
      { ...experiment, preset: "custom", target: elements.target.value },
      ["a", "b"],
      true,
    );
  });

  for (const [slot, controls] of Object.entries(slots)) {
    for (const [field, control] of [
      ["method", controls.method],
      ["delay", controls.delay],
      ["tolerance", controls.tolerance],
      ["iterations", controls.iterations],
    ]) {
      control.addEventListener(field === "method" ? "change" : "input", () => {
        mutateExperiment(
          {
            ...experiment,
            preset: "custom",
            [slot]: {
              ...experiment[slot],
              [field]:
                field === "method" ? control.value : Number(control.value),
            },
          },
          [slot],
          field === "method",
        );
      });
    }
  }

  elements.swap.addEventListener("click", () => {
    mutateExperiment(swapDesigns(experiment), ["a", "b"], true);
  });

  document.querySelectorAll("[data-copy-from]").forEach((button) => {
    button.addEventListener("click", () => {
      const from = button.dataset.copyFrom;
      const to = from === "a" ? "b" : "a";
      mutateExperiment(copyDesign(experiment, from, to), [to], true);
    });
  });
}

function experimentExport() {
  return {
    schema: "algo-mixedphase/lab-experiment-v1",
    experiment,
    designs: {
      a: {
        config: designRequest(experiment, "a"),
        metrics: resultMetrics(results.a),
        taps: results.a.taps,
      },
      b: {
        config: designRequest(experiment, "b"),
        metrics: resultMetrics(results.b),
        taps: results.b.taps,
      },
    },
  };
}

function resultMetrics(result) {
  return {
    rmsMagnitudeErrorDB: result.rmsErrorDB,
    maxMagnitudeErrorDB: result.maxErrorDB,
    relativeMagnitudeError: result.relativeError,
    peakIndex: result.peakIndex,
    energyCentroid: result.energyCentroid,
    prePeakEnergyRatio: result.prePeakEnergy,
    iterations: result.iterations,
    browserRuntimeMS: result.runtimeMS,
  };
}

function download(name, content, type) {
  const link = document.createElement("a");
  const url = URL.createObjectURL(new Blob([content], { type }));
  link.href = url;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function plottedCSV() {
  const count = results.a.magnitudeDB.length;
  const rows = [
    [
      "normalised_frequency",
      "target_magnitude_db",
      "a_magnitude_db",
      "b_magnitude_db",
      "a_group_delay_samples",
      "b_group_delay_samples",
    ].join(","),
  ];

  for (let index = 0; index < count; index += 1) {
    rows.push(
      [
        ((0.5 * index) / (count - 1)).toFixed(9),
        results.a.referenceMagnitudeDB[index].toFixed(9),
        results.a.magnitudeDB[index].toFixed(9),
        results.b.magnitudeDB[index].toFixed(9),
        results.a.groupDelay[index].toFixed(9),
        results.b.groupDelay[index].toFixed(9),
      ].join(","),
    );
  }

  return `${rows.join("\n")}\n`;
}

function bindExports() {
  elements.copyLink.addEventListener("click", async () => {
    updateURL();
    const url = window.location.href;
    try {
      await navigator.clipboard.writeText(url);
      elements.copyLink.textContent = "Link copied";
    } catch {
      window.prompt("Copy this experiment URL", url);
    }
    window.setTimeout(() => {
      elements.copyLink.textContent = "Copy experiment link";
    }, 1600);
  });

  elements.exportData.addEventListener("click", () => {
    if (!allResultsReady()) {
      return;
    }
    download(
      "mixedphase-experiment.json",
      `${JSON.stringify(experimentExport(), null, 2)}\n`,
      "application/json",
    );
  });

  elements.exportCSV.addEventListener("click", () => {
    if (!allResultsReady()) {
      return;
    }
    download("mixedphase-plots.csv", plottedCSV(), "text/csv");
  });
}

worker.addEventListener("message", ({ data }) => {
  if (data.type === "ready") {
    engineReady = true;
    engineTargets = data.targets ?? [];
    setGlobalStatus("computing", "Engine ready");
    const queued = pending.size ? [...pending] : ["a", "b"];
    pending.clear();
    queued.forEach((slot) => scheduleDesign(slot, true));
    return;
  }

  if (data.type === "fatal") {
    setGlobalStatus("error", "Engine failed");
    for (const slot of ["a", "b"]) {
      setStatus(slot, "error", "Unavailable");
    }
    console.error(data.error);
    return;
  }

  if (data.type === "result" && gate.isCurrent(data.slot, data.identifier)) {
    applyResult(data.slot, data.result, data.runtimeMS);
  }
});

worker.addEventListener("error", (error) => {
  setGlobalStatus("error", "Worker failed");
  console.error(error);
});

populateOptions();
renderControls();
renderMetrics();
bindControls();
bindExports();
updateURL();
scheduleBoth(true);

window.__mixedphaseLab = {
  get experiment() {
    return structuredClone(experiment);
  },
  get results() {
    return structuredClone(results);
  },
  get targets() {
    return Object.keys(TARGETS);
  },
  get engineTargets() {
    return [...engineTargets];
  },
};
