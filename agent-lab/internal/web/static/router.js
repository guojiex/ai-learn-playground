// agent-lab M10 Router 面板: 模型注册表 + 路由规则 + 最近调用.
(() => {
  const $registry = document.getElementById("router-registry");
  const $policy = document.getElementById("router-policy");
  const $recent = document.getElementById("router-recent");
  const $refresh = document.getElementById("router-refresh");

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function load() {
    try {
      const r = await fetch("/api/router", { cache: "no-store" });
      if (!r.ok) return;
      const data = await r.json();
      renderRegistry(data.registry);
      renderPolicy(data.policy);
      renderRecent(data.recent || []);
    } catch (e) {
      $registry.innerHTML = '<p class="muted">加载失败: ' + escapeHtml(e.message) + "</p>";
    }
  }

  function renderRegistry(reg) {
    if (!reg || !reg.models || reg.models.length === 0) {
      $registry.innerHTML = '<p class="muted">没有注册模型.</p>';
      return;
    }
    let html = '<table class="router-table"><thead><tr><th>模型</th><th>Base URL</th><th>Ctx</th><th>标签</th><th>TPS</th></tr></thead><tbody>';
    for (const m of reg.models) {
      const tags = (m.tags || []).map((t) => '<span class="layer-tag">' + escapeHtml(t) + "</span>").join(" ");
      html += '<tr><td><code>' + escapeHtml(m.name) + '</code></td><td class="muted">' + escapeHtml(m.base_url) + '</td><td>' + m.ctx + '</td><td>' + tags + '</td><td>' + (m.est_tps || 0) + '</td></tr>';
    }
    html += "</tbody></table>";
    $registry.innerHTML = html;
  }

  function renderPolicy(policy) {
    if (!policy || !policy.routes || policy.routes.length === 0) {
      $policy.innerHTML = '<p class="muted">没有路由规则.</p>';
      return;
    }
    let html = '<table class="router-table"><thead><tr><th>匹配条件</th><th>使用标签</th><th>降级链</th></tr></thead><tbody>';
    for (const r of policy.routes) {
      let match = "";
      if (r.match.task) match = 'task=' + escapeHtml(r.match.task);
      if (r.match.ctx_tokens_gt) match = 'ctx > ' + r.match.ctx_tokens_gt + " tokens";
      const fb = (r.fallback || []).map((f) => '<span class="layer-tag">' + escapeHtml(f) + "</span>").join(" ") || '<span class="muted">无</span>';
      html += '<tr><td>' + match + '</td><td><span class="layer-tag" style="background:#dbeafe;color:#1e3a8a">' + escapeHtml(r.use) + "</span></td><td>" + fb + "</td></tr>";
    }
    html += "</tbody></table>";
    $policy.innerHTML = html;
  }

  function renderRecent(records) {
    if (records.length === 0) {
      $recent.innerHTML = '<p class="muted">还没有调用记录. 在其他面板使用 agent 后, 路由决策会记录在这里.</p>';
      return;
    }
    let html = '<table class="router-table"><thead><tr><th>时间</th><th>Task</th><th>命中模型</th><th>降级链</th><th>耗时</th><th>状态</th></tr></thead><tbody>';
    for (const r of records) {
      const time = new Date(r.timestamp).toLocaleTimeString();
      const fb = (r.fallbacks || []).join(" → ") || '<span class="muted">-</span>';
      const status = r.success ? '<span class="risk-badge risk-low">✓</span>' : '<span class="risk-badge risk-high">✗</span>';
      html += '<tr><td class="muted">' + time + '</td><td>' + escapeHtml(r.task) + '</td><td><code>' + escapeHtml(r.chosen || "-") + '</code></td><td class="muted">' + fb + '</td><td>' + (r.latency_ms || 0) + 'ms</td><td>' + status + '</td></tr>';
    }
    html += "</tbody></table>";
    $recent.innerHTML = html;
  }

  $refresh.addEventListener("click", load);
  load();
})();
