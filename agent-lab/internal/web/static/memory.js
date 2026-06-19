// agent-lab M4 Memory 面板: 浏览长期记忆 KV (按 namespace 折叠), 支持遗忘单条.
(() => {
  const $tree = document.getElementById("mem-tree");
  const $status = document.getElementById("mem-status");
  const $refresh = document.getElementById("mem-refresh");

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  function prettyJson(s) {
    try {
      return JSON.stringify(JSON.parse(s), null, 2);
    } catch (_) {
      return s;
    }
  }

  function fmtTime(unix) {
    if (!unix) return "";
    const d = new Date(unix * 1000);
    return d.toLocaleString();
  }

  async function load() {
    $status.textContent = "加载中...";
    try {
      const r = await fetch("/api/memory", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const data = await r.json();
      render(data.namespaces || []);
      $status.textContent = "";
    } catch (e) {
      $status.textContent = "加载失败: " + e.message;
    }
  }

  function render(namespaces) {
    $tree.innerHTML = "";
    if (namespaces.length === 0) {
      const empty = document.createElement("p");
      empty.className = "muted";
      empty.textContent = "还没有长期记忆. 在 Chat 里让 agent 记住某个卖家的偏好 (例如 \"我喜欢闺蜜风\"), 这里就会出现.";
      $tree.appendChild(empty);
      return;
    }
    for (const ns of namespaces) {
      const details = document.createElement("details");
      details.className = "mem-namespace";
      details.open = true;
      const summary = document.createElement("summary");
      summary.innerHTML = `<code class="mem-ns-name">${escapeHtml(ns.namespace)}</code> <span class="muted">(${ns.entries.length})</span>`;
      details.appendChild(summary);

      const ul = document.createElement("ul");
      ul.className = "mem-entries";
      for (const e of ns.entries) {
        const li = document.createElement("li");
        li.className = "mem-entry";
        const head = document.createElement("div");
        head.className = "mem-entry-head";
        head.innerHTML = `<code class="mem-key">${escapeHtml(e.key)}</code> <span class="muted mem-time">${escapeHtml(fmtTime(e.updated_at))}</span>`;
        const forgetBtn = document.createElement("button");
        forgetBtn.type = "button";
        forgetBtn.className = "ghost mem-forget";
        forgetBtn.textContent = "遗忘";
        forgetBtn.addEventListener("click", () => forget(ns.namespace, e.key));
        head.appendChild(forgetBtn);
        li.appendChild(head);
        const pre = document.createElement("pre");
        pre.className = "mem-value";
        pre.textContent = prettyJson(e.value);
        li.appendChild(pre);
        ul.appendChild(li);
      }
      details.appendChild(ul);
      $tree.appendChild(details);
    }
  }

  async function forget(namespace, key) {
    if (!confirm(`遗忘 ${namespace}/${key} ?`)) return;
    try {
      const r = await fetch("/api/memory", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ namespace, key }),
      });
      if (!r.ok) throw new Error("HTTP " + r.status);
      load();
    } catch (e) {
      alert("遗忘失败: " + e.message);
    }
  }

  $refresh.addEventListener("click", load);
  load();
})();
