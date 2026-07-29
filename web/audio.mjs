function tapEnergyGain(taps) {
  const energy = taps.reduce((sum, tap) => sum + tap * tap, 0);
  return Math.min(1 / Math.sqrt(Math.max(energy, 1e-4)), 4);
}

function transientBuffer(context) {
  const duration = 4;
  const sampleRate = context.sampleRate;
  const buffer = context.createBuffer(1, duration * sampleRate, sampleRate);
  const data = buffer.getChannelData(0);

  for (const time of [0.2, 1.2, 2.2, 3.2]) {
    const start = Math.round(time * sampleRate);
    data[start] = 0.95;

    const burstLength = Math.round(0.025 * sampleRate);
    for (let offset = 1; offset < burstLength; offset += 1) {
      const envelope = Math.exp(-offset / (sampleRate * 0.004));
      data[start + offset] +=
        0.22 * envelope * Math.sin((2 * Math.PI * 1900 * offset) / sampleRate);
    }
  }

  return buffer;
}

function impulseBuffer(context, taps) {
  const buffer = context.createBuffer(1, taps.length, context.sampleRate);
  buffer.copyToChannel(Float32Array.from(taps), 0);
  return buffer;
}

export class TransientAudition {
  constructor({ playButton, pathButtons, status }) {
    this.playButton = playButton;
    this.pathButtons = pathButtons;
    this.status = status;
    this.selected = "a";
    this.taps = null;
    this.context = null;
    this.source = null;
    this.gains = null;

    playButton.addEventListener("click", () => this.toggle());
    pathButtons.forEach((button) => {
      button.addEventListener("click", () =>
        this.select(button.dataset.audition),
      );
    });
  }

  setTaps(a, b) {
    const wasPlaying = Boolean(this.source);
    this.stop();
    this.taps = { a: [...a], b: [...b] };
    this.playButton.disabled = false;
    this.status.textContent = wasPlaying
      ? "Design changed · playback reset"
      : "Level-matched transient ready";
  }

  invalidate() {
    this.stop();
    this.taps = null;
    this.playButton.disabled = true;
    this.status.textContent = "Waiting for both designs";
  }

  async toggle() {
    if (this.source) {
      this.stop();
      this.status.textContent = "Stopped · timeline resets on next play";
      return;
    }

    if (!this.taps) {
      return;
    }

    const Context = window.AudioContext ?? window.webkitAudioContext;
    if (!Context) {
      this.status.textContent = "Web Audio is unavailable in this browser";
      return;
    }

    this.context ??= new Context();
    await this.context.resume();

    const source = this.context.createBufferSource();
    source.buffer = transientBuffer(this.context);
    source.loop = true;

    const master = this.context.createGain();
    master.gain.value = 0.16;
    master.connect(this.context.destination);

    const sourceGain = this.context.createGain();
    const aGain = this.context.createGain();
    const bGain = this.context.createGain();
    const convolverA = this.context.createConvolver();
    const convolverB = this.context.createConvolver();
    convolverA.normalize = false;
    convolverB.normalize = false;
    convolverA.buffer = impulseBuffer(this.context, this.taps.a);
    convolverB.buffer = impulseBuffer(this.context, this.taps.b);

    source.connect(sourceGain).connect(master);
    source.connect(convolverA).connect(aGain).connect(master);
    source.connect(convolverB).connect(bGain).connect(master);

    this.gains = {
      source: { node: sourceGain, level: 1 },
      a: { node: aGain, level: tapEnergyGain(this.taps.a) },
      b: { node: bGain, level: tapEnergyGain(this.taps.b) },
    };
    for (const [path, gain] of Object.entries(this.gains)) {
      gain.node.gain.value = path === this.selected ? gain.level : 0;
    }

    this.source = source;
    source.onended = () => {
      if (this.source === source) {
        this.source = null;
        this.gains = null;
        this.playButton.textContent = "Play transient";
      }
    };
    source.start();
    this.playButton.textContent = "Stop transient";
    this.status.textContent = `Playing ${this.selected === "source" ? "source" : this.selected.toUpperCase()} · switch freely`;
  }

  select(path) {
    this.selected = path;
    this.pathButtons.forEach((button) => {
      button.setAttribute(
        "aria-pressed",
        String(button.dataset.audition === this.selected),
      );
    });

    if (!this.gains || !this.context) {
      return;
    }

    const now = this.context.currentTime;
    for (const [name, gain] of Object.entries(this.gains)) {
      gain.node.gain.cancelScheduledValues(now);
      gain.node.gain.setTargetAtTime(
        name === path ? gain.level : 0,
        now,
        0.008,
      );
    }
    this.status.textContent = `Playing ${path === "source" ? "source" : path.toUpperCase()} · timeline preserved`;
  }

  stop() {
    if (!this.source) {
      return;
    }

    this.source.onended = null;
    this.source.stop();
    this.source.disconnect();
    this.source = null;
    this.gains = null;
    this.playButton.textContent = "Play transient";
  }
}
