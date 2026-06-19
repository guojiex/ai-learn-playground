// agent-lab M6 Plan 面板: DAG 可视化 + SSE 执行进度.
(() => {
  const $goal = document.getElementById("plan-goal");
  const $genBtn = document.getElementById("plan-gen-btn");
  const $execBtn = document.getElementById("plan-exec-btn");
  const $status = document.getElementById("plan-status");
  const $dag = document.getElementById("plan-dag");
  const $timeline = document.getElementById("plan-timeline");

  let currentPlan = null;
  let taskMap = {};

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function generate() {
    const goal = $goal.value.trim();
    if (!goal) return;
    $genBtn.disabled = true;
    $status.textContent = "规划中...";
    $dag.innerHTML = "";
    $timeline.innerHTML = "";
    try {
      const r = await fetch("/api/plan/generate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ goal }),
      });
      if (!r.ok) {
        const err = await r.json().catch(() => ({ error: r.statusText }));
        throw new Error(err.error || "HTTP " + r.status);
      }
      const data = await r.json();
      currentPlan = data.plan;
      taskMap = {};
      for (const t of currentPlan.tasks) {
        taskMap[t.id] = { ...t, status: "pending" };
      }
      renderDAG(data.levels || []);
      $status.textContent = `计划已生成: ${currentPlan.tasks.length} 个任务`;
      $execBtn.disabled = false;
    } catch (e) {
      $status.textContent = "规划失败: " + e.message;
    } finally {
      $genBtn.disabled = false;
    }
  }

  function renderDAG(levels) {
    $dag.innerHTML = "";
    if (!levels || levels.length === 0) {
      $dag.innerHTML = '<p class="muted">还没有计划.</p>';
      return;
    }
    const container = document.createElement("div");
    container.className = "dag-levels";
    for (let li = 0; li < levels.length; li++) {
      const col = document.createElement("div");
      col.className = "dag-level";
      for (const tid of levels[li]) {
        const t = taskMap[tid];
        if (!t) continue;
        const node = document.createElement("div");
        node.className = "dag-node dag-" + (t.status || "pending");
        node.id = "dag-node-" + tid;
        let kind = t.tool ? "tool:" + t.tool : "agent:" + (t.agent || "?");
        let deps = t.depends && t.depends.length ? " ← " + t.depends.join(",") : "";
        node.innerHTML =
          '<div class="dag-node-id">' + escapeHtml(tid) + "</div>" +
          '<div class="dag-node-name">' + escapeHtml(t.name) + "</div>" +
          '<div class="dag-node-kind">' + escapeHtml(kind) + deps + "</div>" +
          '<div class="dag-node-status" id="dag-status-' + tid + '"></div>';
        col.appendChild(node);
      }
      container.appendChild(col);
      if (li < levels.length - 1) {
        const arrow = document.createElement("div");
        arrow.className = "dag-arrow";
        arrow.textContent = "→";
        container.appendChild(arrow);
      }
    }
    $dag.appendChild(container);
  }

  function updateNodeStatus(tid, status, output, error) {
    const node = document.getElementById("dag-node-" + tid);
    if (node) {
      node.className = "dag-node dag-" + status;
    }
    const statusEl = document.getElementById("dag-status-" + tid);
    if (statusEl) {
      let html = "";
      const icons = { ok: "✓", fail: "✗", running: "⟳", replan: "↻", skipped: "⊘", pending: "·" };
      html += '<span class="dag-icon">' + (icons[status] || "·") + "</span> " + status;
      if (output) {
        let preview = output.length > 120 ? output.slice(0, 120) + "..." : output;
        html += '<pre class="dag-output">' + escapeHtml(preview) + "</pre>";
      }
      if (error) {
        html += '<pre class="dag-error">' + escapeHtml(error) + "</pre>";
      }
      statusEl.innerHTML = html;
    }
  }

  function addTimeline(text, cls) {
    const item = document.createElement("div");
    item.className = "timeline-item " + (cls || "");
    const time = new Date().toLocaleTimeString();
    item.innerHTML = '<span class="timeline-time">' + time + "</span> " + escapeHtml(text);
    $timeline.insertBefore(item, $timeline.firstChild);
  }

  async function execute() {
    if (!currentPlan) return;
    $execBtn.disabled = true;
    $genBtn.disabled = true;
    $status.textContent = "执行中...";
    addTimeline("开始执行计划 (" + currentPlan.tasks.length + " tasks)", "start");

    try {
      const r = await fetch("/api/plan/execute", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan: currentPlan }),
      });
      if (!r.ok) throw new Error("HTTP " + r.status);

      const reader = r.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop();
        let evType = "";
        for (const line of lines) {
          if (line.startsWith("event:")) {
            evType = line.slice(6).trim();
          } else if (line.startsWith("data:")) {
            const payload = line.slice(5).trim();
            try {
              const ev = JSON.parse(payload);
              handleEvent(evType || ev.type, ev);
            } catch (_) {}
          }
        }
      }
    } catch (e) {
      $status.textContent = "执行失败: " + e.message;
      addTimeline("执行失败: " + e.message, "fail");
    } finally {
      $execBtn.disabled = false;
      $genBtn.disabled = false;
    }
  }

  function handleEvent(type, ev) {
    switch (type) {
      case "task_done":
        updateNodeStatus(ev.task_id, "ok", ev.output, "");
        addTimeline(ev.task_id + " " + ev.task_name + " ✓ (" + ev.elapsed + ")", "ok");
        break;
      case "task_fail":
        updateNodeStatus(ev.task_id, "fail", "", ev.error);
        addTimeline(ev.task_id + " " + ev.task_name + " ✗ " + ev.error, "fail");
        break;
      case "replan":
        addTimeline("重规划: " + ev.replan.failed_task + " 失败 → " + ev.replan.reason, "replan");
        break;
      case "plan_done":
        $status.textContent = "全部完成 ✓ (" + (ev.plan_run.total_tokens || 0) + " tokens)";
        addTimeline("计划执行完成", "ok");
        break;
      case "plan_fail":
        $status.textContent = "失败: " + ev.error;
        addTimeline("计划失败: " + ev.error, "fail");
        break;
    }
  }

  $genBtn.addEventListener("click", generate);
  $execBtn.addEventListener("click", execute);
  $goal.addEventListener("keydown", (e) => {
    if (e.key === "Enter") generate();
  });
})();
