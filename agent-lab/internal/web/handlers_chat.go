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
	SystemHi string
}

func (s *Server) handleChatPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := chatPageData{
		Title:    "Chat · agent-lab",
		Active:   "/chat",
		Profile:  s.cfg.Profile,
		Model:    s.cfg.ModelChat,
		BaseURL:  s.cfg.BaseURL,
		SystemHi: defaultSystemPrompt,
	}
	s.renderPage(w, "chat.html", data)
}

const defaultSystemPrompt = "你是一个稳重务实的台湾电商文案助理. 请先收集 SKU 信息再开口写文案."

type chatAPIRequest struct {
	System  string  `json:"system"`
	Message string  `json:"message"`
	Model   string  `json:"model"`
	History []chatHistoryEntry `json:"history"`
	Temp    float32 `json:"temperature"`
	Max     int     `json:"max_tokens"`
}

type chatHistoryEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// handleChatAPI 接收 JSON 请求, 用 SSE 把流式 token 转发给浏览器.
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
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
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
	h.Set("X-Accel-Buffering", "no") // 防止前置代理缓冲

	// 组装 messages.
	system := strings.TrimSpace(req.System)
	if system == "" {
		system = defaultSystemPrompt
	}
	msgs := []llm.Message{{Role: llm.RoleSystem, Content: system}}
	for _, e := range req.History {
		role := llm.Role(e.Role)
		if role != llm.RoleUser && role != llm.RoleAssistant {
			continue
		}
		msgs = append(msgs, llm.Message{Role: role, Content: e.Content})
	}
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: req.Message})

	model := req.Model
	if model == "" {
		model = s.cfg.ModelChat
	}
	temp := req.Temp
	if temp <= 0 {
		temp = 0.4
	}
	mt := req.Max
	if mt <= 0 {
		mt = 512
	}
	chatReq := llm.ChatRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: &temp,
		MaxTokens:   &mt,
	}

	// 启动流; ctx 来自 r.Context, 浏览器关闭连接即取消.
	stream, err := s.llm.ChatStream(r.Context(), chatReq)
	if err != nil {
		writeSSE(w, flusher, "error", map[string]any{"message": err.Error()})
		return
	}
	writeSSE(w, flusher, "start", map[string]any{"model": model})

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
	writeSSE(w, flusher, "done", map[string]any{})
}

// writeSSE 把 (event, data) 写成 SSE 帧并 flush.
// 失败 (例如客户端断开) 时静默忽略, 由调用方下一轮迭代终止.
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
