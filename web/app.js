// Mixed Phase Lab — drives the Go designs through WebAssembly and plots the result.

const controls = {
  method: document.getElementById("method"),
  length: document.getElementById("length"),
  cutoff: document.getElementById("cutoff"),
  delay: document.getElementById("delay"),
  tolerance: document.getElementById("tolerance"),
  iterations: document.getElementById("iterations"),
};

const outputs = {
  length: document.getElementById("lengthValue"),
  cutoff: document.getElementById("cutoffValue"),
  delay: document.getElementById("delayValue"),
  tolerance: document.getElementById("toleranceValue"),
  iterations: document.getElementById("iterationsValue"),
  rms: document.getElementById("rms"),
  peak: document.getElementById("peak"),
  centroid: document.getElementById("centroid"),
  prepeak: document.getElementById("prepeak"),
  iterationsRun: document.getElementById("iterationsRun"),
  status: document.getElementById("status"),
};

const canvases = {
  magnitude: document.getElementById("magnitude"),
  groupDelay: document.getElementById("groupDelay"),
  impulse: document.getElementById("impulse"),
};

const style = getComputedStyle(document.documentElement);
const colours = {
  line: style.getPropertyValue("--accent").trim() || "#4f9cf9",
  reference: style.getPropertyValue("--muted").trim() || "#8a94a6",
  grid: style.getPropertyValue("--grid").trim() || "#2a3140",
  mark: style.getPropertyValue("--warn").trim() || "#f2a33c",
};

let ready = false;

async function boot() {
  const go = new Go();
  const source = await WebAssembly.instantiateStreaming(
    fetch("mixedphase_lab.wasm"),
    go.importObject,
  );

  go.run(source.instance);
  ready = true;
  outputs.status.textContent = "";
  render();
}

// The delay control only applies to the prescribing methods; the low-delay
// design chooses its own delay and is steered by its magnitude tolerance instead.
function syncControlVisibility() {
  const lowDelay = controls.method.value === "lowdelay";
  document.querySelector('[data-for="delay"]').hidden = lowDelay;
  document.querySelector('[data-for="tolerance"]').hidden = !lowDelay;
}

function render() {
  if (!ready) {
    return;
  }

  const length = Number(controls.length.value);
  const maximumDelay = (length - 1) / 2;
  controls.delay.max = String(maximumDelay);

  outputs.length.textContent = length;
  outputs.cutoff.textContent = Number(controls.cutoff.value).toFixed(2);
  outputs.delay.textContent = controls.delay.value;
  outputs.tolerance.textContent = Number(controls.tolerance.value).toFixed(1);
  outputs.iterations.textContent = controls.iterations.value;

  const result = window.mixedphaseLab.designMixedPhase({
    method: controls.method.value,
    length,
    cutoff: Number(controls.cutoff.value),
    delay: Number(controls.delay.value),
    toleranceDB: Number(controls.tolerance.value),
    iterations: Number(controls.iterations.value),
  });

  if (result.error) {
    outputs.status.textContent = result.error;
    return;
  }

  outputs.status.textContent = "";
  outputs.rms.textContent = `${result.rmsErrorDB.toFixed(3)} dB`;
  outputs.peak.textContent = `${result.maxErrorDB.toFixed(3)} dB`;
  outputs.centroid.textContent = `${result.energyCentroid.toFixed(2)} samples`;
  outputs.prepeak.textContent = `${(result.prePeakEnergy * 100).toFixed(2)} %`;
  outputs.iterationsRun.textContent = result.iterations;

  drawCurve(canvases.magnitude, result.magnitudeDB, {
    reference: result.referenceMagnitudeDB,
    min: -120,
    max: 10,
  });
  drawCurve(canvases.groupDelay, result.groupDelay, {});
  drawStems(canvases.impulse, result.taps, result.peakIndex);
}

function drawCurve(canvas, values, { reference, min, max }) {
  const context = prepare(canvas);
  const low = min ?? Math.min(...values);
  const high = max ?? Math.max(...values);

  drawGrid(context, canvas);

  if (reference) {
    context.setLineDash([4, 4]);
    context.strokeStyle = colours.reference;
    plot(context, canvas, reference, low, high);
    context.setLineDash([]);
  }

  context.strokeStyle = colours.line;
  plot(context, canvas, values, low, high);
}

function plot(context, canvas, values, low, high) {
  const span = high - low || 1;

  context.beginPath();
  values.forEach((value, index) => {
    const x = (index / (values.length - 1)) * canvas.width;
    const clamped = Math.min(Math.max(value, low), high);
    const y = canvas.height - ((clamped - low) / span) * canvas.height;

    if (index === 0) {
      context.moveTo(x, y);
    } else {
      context.lineTo(x, y);
    }
  });
  context.stroke();
}

function drawStems(canvas, taps, peakIndex) {
  const context = prepare(canvas);
  const scale = Math.max(...taps.map(Math.abs)) || 1;
  const middle = canvas.height / 2;

  drawGrid(context, canvas);

  context.strokeStyle = colours.line;
  context.beginPath();
  taps.forEach((tap, index) => {
    const x = (index / (taps.length - 1)) * canvas.width;
    context.moveTo(x, middle);
    context.lineTo(x, middle - (tap / scale) * (middle - 4));
  });
  context.stroke();

  const peakX = (peakIndex / (taps.length - 1)) * canvas.width;
  context.strokeStyle = colours.mark;
  context.beginPath();
  context.moveTo(peakX, 0);
  context.lineTo(peakX, canvas.height);
  context.stroke();
}

function prepare(canvas) {
  const context = canvas.getContext("2d");
  context.clearRect(0, 0, canvas.width, canvas.height);
  context.lineWidth = 1.25;

  return context;
}

function drawGrid(context, canvas) {
  context.strokeStyle = colours.grid;
  context.beginPath();

  for (let i = 1; i < 4; i += 1) {
    const y = (canvas.height * i) / 4;
    context.moveTo(0, y);
    context.lineTo(canvas.width, y);
  }

  context.stroke();
}

Object.values(controls).forEach((control) => {
  control.addEventListener("input", () => {
    syncControlVisibility();
    render();
  });
});

syncControlVisibility();
boot().catch((error) => {
  outputs.status.textContent = `Failed to load WebAssembly: ${error}`;
});
