package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

type toolsPageData struct {
	Title   string
	Active  string
	Schemas []toolSchemaView
	Profile string
	Model   string
}

type toolSchemaView struct {
	Name        string
	Description string
	Params      string // pretty-printed JSON schema
	Example     string // 该工具的一个最小可用 JSON 示例
}

// toolExamples 给每个内置工具一个"开箱即用"的最小调用示例,
// 让 /tools 面板的试调用框 placeholder 直接可复制粘贴.
var toolExamples = map[string]string{
	"product_lookup": `{"id":"sku_001"}`,
	"price_format":   `{"price_twd":690,"shipping":"現貨","badges":["限時免運"]}`,
	"platform_lint":  `{"platform":"shopee_tw","kind":"title","text":"日本製今治毛巾 蓬鬆吸水 #毛巾 #日本"}`,
	"slang_check":    `{"text":"限時下殺 現貨免運, CP值高的必買神器"}`,
	"memory_get":     `{"namespace":"seller:A001","key":"tone"}`,
	"memory_put":     `{"namespace":"seller:A001","key":"tone","value":"{\"style\":\"girlfriend\",\"emoji\":\"high\",\"price_position\":\"end\"}"}`,
}

func (s *Server) handleToolsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	views := make([]toolSchemaView, 0)
	if s.tools != nil {
		for _, sc := range s.tools.Schemas() {
			pretty := prettyJSON(sc.Function.Parameters)
			ex, ok := toolExamples[sc.Function.Name]
			if !ok {
				ex = "{}"
			}
			views = append(views, toolSchemaView{
				Name:        sc.Function.Name,
				Description: sc.Function.Description,
				Params:      pretty,
				Example:     ex,
			})
		}
	}
	data := toolsPageData{
		Title:   "Tools · agent-lab",
		Active:  "/tools",
		Schemas: views,
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "tools.html", data)
}

// handleToolsRecent 返回最近的工具调用历史.
func (s *Server) handleToolsRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.toolHist == nil {
		writeJSON(w, http.StatusOK, map[string]any{"invocations": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invocations": s.toolHist.Snapshot()})
}

// handleToolsInvoke 直接调用一个已注册工具, 用于在 UI 上做交互式调试.
//
// 这是 M2 之外的便利能力, 允许用户从 /tools 面板手动测试某个 schema.
func (s *Server) handleToolsInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json: " + err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
		return
	}
	if s.tools == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "tool registry not configured"})
		return
	}
	tool, ok := s.tools.Get(req.Name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown tool: " + req.Name})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	started := time.Now()
	result, err := tool.Invoke(ctx, req.Args)
	dur := time.Since(started).Milliseconds()

	inv := ToolInvocation{
		ID:         fmt.Sprintf("inv_%d", started.UnixNano()),
		Name:       req.Name,
		Args:       string(req.Args),
		StartedAt:  started,
		DurationMS: dur,
	}
	if err != nil {
		inv.Err = err.Error()
	} else {
		inv.Result = result
	}
	if s.toolHist != nil {
		s.toolHist.Add(inv)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":          inv.ID,
			"ok":          false,
			"error":       err.Error(),
			"duration_ms": dur,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          inv.ID,
		"ok":          true,
		"result":      result,
		"duration_ms": dur,
	})
}

// prettyJSON 把任意 JSON 文本格式化成 2-空格缩进, 失败时原样返回.
func prettyJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}

// 仅为防止 unused import 的占位; 实际依赖在 server.go 注册时引入.
var (
	_ = llm.RoleSystem
	_ = tools.ErrUnknownTool
)
