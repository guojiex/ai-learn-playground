// agent-lab M8 Approvals 面板: 待审批列表 + 详情 + approve/reject/edit.
(() => {
  const $summary = document.getElementById("approvals-summary");
  const $list = document.getElementById("approvals-list");
  const $detail = document.getElementById("approval-detail");

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function load() {
    try {
      const r = await fetch("/api/approvals", { cache: "no-store" });
      if (!r.ok) return;
      const data = await r.json();
      renderSummary(data.count);
      renderList(data.pending || []);
    } catch (e) {
      $summary.innerHTML = '<p class="muted">加载失败: ' + escapeHtml(e.message) + "</p>";
    }
  }

  function renderSummary(count) {
    if (count === 0) {
      $summary.innerHTML = '<span class="chip">待审批 <strong>0</strong></span><span class="muted">没有待处理的审批.</span>';
    } else {
      $summary.innerHTML = '<span class="chip" style="background:#dc3545;color:white">待审批 <strong>' + count + '</strong></span>';
    }
  }

  function renderList(approvals) {
    if (approvals.length === 0) {
      $list.innerHTML = "";
      $detail.innerHTML = '<p class="muted">选择一条审批查看详情, 或当前无待审批项.</p>';
      return;
    }
    let html = '<table class="approvals-table"><thead><tr><th>ID</th><th>风险</th><th>工具</th><th>会话</th><th>创建</th></tr></thead><tbody>';
    for (const a of approvals) {
      const riskClass = a.risk_level === "high" ? "risk-high" : (a.risk_level === "medium" ? "risk-medium" : "risk-low");
      const t = new Date(a.created_at * 1000).toLocaleString();
      html += '<tr class="approval-row" data-id="' + escapeHtml(a.id) + '">';
      html += '<td><code>' + escapeHtml(a.id) + '</code></td>';
      html += '<td><span class="risk-badge ' + riskClass + '">' + escapeHtml(a.risk_level) + '</span></td>';
      html += '<td>' + escapeHtml(a.tool) + '</td>';
      html += '<td><code>' + escapeHtml(a.conv_id) + '</code></td>';
      html += '<td class="muted">' + t + '</td>';
      html += '</tr>';
    }
    html += '</tbody></table>';
    $list.innerHTML = html;
    document.querySelectorAll(".approval-row").forEach((row) => {
      row.addEventListener("click", () => showDetail(row.dataset.id, approvals));
    });
  }

  function showDetail(id, approvals) {
    const a = approvals.find((x) => x.id === id);
    if (!a) return;
    let html = '<div class="detail-card">';
    html += '<div class="detail-head">';
    html += '<h3>' + escapeHtml(a.tool) + ' <span class="risk-badge risk-' + escapeHtml(a.risk_level) + '">' + escapeHtml(a.risk_level) + '</span></h3>';
    html += '<code class="muted">' + escapeHtml(a.id) + '</code>';
    html += '</div>';
    html += '<div class="detail-section"><label>参数 (args):<span class="tip" data-tip="工具调用的原始参数 JSON。点击下方「修改参数并批准」可编辑后放行"></span></label><pre>' + escapeHtml(prettyJSON(a.args)) + '</pre></div>';
    if (a.payload) {
      html += '<div class="detail-section"><label>Payload (dry-run):<span class="tip" data-tip="工具 dry-run 的执行摘要, 帮助你判断是否应该批准"></span></label><pre>' + escapeHtml(a.payload) + '</pre></div>';
    }
    html += '<div class="detail-actions">';
    html += '<button class="btn-approve" data-id="' + escapeHtml(a.id) + '">批准</button>';
    html += '<button class="btn-reject" data-id="' + escapeHtml(a.id) + '">拒绝</button>';
    html += '<button class="btn-edit" data-id="' + escapeHtml(a.id) + '">修改参数并批准</button>';
    html += '</div>';
    html += '<div class="detail-edit-box" id="edit-box-' + escapeHtml(a.id) + '" style="display:none">';
    html += '<label class="muted" style="font-size:11px;margin-bottom:4px">编辑参数 JSON<span class="tip" data-tip="修改工具调用的参数后批准。Agent 会用修改后的参数继续执行, 而非原始参数"></span></label>';
    html += '<textarea id="edit-args-' + escapeHtml(a.id) + '" rows="6">' + escapeHtml(prettyJSON(a.args)) + '</textarea>';
    html += '<button class="btn-save-edit" data-id="' + escapeHtml(a.id) + '">保存修改</button>';
    html += '</div>';
    html += '</div>';
    $detail.innerHTML = html;

    document.querySelector(".btn-approve").addEventListener("click", () => doAction(a.id, "approve", ""));
    document.querySelector(".btn-reject").addEventListener("click", () => {
      const note = prompt("拒绝原因:");
      if (note !== null) doAction(a.id, "reject", note);
    });
    document.querySelector(".btn-edit").addEventListener("click", () => {
      document.getElementById("edit-box-" + a.id).style.display = "block";
    });
    document.querySelector(".btn-save-edit").addEventListener("click", () => {
      const args = document.getElementById("edit-args-" + a.id).value;
      doAction(a.id, "edit", "edited", args);
    });
  }

  async function doAction(id, action, note, args) {
    const body = { id, action, note };
    if (args !== undefined) body.args = args;
    try {
      const r = await fetch("/api/approvals", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) {
        const err = await r.json().catch(() => ({ error: r.statusText }));
        alert("操作失败: " + err.error);
        return;
      }
      load();
    } catch (e) {
      alert("网络错误: " + e.message);
    }
  }

  function prettyJSON(s) {
    try {
      return JSON.stringify(JSON.parse(s), null, 2);
    } catch (_) {
      return s;
    }
  }

  load();
})();
