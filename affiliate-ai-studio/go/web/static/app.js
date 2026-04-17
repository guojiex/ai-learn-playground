const STORAGE_KEY = "affiliate-ai-studio:last-input:v1";

const form = document.getElementById("run-form");
const resultPanel = document.getElementById("result-panel");
const retrievalPanel = document.getElementById("retrieval-panel");
const toolsPanel = document.getElementById("tools-panel");
const tracePanel = document.getElementById("trace-panel");

const FIELDS = [
  { name: "product_url", type: "text" },
  { name: "product_text", type: "text" },
  { name: "platform", type: "text" },
  { name: "locale", type: "text" },
  { name: "style", type: "text" },
  { name: "min_commission_rate", type: "number" },
  { name: "enable_compression", type: "checkbox" },
];

function pretty(value) {
  return JSON.stringify(value, null, 2);
}

function readForm() {
  if (!form) return {};
  const data = new FormData(form);
  const out = {};
  for (const field of FIELDS) {
    if (field.type === "checkbox") {
      const el = form.elements.namedItem(field.name);
      out[field.name] = el instanceof HTMLInputElement ? el.checked : false;
    } else if (field.type === "number") {
      const raw = data.get(field.name);
      out[field.name] = raw === null || raw === "" ? null : Number(raw);
    } else {
      out[field.name] = (data.get(field.name) || "").toString();
    }
  }
  return out;
}

function applyForm(values) {
  if (!form || !values) return;
  for (const field of FIELDS) {
    const el = form.elements.namedItem(field.name);
    if (!(el instanceof HTMLInputElement)) continue;
    const raw = values[field.name];
    if (raw === undefined || raw === null) continue;
    if (field.type === "checkbox") {
      el.checked = Boolean(raw);
    } else {
      el.value = String(raw);
    }
  }
}

function saveInput() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(readForm()));
  } catch (err) {
    console.warn("[studio] failed to persist form input", err);
  }
}

function loadInput() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw);
    applyForm(parsed);
  } catch (err) {
    console.warn("[studio] failed to restore form input", err);
  }
}

function resetInput() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch (err) {
    console.warn("[studio] failed to clear persisted input", err);
  }
  if (form) form.reset();
}

function buildPayload() {
  const values = readForm();
  return {
    product_url: values.product_url || "",
    product_text: values.product_text || "",
    platform: values.platform || "",
    locale: values.locale || "",
    style: values.style || "",
    min_commission_rate: Number.isFinite(values.min_commission_rate)
      ? values.min_commission_rate
      : 0,
    enable_compression: Boolean(values.enable_compression),
    debug: true,
  };
}

loadInput();

form?.addEventListener("input", saveInput);
form?.addEventListener("change", saveInput);

document
  .getElementById("reset-form")
  ?.addEventListener("click", () => {
    resetInput();
  });

form?.addEventListener("submit", async (event) => {
  event.preventDefault();
  saveInput();

  const payload = buildPayload();

  resultPanel.textContent = "运行中...";
  retrievalPanel.textContent = "运行中...";
  toolsPanel.textContent = "运行中...";
  tracePanel.textContent = "运行中...";

  try {
    const response = await fetch("/api/affiliate/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const json = await response.json();

    if (!response.ok) {
      resultPanel.textContent = pretty(json);
      retrievalPanel.textContent = "-";
      toolsPanel.textContent = "-";
      tracePanel.textContent = "-";
      return;
    }

    resultPanel.textContent = pretty(json.result);
    retrievalPanel.textContent = pretty(json.retrieval);
    toolsPanel.textContent = pretty(json.tools);
    tracePanel.textContent = pretty(json.trace);
  } catch (err) {
    resultPanel.textContent = `请求失败：${err?.message || err}`;
    retrievalPanel.textContent = "-";
    toolsPanel.textContent = "-";
    tracePanel.textContent = "-";
  }
});
