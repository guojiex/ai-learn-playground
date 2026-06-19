// agent-lab M5 Knowledge 面板: RAG 检索界面.
(() => {
  const $stats = document.getElementById("kb-stats");
  const $query = document.getElementById("kb-query");
  const $btn = document.getElementById("kb-search-btn");
  const $status = document.getElementById("kb-search-status");
  const $results = document.getElementById("kb-results");

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function loadStats() {
    try {
      const r = await fetch("/api/knowledge", { cache: "no-store" });
      if (!r.ok) return;
      const data = await r.json();
      const sources = data.sources || [];
      let html = `<span class="chip">文档块 <strong>${data.count || 0}</strong></span>`;
      html += `<span class="chip">维度 <strong>${data.dim || 0}</strong></span>`;
      html += `<span class="chip">来源 <strong>${sources.length}</strong></span>`;
      if (sources.length > 0) {
        html += '<ul class="kb-source-list">';
        for (const s of sources) {
          html += `<li><code>${escapeHtml(s.source)}</code> <span class="muted">${s.chunks} 块</span></li>`;
        }
        html += "</ul>";
      } else {
        html += '<p class="muted">知识库为空. 运行 <code>go run ./agent-lab/cmd/ingest -dir agent-lab/data/platform_rules</code> 导入文档.</p>';
      }
      $stats.innerHTML = html;
    } catch (e) {
      $stats.innerHTML = '<p class="muted">统计加载失败: ' + escapeHtml(e.message) + "</p>";
    }
  }

  async function search() {
    const query = $query.value.trim();
    if (!query) return;
    $btn.disabled = true;
    $status.textContent = "检索中...";
    $results.innerHTML = "";
    try {
      const r = await fetch("/api/knowledge", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query, k: 5 }),
      });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const data = await r.json();
      renderResults(data);
      $status.textContent = `找到 ${data.count} 条结果`;
    } catch (e) {
      $status.textContent = "检索失败: " + e.message;
    } finally {
      $btn.disabled = false;
    }
  }

  function renderResults(data) {
    const results = data.results || [];
    if (results.length === 0) {
      $results.innerHTML = '<p class="muted">没有找到相关文档.</p>';
      return;
    }
    let html = "";
    for (let i = 0; i < results.length; i++) {
      const r = results[i];
      const score = (r.score * 100).toFixed(1);
      html += `<div class="kb-result">`;
      html += `<div class="kb-result-head">`;
      html += `<span class="kb-rank">#${i + 1}</span>`;
      html += `<span class="kb-score">score ${score}%</span>`;
      html += `<code class="kb-source">${escapeHtml(r.source)}</code>`;
      html += `</div>`;
      html += `<pre class="kb-text">${escapeHtml(r.text)}</pre>`;
      html += `</div>`;
    }
    $results.innerHTML = html;
  }

  $btn.addEventListener("click", search);
  $query.addEventListener("keydown", (e) => {
    if (e.key === "Enter") search();
  });
  loadStats();
})();
