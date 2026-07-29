import assert from "node:assert/strict";

const debugPort = Number(process.argv[2]);
const pageURL = process.argv[3];

if (!Number.isInteger(debugPort) || !pageURL) {
  throw new Error("usage: node scripts/web-smoke.mjs DEBUG_PORT PAGE_URL");
}

const target = await fetch(
  `http://127.0.0.1:${debugPort}/json/new?${encodeURIComponent("about:blank")}`,
  { method: "PUT" },
).then((response) => response.json());
const socket = new WebSocket(target.webSocketDebuggerUrl);

await new Promise((resolve, reject) => {
  socket.addEventListener("open", resolve, { once: true });
  socket.addEventListener("error", reject, { once: true });
});

let nextIdentifier = 0;
const pending = new Map();
const browserErrors = [];

socket.addEventListener("message", ({ data }) => {
  const message = JSON.parse(data);
  if (message.id && pending.has(message.id)) {
    const { resolve, reject } = pending.get(message.id);
    pending.delete(message.id);
    if (message.error) {
      reject(new Error(message.error.message));
    } else {
      resolve(message.result);
    }
    return;
  }

  if (
    message.method === "Runtime.exceptionThrown" ||
    (message.method === "Log.entryAdded" &&
      message.params.entry.level === "error")
  ) {
    browserErrors.push(message);
  }
});

function command(method, params = {}) {
  nextIdentifier += 1;
  socket.send(JSON.stringify({ id: nextIdentifier, method, params }));
  return new Promise((resolve, reject) => {
    pending.set(nextIdentifier, { resolve, reject });
  });
}

async function evaluate(expression) {
  const result = await command("Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true,
  });
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text);
  }
  return result.result.value;
}

async function waitFor(expression, message, attempts = 120) {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    if (await evaluate(expression)) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`timed out waiting for ${message}`);
}

await command("Runtime.enable");
await command("Log.enable");
await command("Emulation.setDeviceMetricsOverride", {
  width: 1280,
  height: 900,
  deviceScaleFactor: 1,
  mobile: false,
});
await command("Page.navigate", { url: pageURL });
await waitFor("document.body?.dataset.ready === 'true'", "initial designs");

const initial = JSON.parse(
  await evaluate(`JSON.stringify({
    ready: document.body.dataset.ready,
    rows: document.querySelectorAll("#metricsBody tr").length,
    aMethod: document.querySelector("#aMethod").value,
    bMethod: document.querySelector("#bMethod").value,
    exportsEnabled:
      !document.querySelector("#exportData").disabled &&
      !document.querySelector("#exportCSV").disabled,
    metricsComplete: !document.querySelector("#metricsBody").textContent.includes("…"),
    paperHref: document.querySelector("[data-paper-link]").href,
    noOverflow: document.documentElement.scrollWidth <= window.innerWidth
  })`),
);
assert.deepEqual(initial, {
  ready: "true",
  rows: 6,
  aMethod: "iterative",
  bMethod: "interpolation",
  exportsEnabled: true,
  metricsComplete: true,
  paperHref:
    "https://github.com/cwbudde/algo-mixedphase/releases/latest/download/mixed-phase-filter-design-en.pdf",
  noOverflow: true,
});

// The page validates a shared URL before the engine loads, so it keeps its own
// copy of the target list. A copy that has drifted from the engine's would offer
// a target that fails to design, or hide one that works.
const targetLists = JSON.parse(
  await evaluate(`JSON.stringify({
    page: window.__mixedphaseLab.targets,
    engine: window.__mixedphaseLab.engineTargets
  })`),
);
assert.deepEqual(
  targetLists.page,
  targetLists.engine,
  "the page's target list has drifted from the WebAssembly engine's",
);
assert.ok(targetLists.engine.length > 1, "the engine published no benchmark targets");

for (const target of targetLists.engine) {
  await evaluate(`(() => {
    const select = document.querySelector("#target");
    select.value = ${JSON.stringify(target)};
    select.dispatchEvent(new Event("change"));
  })()`);
  await waitFor("document.body.dataset.ready === 'true'", `designs for ${target}`);

  const selected = JSON.parse(
    await evaluate(`JSON.stringify({
      target: window.__mixedphaseLab.experiment.target,
      status: document.querySelector("#globalStatus").dataset.state,
      metricsComplete: !document.querySelector("#metricsBody").textContent.includes("…"),
      cutoffHidden: document.querySelector('[data-common-control="cutoff"]').hidden,
      url: window.location.search
    })`),
  );

  assert.equal(selected.target, target);
  assert.equal(selected.status, "ready", `${target} did not design cleanly`);
  assert.equal(selected.metricsComplete, true, `${target} left metrics blank`);
  assert.equal(
    selected.cutoffHidden,
    target !== "lowpass",
    `${target} shows the wrong cutoff control`,
  );
  assert.match(selected.url, new RegExp(`target=${target}`));
}

await evaluate(`document.querySelector("#swapDesigns").click()`);
await waitFor("document.body.dataset.ready === 'false'", "swap invalidation");
await waitFor("document.body.dataset.ready === 'true'", "swapped designs");
const swapped = JSON.parse(
  await evaluate(`JSON.stringify({
    aMethod: document.querySelector("#aMethod").value,
    bMethod: document.querySelector("#bMethod").value,
    url: window.location.search
  })`),
);
assert.equal(swapped.aMethod, "interpolation");
assert.equal(swapped.bMethod, "iterative");
assert.match(swapped.url, /aMethod=interpolation/);
assert.match(swapped.url, /bMethod=iterative/);

await command("Emulation.setDeviceMetricsOverride", {
  width: 390,
  height: 844,
  deviceScaleFactor: 2,
  mobile: true,
});
const mobileHasNoOverflow = await evaluate(
  "document.documentElement.scrollWidth <= window.innerWidth",
);
assert.equal(mobileHasNoOverflow, true);
assert.deepEqual(browserErrors, []);

await command("Page.close");
socket.close();
console.log("Mixed Phase Lab browser smoke test passed");
