// agent-lab M2 Tools 面板交互: 工具试调用 + 最近调用列表
(() => {
  const $recentList = document.getElementById("recent-list");
  const $recentCount = document.getElementById("recent-count");

  async function loadRecent() {
    try {
      const r = await fetch("/api/tools/recent");
      const data = await r.json();
      renderRecent(data.invocations || []);
    } catch (_) { /* ignore */ }
  }

  function renderRecent(items) {
    $recentCount.textContent = items.length ? `(${items.length})` : "";
    $recentList.innerHTML = "";
    if (items.length === 0) {
      const li = document.createElement("li");
      li.className = "recent-empty";
      li.textContent = "还没有调用";
      $recentList.appendChild(li);
      return;
    }
    for (const it of items) {
      const li = document.createElement("li");
      li.className = "recent-item" + (it.error ? " err" : " ok");
      const head = document.createElement("div");
      head.className = "recent-head";
      const name = document.createElement("code");
      name.className = "tool-name";
      name.textContent = it.name;
      const meta = document.createElement("span");
      meta.className = "muted";
      const ts = new Date(it.started_at).toLocaleTimeString();
      meta.textContent = `${ts} · ${it.duration_ms}ms`;
      head.appendChild(name);
      head.appendChild(meta);

      const args = document.createElement("pre");
      args.className = "recent-args";
      args.textContent = "args: " + (it.args || "{}");

      const body = document.createElement("pre");
      body.className = "recent-body";
      body.textContent = it.error ? ("ERROR: " + it.error) : (it.result || "");

      li.appendChild(head);
      li.appendChild(args);
      li.appendChild(body);
      $recentList.appendChild(li);
    }
  }

  // 填示例按钮: 把 data-example 写进对应卡片的 textarea
  document.querySelectorAll(".fill-example").forEach((btn) => {
    btn.addEventListener("click", () => {
      const card = btn.closest("details");
      const ta = card.querySelector(".tool-args");
      ta.value = btn.dataset.example || "";
      ta.focus();
    });
  });

  // 试调用
  document.querySelectorAll(".invoke-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const name = btn.dataset.name;
      const card = btn.closest("details");
      const ta = card.querySelector(".tool-args");
      const status = card.querySelector(".invoke-status");
      const result = card.querySelector(".invoke-result");

      let args = {};
      const text = ta.value.trim();
      if (text) {
        try { args = JSON.parse(text); }
        catch (e) { status.textContent = "JSON 解析失败: " + e.message; return; }
      }

      btn.disabled = true;
      status.textContent = "调用中...";
      result.textContent = "";
      try {
        const r = await fetch("/api/tools/invoke", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name, args }),
        });
        const data = await r.json();
        if (data.ok) {
          status.textContent = `ok · ${data.duration_ms}ms`;
          result.textContent = pretty(data.result);
        } else {
          status.textContent = `error · ${data.duration_ms || 0}ms`;
          result.textContent = "ERR: " + (data.error || "unknown");
        }
      } catch (err) {
        status.textContent = "错误: " + err.message;
      } finally {
        btn.disabled = false;
        loadRecent();
      }
    });
  });

  function pretty(s) {
    if (!s) return "";
    try {
      const v = JSON.parse(s);
      return JSON.stringify(v, null, 2);
    } catch (_) {
      return s;
    }
  }

  loadRecent();
  // 每 5s 拉一次, 简单轮询.
  setInterval(loadRecent, 5000);
})();
