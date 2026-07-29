importScripts("wasm_exec.js");

let ready;

async function instantiate() {
  const go = new Go();
  const response = await fetch("mixedphase_lab.wasm");
  let instance;

  if (WebAssembly.instantiateStreaming) {
    try {
      instance = await WebAssembly.instantiateStreaming(
        response.clone(),
        go.importObject,
      );
    } catch {
      const bytes = await response.arrayBuffer();
      instance = await WebAssembly.instantiate(bytes, go.importObject);
    }
  } else {
    const bytes = await response.arrayBuffer();
    instance = await WebAssembly.instantiate(bytes, go.importObject);
  }

  go.run(instance.instance);

  for (let attempt = 0; attempt < 100; attempt += 1) {
    if (globalThis.mixedphaseLab) {
      postMessage({ type: "ready" });
      return;
    }

    await new Promise((resolve) => setTimeout(resolve, 10));
  }

  throw new Error("WebAssembly API did not become ready");
}

ready = instantiate().catch((error) => {
  postMessage({ type: "fatal", error: String(error) });
  throw error;
});

self.addEventListener("message", async ({ data }) => {
  if (data.type !== "design") {
    return;
  }

  try {
    await ready;
    const started = performance.now();
    const result = globalThis.mixedphaseLab.designMixedPhase(data.request);
    const runtimeMS = performance.now() - started;

    postMessage({
      type: "result",
      slot: data.slot,
      identifier: data.identifier,
      result,
      runtimeMS,
    });
  } catch (error) {
    postMessage({
      type: "result",
      slot: data.slot,
      identifier: data.identifier,
      result: { error: String(error) },
      runtimeMS: 0,
    });
  }
});
