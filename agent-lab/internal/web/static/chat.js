// agent-lab M1 Chat UI: 多会话 + 角色卡 + 摘要提示
(() => {
  const $messages = document.getElementById("messages");
  const $input    = document.getElementById("input");
  const $send     = document.getElementById("send");
  const $stop     = document.getElementById("stop");
  const $reset    = document.getElementById("reset");
  const $export   = document.getElementById("export");
  const $importBtn= document.getElementById("import-btn");
  const $importFile = document.getElementById("import-file");
  const $system   = document.getElementById("system");
  const $saveSystem = document.getElementById("save-system");
  const $systemStatus = document.getElementById("system-status");
  const $status   = document.getElementById("status");
  const $newConv  = document.getElementById("new-conv");
  const $convList = document.getElementById("conv-list");
  const $mode     = document.getElementById("mode");
  const $temp     = document.getElementById("temperature");
  const $maxTk    = document.getElementById("max-tokens");

  // 当前会话 ID, 初次为 "" (将在首次发送时由服务端生成).
  let currentConv = "";
  let currentTitle = "";
  let abortCtrl = null;
  let busy = false;
  // 请求序号: 每次发起 loadConversations 时递增, 旧响应的序号如果低于当前值则丢弃.
  let loadSeq = 0;

  // ----------------- 会话列表 -----------------
  async function loadConversations() {
    const seq = ++loadSeq; // 获取本次请求的序号
    try {
      const r = await fetch("/api/conversations");
      if (r.status !== 200) throw new Error("status " + r.status);
      const data = await r.json();
      // 如果期间有更新的请求发出, 丢弃本响应, 防止陈旧数据覆盖新列表.
      if (seq !== loadSeq) return;
      // 清空后再 render, 防止旧的 <li> 与 fetch 结果叠加导致"幽灵残留".
      $convList.innerHTML = "";
      renderConvList(data.conversations || []);
    } catch (e) {
      console.error("[chat] loadConversations failed:", e);
    }
  }

  function renderConvList(convs) {
    $convList.innerHTML = "";
    if (convs.length === 0) {
      const li = document.createElement("li");
      li.className = "conv-empty";
      li.textContent = "还没有会话";
      $convList.appendChild(li);
      return;
    }
    for (const c of convs) {
      const li = document.createElement("li");
      li.className = "conv-item" + (c.id === currentConv ? " active" : "");
      li.dataset.id = c.id;
      const title = document.createElement("span");
      title.className = "conv-title";
      title.textContent = c.title || "(无标题)";
      title.title = "点击切换会话";
      title.addEventListener("click", () => switchConversation(c.id));
      const actions = document.createElement("span");
      actions.className = "conv-actions";
      const renameBtn = document.createElement("button");
      renameBtn.type = "button";
      renameBtn.textContent = "✎";
      renameBtn.title = "重命名";
      renameBtn.addEventListener("click", (e) => { e.stopPropagation(); renameConversation(c.id); });
      const delBtn = document.createElement("button");
      delBtn.type = "button";
      delBtn.textContent = "✕";
      delBtn.title = "删除";
      delBtn.addEventListener("click", (e) => { e.stopPropagation(); deleteConversation(c.id); });
      actions.appendChild(renameBtn);
      actions.appendChild(delBtn);
      li.appendChild(title);
      li.appendChild(actions);
      $convList.appendChild(li);
    }
  }

  async function switchConversation(id) {
    if (busy) return;
    const resp = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "switch", conversation_id: id }),
    });
    if (!resp.ok) return;
    const data = await resp.json();
    currentConv = data.id;
    currentTitle = data.title || "";
    if (data.system) $system.value = data.system;
    $messages.innerHTML = "";
    if (data.messages) {
      for (const m of data.messages) {
        addBubble(m.role, m.content);
      }
    }
    loadConversations();
  }

  async function renameConversation(id) {
    const newName = prompt("新标题:", currentTitle || "");
    if (!newName) return;
    const resp = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "rename", conversation_id: id, title: newName }),
    });
    if (resp.ok) {
      if (id === currentConv) currentTitle = newName;
      loadConversations();
    }
  }

  async function deleteConversation(id) {
    if (!confirm("确认删除这个会话?")) return;
    // 乐观删除: 先从 DOM 上拿掉这一条, 让 UI 立即响应.
    const li = $convList.querySelector(`li.conv-item[data-id="${cssEscape(id)}"]`);
    if (li) li.remove();
    if (id === currentConv) {
      currentConv = "";
      currentTitle = "";
      $messages.innerHTML = "";
    }
    try {
      await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "delete", conversation_id: id }),
      });
    } catch (_) { /* 网络错误也照样最终对账, 否则永远删不掉 */ }
    // 与服务端对账, 防止幽灵条目残留.
    loadConversations();
  }

  // cssEscape 兼容老浏览器, 处理 conv id 里可能出现的特殊字符.
  function cssEscape(s) {
    if (window.CSS && CSS.escape) return CSS.escape(s);
    return String(s).replace(/[^a-zA-Z0-9_\-]/g, (ch) => "\\" + ch);
  }

  $newConv.addEventListener("click", async () => {
    if (busy) return;
    const resp = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "new" }),
    });
    if (!resp.ok) return;
    const data = await resp.json();
    currentConv = data.id;
    currentTitle = data.title || "";
    $messages.innerHTML = "";
    loadConversations();
    $input.focus();
  });

  // ----------------- 角色卡 -----------------
  $saveSystem.addEventListener("click", async () => {
    if (!currentConv) return;
    const resp = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "set_system", conversation_id: currentConv, system: $system.value }),
    });
    if (resp.ok) {
      $systemStatus.textContent = "已保存";
      setTimeout(() => ($systemStatus.textContent = ""), 1500);
    }
  });

  // ----------------- 重置 / 导出 / 导入 -----------------
  $reset.addEventListener("click", async () => {
    if (!currentConv) return;
    if (!confirm("清空当前会话?")) return;
    const resp = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "reset", conversation_id: currentConv }),
    });
    if (resp.ok) $messages.innerHTML = "";
  });

  $export.addEventListener("click", async () => {
    if (!currentConv) return;
    const resp = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "export", conversation_id: currentConv }),
    });
    if (!resp.ok) return;
    const data = await resp.json();
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = `conversation-${currentConv}.json`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });

  $importBtn.addEventListener("click", () => $importFile.click());
  $importFile.addEventListener("change", async (e) => {
    const f = e.target.files && e.target.files[0];
    if (!f) return;
    try {
      const text = await f.text();
      const parsed = JSON.parse(text);
      // 先确保有一个会话; 没有就新建.
      if (!currentConv) {
        const r = await fetch("/api/chat", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ action: "new" }),
        });
        if (!r.ok) throw new Error("create failed");
        const d = await r.json();
        currentConv = d.id;
      }
      const resp = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "load",
          conversation_id: currentConv,
          system: parsed.system || "",
          messages: parsed.messages || [],
        }),
      });
      if (!resp.ok) throw new Error("import failed");
      if (parsed.system) $system.value = parsed.system;
      $messages.innerHTML = "";
      if (parsed.messages) {
        for (const m of parsed.messages) addBubble(m.role, m.content);
      }
      loadConversations();
    } catch (err) {
      alert("导入失败: " + err.message);
    } finally {
      $importFile.value = "";
    }
  });

  // ----------------- 消息气泡 -----------------
  function addBubble(role, content, opts = {}) {
    const el = document.createElement("div");
    el.className = "msg " + role + (opts.streaming ? " streaming" : "");
    el.textContent = content;
    $messages.appendChild(el);
    $messages.scrollTop = $messages.scrollHeight;
    return el;
  }

  function addStepCard(step) {
    const card = document.createElement("div");
    card.className = "msg step-card";
    const header = document.createElement("div");
    header.className = "step-header";
    header.innerHTML = `<span class="step-index">step ${step.step_index}</span> <span class="step-kind">${escapeHtml(step.kind || "")}</span> <span class="step-time">${step.elapsed_ms ? step.elapsed_ms + " ms" : ""}</span>`;
    card.appendChild(header);

    const body = document.createElement("div");
    body.className = "step-body";
    if (step.thought) {
      const b = document.createElement("div");
      b.className = "step-row";
      b.innerHTML = `<span class="step-label">thought</span><pre>${escapeHtml(step.thought)}</pre>`;
      body.appendChild(b);
    }
    if (step.action_name) {
      const b = document.createElement("div");
      b.className = "step-row";
      b.innerHTML = `<span class="step-label">action</span><pre>${escapeHtml(step.action_name)}${step.action_args ? " " + step.action_args : ""}</pre>`;
      body.appendChild(b);
    }
    if (step.observation) {
      const b = document.createElement("div");
      b.className = "step-row";
      b.innerHTML = `<span class="step-label">observation</span><pre>${escapeHtml(step.observation)}</pre>`;
      body.appendChild(b);
    }
    if (step.error) {
      const b = document.createElement("div");
      b.className = "step-row";
      b.innerHTML = `<span class="step-label">error</span><pre>${escapeHtml(step.error)}</pre>`;
      body.appendChild(b);
    }
    card.appendChild(body);
    $messages.appendChild(card);
    $messages.scrollTop = $messages.scrollHeight;
    return card;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  function addNote(text) {
    const el = document.createElement("div");
    el.className = "msg note";
    el.textContent = text;
    $messages.appendChild(el);
    $messages.scrollTop = $messages.scrollHeight;
    return el;
  }

  function setBusy(v) {
    busy = v;
    $send.disabled = v;
    $input.disabled = v;
    $stop.disabled = !v;
    $status.textContent = v ? "生成中..." : "";
  }

  // ----------------- 发送 -----------------
  async function send() {
    const msg = $input.value.trim();
    if (!msg || busy) return;

    addBubble("user", msg);
    $input.value = "";

    const mode = ($mode && $mode.value) || "native";
    const temperature = ($temp && $temp.value) ? parseFloat($temp.value) : 0.4;
    const maxTokens = ($maxTk && $maxTk.value) ? parseInt($maxTk.value, 10) : 512;

    const assistantEl = addBubble("assistant", "", { streaming: true });
    let acc = "";
    let finishReason = "";
    let usage = null;
    let hasAnyContent = false;

    abortCtrl = new AbortController();
    setBusy(true);

    try {
      const resp = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "send",
          conversation_id: currentConv,
          system: $system.value,
          message: msg,
          mode: mode,
          temperature: isNaN(temperature) ? 0.4 : temperature,
          max_tokens: isNaN(maxTokens) ? 512 : maxTokens,
        }),
        signal: abortCtrl.signal,
      });
      if (!resp.ok || !resp.body) {
        const text = await resp.text().catch(() => "");
        throw new Error("HTTP " + resp.status + " " + text);
      }

      const reader  = resp.body.getReader();
      const decoder = new TextDecoder("utf-8");
      let buf = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        let idx;
        while ((idx = buf.indexOf("\n\n")) >= 0) {
          const frame = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          const evt = parseFrame(frame);
          if (!evt) continue;
          if (evt.event === "start" && evt.data) {
            if (evt.data.conversation_id && !currentConv) {
              currentConv = evt.data.conversation_id;
            }
            if (evt.data.mode === "react") {
              addNote(`agent 模式: react`);
            }
            continue;
          }
          if (evt.event === "summary" && evt.data) {
            const d = evt.data;
            const text = `摘要: 压缩 ${d.before_turns} → ${d.after_turns} 轮` + (d.summary ? `: ${d.summary}` : "");
            addNote(text);
            continue;
          }
          if (evt.event === "step" && evt.data) {
            addStepCard(evt.data);
            continue;
          }
          if (evt.event === "final" && evt.data && evt.data.content) {
            acc = evt.data.content;
            assistantEl.textContent = acc;
            hasAnyContent = true;
            $messages.scrollTop = $messages.scrollHeight;
            continue;
          }
          if (evt.event === "delta" && evt.data && evt.data.content) {
            acc += evt.data.content;
            assistantEl.textContent = acc;
            hasAnyContent = true;
            $messages.scrollTop = $messages.scrollHeight;
          } else if (evt.event === "finish" && evt.data) {
            finishReason = evt.data.reason || finishReason;
          } else if (evt.event === "usage" && evt.data) {
            usage = evt.data;
          } else if (evt.event === "error" && evt.data) {
            assistantEl.classList.add("error");
            assistantEl.textContent = "错误: " + (evt.data.message || "unknown");
            hasAnyContent = true;
          } else if (evt.event === "canceled") {
            addNote("已停止");
          } else if (evt.event === "done" && evt.data && evt.data.conversation_id) {
            if (evt.data.title) currentTitle = evt.data.title;
          }
        }
      }
      if (!hasAnyContent && !acc) {
        assistantEl.textContent = "(无回复)";
      }
    } catch (err) {
      if (err.name === "AbortError") {
        addNote("已停止");
      } else {
        assistantEl.classList.add("error");
        assistantEl.textContent = "错误: " + err.message;
      }
    } finally {
      assistantEl.classList.remove("streaming");
      setBusy(false);
      abortCtrl = null;
      const parts = [];
      if (mode) parts.push("mode=" + mode);
      if (finishReason) parts.push("finish=" + finishReason);
      if (usage) parts.push(`tokens=${usage.prompt_tokens}/${usage.completion_tokens}/${usage.total_tokens}`);
      $status.textContent = parts.join(" · ");
      loadConversations();
    }
  }

  function parseFrame(raw) {
    let event = "message";
    const dataLines = [];
    for (const line of raw.split("\n")) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
    }
    if (dataLines.length === 0) return null;
    let data = null;
    try { data = JSON.parse(dataLines.join("")); } catch (_) { /* keep null */ }
    return { event, data };
  }

  $send.addEventListener("click", send);
  $stop.addEventListener("click", () => { if (abortCtrl) abortCtrl.abort(); });
  $input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  });

  // 初始加载会话列表
  loadConversations();
})();
