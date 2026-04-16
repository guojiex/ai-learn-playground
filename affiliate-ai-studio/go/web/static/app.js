const form = document.getElementById("run-form");
const resultPanel = document.getElementById("result-panel");
const retrievalPanel = document.getElementById("retrieval-panel");
const toolsPanel = document.getElementById("tools-panel");
const tracePanel = document.getElementById("trace-panel");

function pretty(value) {
  return JSON.stringify(value, null, 2);
}

form?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const data = new FormData(form);
  const payload = {
    product_url: data.get("product_url") || "",
    product_text: data.get("product_text") || "",
    platform: data.get("platform") || "",
    locale: data.get("locale") || "",
    style: data.get("style") || "",
    min_commission_rate: Number(data.get("min_commission_rate") || 0),
    enable_compression: data.get("enable_compression") === "on",
    debug: true,
  };

  resultPanel.textContent = "运行中...";
  retrievalPanel.textContent = "运行中...";
  toolsPanel.textContent = "运行中...";
  tracePanel.textContent = "运行中...";

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
});
