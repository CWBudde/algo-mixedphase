const AXIS_FONT = '10px "SFMono-Regular", "Cascadia Code", monospace';
const READOUT_FONT = '11px "SFMono-Regular", "Cascadia Code", monospace';

function finiteExtent(values, fallback) {
  const finite = values.filter(Number.isFinite);
  if (!finite.length) {
    return fallback;
  }

  return [Math.min(...finite), Math.max(...finite)];
}

function tickValues(minimum, maximum, count) {
  return Array.from(
    { length: count },
    (_, index) => minimum + ((maximum - minimum) * index) / (count - 1),
  );
}

function formatSigned(value, digits = 2) {
  if (!Number.isFinite(value)) {
    return "–";
  }

  return `${value >= 0 ? "+" : ""}${value.toFixed(digits)}`;
}

export class ComparisonPlots {
  constructor({ canvases, readouts, colours }) {
    this.canvases = canvases;
    this.readouts = readouts;
    this.colours = colours;
    this.results = { a: null, b: null };
    this.length = 129;
    this.crosshair = null;
    this.frames = new Map();

    for (const [name, canvas] of Object.entries(canvases)) {
      canvas.addEventListener("pointermove", (event) => {
        const frame = this.frames.get(name);
        if (!frame) {
          return;
        }

        this.crosshair = Math.min(
          Math.max((event.offsetX - frame.left) / frame.width, 0),
          1,
        );
        this.render();
      });
      canvas.addEventListener("pointerleave", () => {
        this.crosshair = null;
        this.render();
      });
      canvas.addEventListener("keydown", (event) => {
        if (
          !["ArrowLeft", "ArrowRight", "Home", "End", "Escape"].includes(
            event.key,
          )
        ) {
          return;
        }

        event.preventDefault();
        if (event.key === "Escape") {
          this.crosshair = null;
        } else if (event.key === "Home") {
          this.crosshair = 0;
        } else if (event.key === "End") {
          this.crosshair = 1;
        } else {
          const direction = event.key === "ArrowLeft" ? -1 : 1;
          this.crosshair = Math.min(
            Math.max((this.crosshair ?? 0.5) + direction / 128, 0),
            1,
          );
        }
        this.render();
      });
    }

    this.resizeObserver = new ResizeObserver(() => this.render());
    Object.values(canvases).forEach((canvas) =>
      this.resizeObserver.observe(canvas),
    );
  }

  update(results, length) {
    this.results = results;
    this.length = length;
    this.render();
  }

  prepare(name) {
    const canvas = this.canvases[name];
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    const width = Math.max(canvas.clientWidth, 320);
    const height = Math.max(canvas.clientHeight, 220);
    const pixelWidth = Math.round(width * ratio);
    const pixelHeight = Math.round(height * ratio);

    if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
      canvas.width = pixelWidth;
      canvas.height = pixelHeight;
    }

    const context = canvas.getContext("2d");
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, width, height);
    context.lineJoin = "round";
    context.lineCap = "round";

    const frame = {
      context,
      width: width - 60 - 18,
      height: height - 18 - 34,
      left: 60,
      top: 18,
      canvasWidth: width,
      canvasHeight: height,
    };
    this.frames.set(name, frame);
    return frame;
  }

  axes(
    frame,
    { xMin, xMax, yMin, yMax, xTicks, yTicks, xLabel, xFormat, yFormat },
  ) {
    const { context, left, top, width, height } = frame;
    context.save();
    context.strokeStyle = this.colours.grid;
    context.fillStyle = this.colours.muted;
    context.font = AXIS_FONT;
    context.lineWidth = 1;

    for (const tick of yTicks) {
      const y = top + height - ((tick - yMin) / (yMax - yMin)) * height;
      context.beginPath();
      context.moveTo(left, y);
      context.lineTo(left + width, y);
      context.stroke();
      context.textAlign = "right";
      context.textBaseline = "middle";
      context.fillText(yFormat(tick), left - 9, y);
    }

    for (const tick of xTicks) {
      const x = left + ((tick - xMin) / (xMax - xMin)) * width;
      context.beginPath();
      context.moveTo(x, top);
      context.lineTo(x, top + height);
      context.stroke();
      context.textAlign = "center";
      context.textBaseline = "top";
      context.fillText(xFormat(tick), x, top + height + 9);
    }

    context.fillStyle = this.colours.faint;
    context.textAlign = "right";
    context.fillText(xLabel, left + width, frame.canvasHeight - 5);
    context.restore();

    return {
      x: (value) => left + ((value - xMin) / (xMax - xMin)) * width,
      y: (value) => top + height - ((value - yMin) / (yMax - yMin)) * height,
    };
  }

  curve(frame, values, mapping, style, valid = () => true) {
    if (!values?.length) {
      return;
    }

    const { context } = frame;
    context.save();
    context.strokeStyle = style.colour;
    context.globalAlpha = style.alpha ?? 1;
    context.lineWidth = style.width ?? 1.6;
    context.setLineDash(style.dash ?? []);
    context.beginPath();

    let active = false;
    values.forEach((value, index) => {
      if (!Number.isFinite(value) || !valid(index)) {
        active = false;
        return;
      }

      const xValue = style.xValue
        ? style.xValue(index)
        : index / Math.max(values.length - 1, 1);
      const x = mapping.x(xValue);
      const y = mapping.y(Math.min(Math.max(value, style.yMin), style.yMax));

      if (!active) {
        context.moveTo(x, y);
        active = true;
      } else {
        context.lineTo(x, y);
      }
    });
    context.stroke();
    context.restore();
  }

  crosshairLine(frame) {
    if (this.crosshair === null) {
      return;
    }

    const x = frame.left + this.crosshair * frame.width;
    frame.context.save();
    frame.context.strokeStyle = this.colours.crosshair;
    frame.context.lineWidth = 1;
    frame.context.setLineDash([2, 4]);
    frame.context.beginPath();
    frame.context.moveTo(x, frame.top);
    frame.context.lineTo(x, frame.top + frame.height);
    frame.context.stroke();
    frame.context.restore();
  }

  render() {
    this.drawMagnitude();
    this.drawGroupDelay();
    this.drawImpulse();
  }

  drawMagnitude() {
    const frame = this.prepare("magnitude");
    const yMin = -100;
    const yMax = 5;
    const mapping = this.axes(frame, {
      xMin: 0,
      xMax: 0.5,
      yMin,
      yMax,
      xTicks: tickValues(0, 0.5, 6),
      yTicks: [-100, -80, -60, -40, -20, 0],
      xLabel: "normalised frequency",
      xFormat: (value) => value.toFixed(1),
      yFormat: (value) => `${value}`,
    });

    const reference =
      this.results.a?.referenceMagnitudeDB ??
      this.results.b?.referenceMagnitudeDB;
    this.curve(frame, reference, mapping, {
      colour: this.colours.target,
      dash: [3, 5],
      width: 1.2,
      yMin,
      yMax,
      xValue: (index) => (0.5 * index) / Math.max(reference.length - 1, 1),
    });
    this.curve(frame, this.results.a?.magnitudeDB, mapping, {
      colour: this.colours.a,
      width: 1.8,
      yMin,
      yMax,
      xValue: (index) =>
        (0.5 * index) /
        Math.max((this.results.a?.magnitudeDB.length ?? 1) - 1, 1),
    });
    this.curve(frame, this.results.b?.magnitudeDB, mapping, {
      colour: this.colours.b,
      dash: [8, 4],
      width: 1.8,
      yMin,
      yMax,
      xValue: (index) =>
        (0.5 * index) /
        Math.max((this.results.b?.magnitudeDB.length ?? 1) - 1, 1),
    });
    this.crosshairLine(frame);

    if (this.crosshair === null || (!this.results.a && !this.results.b)) {
      this.readouts.magnitude.textContent = "";
      return;
    }

    const count =
      this.results.a?.magnitudeDB.length ??
      this.results.b?.magnitudeDB.length ??
      1;
    const index = Math.round(this.crosshair * (count - 1));
    const frequency = this.crosshair * 0.5;
    const a = this.results.a?.magnitudeDB[index];
    const b = this.results.b?.magnitudeDB[index];
    this.readouts.magnitude.textContent =
      `${frequency.toFixed(3)} × fs · A ${a?.toFixed(2) ?? "–"} dB · ` +
      `B ${b?.toFixed(2) ?? "–"} dB`;
  }

  drawGroupDelay() {
    const frame = this.prepare("groupDelay");
    const maximumDelay = (this.length - 1) / 2;
    const yMin = -Math.max(8, maximumDelay * 0.25);
    const yMax = Math.max(16, maximumDelay * 1.25);
    const mapping = this.axes(frame, {
      xMin: 0,
      xMax: 0.5,
      yMin,
      yMax,
      xTicks: tickValues(0, 0.5, 6),
      yTicks: tickValues(yMin, yMax, 6),
      xLabel: "normalised frequency",
      xFormat: (value) => value.toFixed(1),
      yFormat: (value) => value.toFixed(0),
    });

    this.curve(
      frame,
      this.results.a?.groupDelay,
      mapping,
      {
        colour: this.colours.a,
        width: 1.8,
        yMin,
        yMax,
        xValue: (index) =>
          (0.5 * index) /
          Math.max((this.results.a?.groupDelay.length ?? 1) - 1, 1),
      },
      (index) => (this.results.a?.magnitudeDB[index] ?? -Infinity) > -60,
    );
    this.curve(
      frame,
      this.results.b?.groupDelay,
      mapping,
      {
        colour: this.colours.b,
        dash: [8, 4],
        width: 1.8,
        yMin,
        yMax,
        xValue: (index) =>
          (0.5 * index) /
          Math.max((this.results.b?.groupDelay.length ?? 1) - 1, 1),
      },
      (index) => (this.results.b?.magnitudeDB[index] ?? -Infinity) > -60,
    );
    this.crosshairLine(frame);

    if (this.crosshair === null || (!this.results.a && !this.results.b)) {
      this.readouts.delay.textContent = "";
      return;
    }

    const count =
      this.results.a?.groupDelay.length ??
      this.results.b?.groupDelay.length ??
      1;
    const index = Math.round(this.crosshair * (count - 1));
    const frequency = this.crosshair * 0.5;
    const a =
      (this.results.a?.magnitudeDB[index] ?? -Infinity) > -60
        ? this.results.a?.groupDelay[index]
        : undefined;
    const b =
      (this.results.b?.magnitudeDB[index] ?? -Infinity) > -60
        ? this.results.b?.groupDelay[index]
        : undefined;
    this.readouts.delay.textContent =
      `${frequency.toFixed(3)} × fs · A ${a?.toFixed(2) ?? "masked"} · ` +
      `B ${b?.toFixed(2) ?? "masked"} samples`;
  }

  cumulativeEnergy(result) {
    if (!result?.taps?.length) {
      return [];
    }

    const total = result.taps.reduce((sum, tap) => sum + tap * tap, 0) || 1;
    let accumulated = 0;
    return result.taps.map((tap) => {
      accumulated += tap * tap;
      return (100 * accumulated) / total;
    });
  }

  drawImpulse() {
    const frame = this.prepare("impulse");
    const halfDomain = this.length - 1;
    const allTaps = [
      ...(this.results.a?.taps ?? []),
      ...(this.results.b?.taps ?? []),
    ];
    const [, largest] = finiteExtent(allTaps.map(Math.abs), [0, 1]);
    const yMax = Math.max(largest * 1.08, 1e-6);
    const yMin = -yMax;
    const mapping = this.axes(frame, {
      xMin: -halfDomain,
      xMax: halfDomain,
      yMin,
      yMax,
      xTicks: tickValues(-halfDomain, halfDomain, 5),
      yTicks: tickValues(yMin, yMax, 5),
      xLabel: "samples relative to peak",
      xFormat: (value) => `${Math.round(value)}`,
      yFormat: (value) => value.toFixed(2),
    });

    const peakX = mapping.x(0);
    frame.context.save();
    frame.context.fillStyle = this.colours.prering;
    frame.context.fillRect(
      frame.left,
      frame.top,
      peakX - frame.left,
      frame.height,
    );
    frame.context.strokeStyle = this.colours.crosshair;
    frame.context.beginPath();
    frame.context.moveTo(peakX, frame.top);
    frame.context.lineTo(peakX, frame.top + frame.height);
    frame.context.stroke();
    frame.context.restore();

    for (const [slot, style] of [
      ["a", { colour: this.colours.a }],
      ["b", { colour: this.colours.b, dash: [8, 4] }],
    ]) {
      const result = this.results[slot];
      if (!result) {
        continue;
      }

      this.curve(frame, result.taps, mapping, {
        ...style,
        width: 1.5,
        yMin,
        yMax,
        xValue: (index) => index - result.peakIndex,
      });

      const energy = this.cumulativeEnergy(result);
      this.curve(
        frame,
        energy,
        {
          x: mapping.x,
          y: (value) => frame.top + frame.height - (value / 100) * frame.height,
        },
        {
          ...style,
          dash: [2, 5],
          width: 1.1,
          alpha: 0.62,
          yMin: 0,
          yMax: 100,
          xValue: (index) => index - result.peakIndex,
        },
      );
    }

    frame.context.save();
    frame.context.fillStyle = this.colours.faint;
    frame.context.font = AXIS_FONT;
    frame.context.textAlign = "right";
    frame.context.textBaseline = "top";
    frame.context.fillText(
      "100% energy",
      frame.left + frame.width,
      frame.top + 5,
    );
    frame.context.restore();
    this.crosshairLine(frame);

    if (this.crosshair === null || (!this.results.a && !this.results.b)) {
      this.readouts.impulse.textContent = "";
      return;
    }

    const relativeSample = Math.round(
      -halfDomain + this.crosshair * 2 * halfDomain,
    );
    const sample = (result) => {
      const index = relativeSample + result.peakIndex;
      return index >= 0 && index < result.taps.length
        ? result.taps[index]
        : undefined;
    };
    const a = this.results.a ? sample(this.results.a) : undefined;
    const b = this.results.b ? sample(this.results.b) : undefined;
    this.readouts.impulse.textContent =
      `${formatSigned(relativeSample, 0)} samples · A ${formatSigned(a, 4)} · ` +
      `B ${formatSigned(b, 4)}`;
  }
}
