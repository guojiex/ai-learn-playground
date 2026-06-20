// agent-lab M9 Traces 面板: trace 列表 + span 时间线.
(() => {
  const $list = document.getElementById("traces-list");
  const $detail = document.getElementById("trace-detail");
  const $refresh = document.getElementById("traces-refresh");

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function loadList() {
    try {
      const r = await fetch("/api/traces", { cache: "no-store" });
      if (!r.ok) return;
      const data = await r.json();
      renderList(data.traces || []);
    } catch (e) {
      $list.innerHTML = '<p class="muted">加载失败: ' + escapeHtml(e.message) + "</p>";
    }
  }

  function renderList(traces) {
    if (traces.length === 0) {
      $list.innerHTML = '<p class="muted">没有 trace 记录. 在 /chat 或 /multi 发起对话后会产生 trace.</p>';
      return;
    }
    let html = "";
    for (const t of traces) {
      const statusCls = t.status === "ok" ? "trace-ok" : (t.status === "fail" ? "trace-fail" : "trace-running");
      const time = new Date(t.started_at * 1000).toLocaleTimeString();
      const goal = escapeHtml(t.goal || t.conv_id || t.trace_id).slice(0, 30);
      html += '<div class="trace-item ' + statusCls + '" data-id="' + escapeHtml(t.trace_id) + '">';
      html += '<div class="trace-item-goal">' + goal + "</div>";
      html += '<div class="trace-item-meta"><span class="trace-status">' + escapeHtml(t.status) + '</span> · ' + time + "</div>";
      html += "</div>";
    }
    $list.innerHTML = html;
    document.querySelectorAll(".trace-item").forEach((el) => {
      el.addEventListener("click", () => loadDetail(el.dataset.id));
    });
  }

  async function loadDetail(traceID) {
    try {
      const r = await fetch("/api/traces?id=" + encodeURIComponent(traceID), { cache: "no-store" });
      if (!r.ok) return;
      const t = await r.json();
      renderDetail(t);
    } catch (e) {
      $detail.innerHTML = '<p class="muted">加载失败: ' + escapeHtml(e.message) + "</p>";
    }
  }

  function renderDetail(t) {
    const spans = t.spans || [];
    let html = '<div class="trace-detail-head">';
    html += "<h3>" + escapeHtml(t.goal || t.trace_id) + "</h3>";
    html += '<span class="trace-status">' + escapeHtml(t.status) + "</span>";
    html += "</div>";
    html += '<div class="trace-meta">';
    html += "<span><strong>Trace:</strong> <code>" + escapeHtml(t.trace_id) + "</code></span>";
    if (t.conv_id) html += "<span><strong>Conv:</strong> <code>" + escapeHtml(t.conv_id) + "</code></span>";
    html += "</div>";

    if (spans.length === 0) {
      html += '<p class="muted">没有 spans.</p>';
    } else {
      html += '<div class="spans-timeline">';
      for (const s of spans) {
        const kindCls = "span-" + s.kind;
        const dur = s.ended_at > 0 ? ((s.ended_at - s.started_at) / 1000).toFixed(2) + "s" : "running";
        const tokens = (s.tokens_in + s.tokens_out) > 0 ? (s.tokens_in + "/" + s.tokens_out) : "";
        html += '<div class="span-row ' + kindCls + '">';
        html += '<span class="span-kind">' + escapeHtml(s.kind) + "</span>";
        html += '<span class="span-name">' + escapeHtml(s.name) + "</span>";
        html += '<span class="span-dur">' + dur + "</span>";
        if (tokens) html += '<span class="span-tokens">' + tokens + " tok</span>";
        html += "</div>";
        if (s.input || s.output) {
          html += '<div class="span-io">';
          if (s.input) html += '<details><summary>input</summary><pre>' + escapeHtml(truncate(s.input, 300)) + "</pre></details>";
          if (s.output) html += '<details><summary>output</summary><pre>' + escapeHtml(truncate(s.output, 300)) + "</pre></details>";
          html += "</div>";
        }
        if (s.error) {
          html += '<div class="span-error">' + escapeHtml(s.error) + "</div>";
        }
      }
      html += "</div>";
    }
    $detail.innerHTML = html;
  }

  function truncate(s, n) {
    if (!s) return "";
    return s.length > n ? s.slice(0, n) + "..." : s;
  }

  $refresh.addEventListener("click", loadList);
  loadList();
})();
