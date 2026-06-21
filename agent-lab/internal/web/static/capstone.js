// agent-lab M11 Capstone 面板: 一键生成多平台文案 + 评测.
(() => {
  const $seller = document.getElementById("cs-seller");
  const $sku = document.getElementById("cs-sku");
  const $platforms = document.getElementById("cs-platforms");
  const $style = document.getElementById("cs-style");
  const $btn = document.getElementById("cs-run-btn");
  const $status = document.getElementById("cs-status");
  const $eval = document.getElementById("cs-eval");
  const $output = document.getElementById("cs-output");

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  async function run() {
    $btn.disabled = true;
    $status.textContent = "生成中... (Multi-Agent 协作 + 评测, 约 10-30 秒)";
    $eval.style.display = "none";
    $output.innerHTML = "";

    const platforms = $platforms.value.split(",").map((s) => s.trim()).filter(Boolean);
    const body = {
      seller: $seller.value.trim(),
      sku_id: $sku.value.trim(),
      platforms,
      style: $style.value,
      max_rounds: 3,
    };

    try {
      const r = await fetch("/api/capstone/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
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
      $status.textContent = "失败: " + e.message;
    } finally {
      $btn.disabled = false;
    }
  }

  function handleEvent(type, ev) {
    switch (type) {
      case "start":
        $status.textContent = "Multi-Agent 协作中...";
        break;
      case "done":
        renderResult(ev.result);
        $status.textContent = "完成 ✓ (" + ev.result.duration + ")";
        break;
      case "error":
        $status.textContent = "失败: " + ev.error;
        break;
    }
  }

  function renderResult(result) {
    // 评测卡片.
    const sum = result.eval_summary || {};
    $eval.style.display = "flex";
    $eval.innerHTML = "";
    $eval.appendChild(evalCard(sum.mean_judge_score ? sum.mean_judge_score.toFixed(1) : "0", "Judge 均分"));
    $eval.appendChild(evalCard(Math.round((sum.mean_slang_hit || 0) * 100) + "%", "黑话命中"));
    $eval.appendChild(evalCard(Math.round((sum.compliance_rate || 0) * 100) + "%", "合规率"));
    if (result.multi_run) {
      $eval.appendChild(evalCard(result.multi_run.rounds, "协作轮次"));
      $eval.appendChild(evalCard(result.multi_run.total_tokens, "总 Token"));
    }

    // 多平台输出.
    $output.innerHTML = "";
    for (const o of result.outputs || []) {
      const card = document.createElement("div");
      card.className = "capstone-platform";
      let html = '<div class="capstone-platform-head"><h3>' + escapeHtml(o.name) + "</h3>";
      // 找对应评测.
      const ev = (result.eval_results || []).find((e) => e.platform === o.platform);
      if (ev) {
        html += '<span class="muted">Judge ' + ev.judge_score.toFixed(1) + " · " + (ev.compliance_ok ? "✓ 合规" : "✗ 违规") + "</span>";
      }
      html += "</div>";
      if (o.title) html += '<div class="capstone-platform-body" style="font-weight:700;margin-bottom:6px">' + escapeHtml(o.title) + "</div>";
      html += '<div class="capstone-platform-body">' + escapeHtml(o.body) + "</div>";
      if (o.tags) html += '<div class="capstone-platform-body muted" style="margin-top:6px;font-size:12px">' + escapeHtml(o.tags) + "</div>";
      card.innerHTML = html;
      $output.appendChild(card);
    }
  }

  function evalCard(val, label) {
    const card = document.createElement("div");
    card.className = "capstone-eval-card";
    card.innerHTML = '<div class="val">' + escapeHtml(String(val)) + '</div><div class="lbl">' + escapeHtml(label) + "</div>";
    return card;
  }

  $btn.addEventListener("click", run);
})();
