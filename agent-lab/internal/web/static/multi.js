// agent-lab M7 Multi-Agent 面板: 4 列消息流 + 轮次状态.
(() => {
  const $goal = document.getElementById("multi-goal");
  const $rounds = document.getElementById("multi-rounds");
  const $btn = document.getElementById("multi-run-btn");
  const $status = document.getElementById("multi-status");
  const $statusBar = document.getElementById("multi-status-bar");
  const $roundInfo = document.getElementById("multi-round-info");
  const $tokenInfo = document.getElementById("multi-token-info");
  const $feedback = document.getElementById("multi-feedback");

  const cols = {
    researcher: document.querySelector("#col-researcher .multi-col-body"),
    writer: document.querySelector("#col-writer .multi-col-body"),
    critic: document.querySelector("#col-critic .multi-col-body"),
    compliance: document.querySelector("#col-compliance .multi-col-body"),
  };

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  function clearColumns() {
    for (const k in cols) cols[k].innerHTML = "";
    $feedback.innerHTML = "";
  }

  function addCard(col, round, text, approved, issues, error) {
    const card = document.createElement("div");
    card.className = "multi-card" + (approved ? " approved" : "") + (error ? " errored" : "");
    let html = '<div class="multi-card-round">R' + round + "</div>";
    if (text) {
      let preview = text.length > 200 ? text.slice(0, 200) + "..." : text;
      html += '<pre class="multi-card-text">' + escapeHtml(preview) + "</pre>";
    }
    if (approved) {
      html += '<span class="multi-badge ok">✓ approve</span>';
    }
    if (issues && issues.length > 0) {
      html += '<span class="multi-badge warn">' + issues.length + " issues</span>";
      html += '<ul class="multi-issues">' + issues.map((i) => "<li>" + escapeHtml(i) + "</li>").join("") + "</ul>";
    }
    if (error) {
      html += '<span class="multi-badge err">✗ ' + escapeHtml(error) + "</span>";
    }
    card.innerHTML = html;
    col.appendChild(card);
    col.scrollTop = col.scrollHeight;
  }

  async function run() {
    const goal = $goal.value.trim();
    if (!goal) return;
    $btn.disabled = true;
    $status.textContent = "协作中...";
    clearColumns();
    $statusBar.style.display = "flex";
    $roundInfo.textContent = "round 0";
    $tokenInfo.textContent = "0 tokens";
    let totalTokens = 0;

    try {
      const r = await fetch("/api/multi/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ goal, max_rounds: parseInt($rounds.value) || 4 }),
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
              evType = evType || ev.type;
              handleEvent(evType, ev, () => (totalTokens += ev.tokens || 0));
            } catch (_) {}
          }
        }
      }
    } catch (e) {
      $status.textContent = "失败: " + e.message;
    } finally {
      $btn.disabled = false;
    }
  }

  function handleEvent(type, ev, addTokens) {
    switch (type) {
      case "round_start":
        $roundInfo.textContent = "round " + ev.round;
        $feedback.innerHTML = "";
        break;
      case "agent_done":
        if (cols[ev.agent]) {
          addCard(cols[ev.agent], ev.round, ev.output, ev.approved, ev.issues, ev.error);
        }
        addTokens();
        $tokenInfo.textContent = (ev.total_tokens || 0) + " tokens";
        break;
      case "round_end":
        if (ev.feedback) {
          const fb = document.createElement("div");
          fb.className = "feedback-item";
          fb.innerHTML = '<span class="feedback-label">R' + ev.round + ' 反馈 → Writer:</span> ' + escapeHtml(ev.feedback);
          $feedback.appendChild(fb);
        }
        break;
      case "done":
        if (ev.error && ev.error.indexOf("stale") >= 0) {
          $status.textContent = "循环防御: " + ev.error;
        } else if (ev.error) {
          $status.textContent = "结束: " + ev.error;
        } else {
          $status.textContent = "全部通过 ✓ (" + ev.elapsed + ", " + ev.total_tokens + " tokens)";
        }
        $tokenInfo.textContent = (ev.total_tokens || 0) + " tokens";
        break;
      case "fail":
        $status.textContent = "失败: " + ev.error;
        break;
    }
  }

  $btn.addEventListener("click", run);
  $goal.addEventListener("keydown", (e) => {
    if (e.key === "Enter") run();
  });
})();
