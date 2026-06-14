package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
)

type chatPageData struct {
	Title    string
	Active   string
	Profile  string
	Model    string
	BaseURL  string
	System   string
	Budget   int
	Reserve  int
}

func (s *Server) handleChatPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := chatPageData{
		Title:   "Chat · agent-lab",
		Active:  "/chat",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
		BaseURL: s.cfg.BaseURL,
		System:  s.defaultSystem,
		Budget:  s.budget,
		Reserve: s.reserve,
	}
	s.renderPage(w, "chat.html", data)
}

// chatAPIRequest 是浏览器发来的一条聊天请求.
// 支持: 新建消息 / 切换会话 / 编辑角色卡 / 重置 / 导出/导入历史.
type chatAPIRequest struct {
	Action       string            `json:"action"`        // "send" | "new" | "rename" | "delete" | "switch" | "set_system" | "reset" | "export" | "load"
	ConvID       string            `json:"conversation_id"`
	Title        string            `json:"title"`
	System       string            `json:"system"`
	Message      string            `json:"message"`
	Messages     []chatHistoryEntry `json:"messages"`
	Temp         float32           `json:"temperature"`
	Max          int               `json:"max_tokens"`
}

type chatHistoryEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// handleChatAPI 处理聊天相关的所有动作. 当 Action="send" 时用 SSE 流式回复.
func (s *Server) handleChatAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req chatAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	switch req.Action {
	case "", "send":
		s.handleChatSend(w, r, req)
	case "new":
		s.handleChatNew(w, req)
	case "switch":
		s.handleChatSwitch(w, req)
	case "rename":
		s.handleChatRename(w, req)
	case "delete":
		s.handleChatDelete(w, req)
	case "set_system":
		s.handleChatSetSystem(w, req)
	case "reset":
		s.handleChatReset(w, req)
	case "export":
		s.handleChatExport(w, req)
	case "load":
		s.handleChatLoad(w, req)
	default:
		http.Error(w, "unknown action: "+req.Action, http.StatusBadRequest)
	}
}

// handleConversationsAPI 返回会话列表 (GET).
func (s *Server) handleConversationsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list := s.convs.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"conversations": list})
}

// handleTutorial 渲染教程页 (直接从 static/tutorial.html 读取).
func (s *Server) handleTutorial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// tutorial.html 是完整 HTML, 不通过 layout 模板.
	f, err := staticFS.Open("static/tutorial.html")
	if err != nil {
		http.Error(w, "tutorial not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 直接拷贝到 ResponseWriter.
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Server) handleChatNew(w http.ResponseWriter, req chatAPIRequest) {
	c := s.convs.New("", req.Title, s.defaultSystem, s.budget, s.reserve)
	writeJSON(w, http.StatusOK, map[string]any{"id": c.ID, "title": c.Title})
}

func (s *Server) handleChatSwitch(w http.ResponseWriter, req chatAPIRequest) {
	c := s.convs.Get(req.ConvID)
	if c == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	msgs := c.Memory.Messages()
	entries := make([]chatHistoryEntry, 0, len(msgs))
	for _, m := range msgs {
		entries = append(entries, chatHistoryEntry{Role: string(m.Role), Content: m.Content})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       c.ID,
		"title":    c.Title,
		"system":   c.Memory.System(),
		"messages": entries,
	})
}

func (s *Server) handleChatRename(w http.ResponseWriter, req chatAPIRequest) {
	if !s.convs.Rename(req.ConvID, req.Title) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatDelete(w http.ResponseWriter, req chatAPIRequest) {
	// 幂等删除: 即使 id 不存在 (例如 server 重启后的"幽灵会话"), 也返回 ok,
	// 让前端可以放心地刷新列表把它清掉.
	existed := s.convs.Delete(req.ConvID)
	fmt.Printf("[chat] delete id=%q existed=%v remaining=%d\n", req.ConvID, existed, len(s.convs.List()))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "existed": existed})
}

func (s *Server) handleChatSetSystem(w http.ResponseWriter, req chatAPIRequest) {
	c := s.convs.Get(req.ConvID)
	if c == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	c.Memory.SetSystem(req.System)
	s.convs.Touch(req.ConvID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatReset(w http.ResponseWriter, req chatAPIRequest) {
	c := s.convs.Get(req.ConvID)
	if c == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	c.Memory.Reset()
	s.convs.Touch(req.ConvID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatExport(w http.ResponseWriter, req chatAPIRequest) {
	c := s.convs.Get(req.ConvID)
	if c == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	msgs := c.Memory.Messages()
	entries := make([]chatHistoryEntry, 0, len(msgs))
	for _, m := range msgs {
		entries = append(entries, chatHistoryEntry{Role: string(m.Role), Content: m.Content})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"system":   c.Memory.System(),
		"messages": entries,
	})
}

func (s *Server) handleChatLoad(w http.ResponseWriter, req chatAPIRequest) {
	c := s.convs.Get(req.ConvID)
	if c == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	// 先清空, 再恢复.
	c.Memory.Reset()
	if req.System != "" {
		c.Memory.SetSystem(req.System)
	}
	for _, m := range req.Messages {
		role := llm.Role(strings.ToLower(m.Role))
		if role != llm.RoleUser && role != llm.RoleAssistant {
			continue
		}
		c.Memory.Append(role, m.Content)
	}
	s.convs.Touch(req.ConvID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": c.Memory.Len()})
}

// handleChatSend 发送一条消息给 LLM, 用 SSE 流式回复.
func (s *Server) handleChatSend(w http.ResponseWriter, r *http.Request, req chatAPIRequest) {
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "message is required"})
		return
	}
	// 拿到或创建会话.
	conv := s.convs.Get(req.ConvID)
	if conv == nil {
		conv = s.convs.New(req.ConvID, "", s.defaultSystem, s.budget, s.reserve)
	}

	conv.Memory.Append(llm.RoleUser, req.Message)

	// 压缩检查
	info, cerr := conv.Memory.EnsureBudget(r.Context(), s.llm, s.cfg.ModelChat, req.Max)
	if cerr != nil {
		fmt.Printf("[chat] memory: %v\n", cerr)
	}
	s.convs.Touch(conv.ID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}

	// SSE 头.
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	// 如果发生了压缩, 先推一条 summary 事件.
	if info.DidCompress {
		writeSSE(w, flusher, "summary", map[string]any{
			"before_turns": info.BeforeTurns,
			"after_turns":  info.AfterTurns,
			"before_chars": info.BeforeChars,
			"summary":      info.Summary,
		})
	}

	// 组装 messages.
	msgs := conv.Memory.Snapshot()

	temp := req.Temp
	if temp <= 0 {
		temp = 0.4
	}
	mt := req.Max
	if mt <= 0 {
		mt = 512
	}
	chatReq := llm.ChatRequest{
		Model:       s.cfg.ModelChat,
		Messages:    msgs,
		Temperature: &temp,
		MaxTokens:   &mt,
	}

	stream, err := s.llm.ChatStream(r.Context(), chatReq)
	if err != nil {
		writeSSE(w, flusher, "error", map[string]any{"message": err.Error()})
		return
	}
	writeSSE(w, flusher, "start", map[string]any{"model": s.cfg.ModelChat, "conversation_id": conv.ID})

	var sb strings.Builder
	for chunk := range stream {
		if chunk.Err != nil {
			if errors.Is(chunk.Err, r.Context().Err()) {
				writeSSE(w, flusher, "canceled", map[string]any{})
			} else {
				writeSSE(w, flusher, "error", map[string]any{"message": chunk.Err.Error()})
			}
			return
		}
		if chunk.DeltaContent != "" {
			sb.WriteString(chunk.DeltaContent)
			writeSSE(w, flusher, "delta", map[string]any{"content": chunk.DeltaContent})
		}
		if chunk.FinishReason != "" {
			writeSSE(w, flusher, "finish", map[string]any{"reason": chunk.FinishReason})
		}
		if chunk.Usage != nil {
			writeSSE(w, flusher, "usage", map[string]any{
				"prompt_tokens":     chunk.Usage.PromptTokens,
				"completion_tokens": chunk.Usage.CompletionTokens,
				"total_tokens":      chunk.Usage.TotalTokens,
			})
		}
	}
	// 把回复写入历史.
	conv.Memory.Append(llm.RoleAssistant, sb.String())
	// 更新标题 (如果是首条消息), 仅在能从消息里抽到非空首句时更新, 避免把 title 覆盖成空字符串.
	if len(conv.Memory.Messages()) <= 2 {
		if first := firstLine(req.Message); first != "" {
			conv.Title = truncRunes(first, 20)
		}
	}
	s.convs.Touch(conv.ID)

	writeSSE(w, flusher, "done", map[string]any{"conversation_id": conv.ID, "title": conv.Title})
}

func firstLine(s string) string {
	if idx := strings.IndexAny(s, "\n\r"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// truncRunes 按 rune 截断, 避免在中文字节中间切断造成乱码.
func truncRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// writeSSE 把 (event, data) 写成 SSE 帧并 flush.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return
	}
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
