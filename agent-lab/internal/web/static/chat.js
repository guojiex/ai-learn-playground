// 单文件 SSE Chat 客户端. 故意手写, 不引任何运行时.
(() => {
  const $messages = document.getElementById("messages");
  const $input    = document.getElementById("input");
  const $send     = document.getElementById("send");
  const $stop     = document.getElementById("stop");
  const $reset    = document.getElementById("reset");
  const $system   = document.getElementById("system");
  const $status   = document.getElementById("status");

  /** @type {{role:string, content:string}[]} */
  const history = [];
  let abortCtrl = null;

  function addBubble(role, content, opts = {}) {
    const el = document.createElement("div");
    el.className = "msg " + role + (opts.streaming ? " streaming" : "");
    el.textContent = content;
    $messages.appendChild(el);
    $messages.scrollTop = $messages.scrollHeight;
    return el;
  }

  function addNote(text) {
    addBubble("note", text);
  }

  function setBusy(busy) {
    $send.disabled  = busy;
    $input.disabled = busy;
    $stop.disabled  = !busy;
    $status.textContent = busy ? "生成中…" : "";
  }

  async function send() {
    const msg = $input.value.trim();
    if (!msg) return;

    addBubble("user", msg);
    history.push({ role: "user", content: msg });
    $input.value = "";

    const assistantEl = addBubble("assistant", "", { streaming: true });
    let acc = "";
    let finishReason = "";
    let usage = null;

    abortCtrl = new AbortController();
    setBusy(true);

    try {
      const resp = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          system: $system.value,
          message: msg,
          history: history.slice(0, -1), // 当前 user 还未完成一轮
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
        // SSE 帧之间用空行分隔.
        while ((idx = buf.indexOf("\n\n")) >= 0) {
          const frame = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          const evt = parseFrame(frame);
          if (!evt) continue;
          handleEvent(evt);
        }
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
      if (acc) history.push({ role: "assistant", content: acc });
      if (finishReason || usage) {
        const parts = [];
        if (finishReason) parts.push("finish=" + finishReason);
        if (usage) parts.push(`tokens=${usage.prompt_tokens}/${usage.completion_tokens}/${usage.total_tokens}`);
        $status.textContent = parts.join(" · ");
      }
    }

    function handleEvent(evt) {
      if (evt.event === "delta" && evt.data && evt.data.content) {
        acc += evt.data.content;
        assistantEl.textContent = acc;
        $messages.scrollTop = $messages.scrollHeight;
      } else if (evt.event === "finish" && evt.data) {
        finishReason = evt.data.reason || finishReason;
      } else if (evt.event === "usage" && evt.data) {
        usage = evt.data;
      } else if (evt.event === "error" && evt.data) {
        assistantEl.classList.add("error");
        assistantEl.textContent = "错误: " + (evt.data.message || "unknown");
      } else if (evt.event === "canceled") {
        addNote("已停止");
      }
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
    try { data = JSON.parse(dataLines.join("\n")); } catch (_) { /* keep null */ }
    return { event, data };
  }

  $send.addEventListener("click", send);
  $stop.addEventListener("click", () => { if (abortCtrl) abortCtrl.abort(); });
  $reset.addEventListener("click", () => {
    history.length = 0;
    $messages.innerHTML = "";
    $status.textContent = "";
  });
  $input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  });
})();
