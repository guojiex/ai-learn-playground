package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Router 根据任务标签和上下文长度自动选择模型, 并在失败时降级 (M10).
//
// 路由器与 LLMClient 解耦: 调用方只传 task 标签, 路由内部决定打到哪.
type Router struct {
	registry *Registry
	policy   *Policy
	clients  map[string]Client // model name → client (同 base_url 共享 client)
	mu       sync.Mutex
	history  []RouteRecord
}

// NewRouter 构造一个路由器.
func NewRouter(reg *Registry, policy *Policy, defaultClient Client) *Router {
	clients := make(map[string]Client)
	for _, m := range reg.Models {
		clients[m.Name] = defaultClient
	}
	return &Router{
		registry: reg,
		policy:   policy,
		clients:  clients,
	}
}

// Route 返回路由决策结果 (不执行调用).
func (r *Router) Route(task string, ctxTokens int) (*RouteResult, error) {
	return r.policy.Evaluate(task, ctxTokens, r.registry)
}

// ChatForTask 按任务标签路由, 执行调用, 失败时走 fallback 链.
func (r *Router) ChatForTask(ctx context.Context, task string, req ChatRequest) (ChatResponse, *RouteRecord, error) {
	start := time.Now()
	ctxTokens := estimateCtxTokens(req)

	result, err := r.Route(task, ctxTokens)
	if err != nil {
		rec := RouteRecord{
			Timestamp: start,
			Task:      task,
			CtxTokens: ctxTokens,
			Success:   false,
			Error:     err.Error(),
		}
		r.record(rec)
		return ChatResponse{}, &rec, err
	}

	// 尝试 primary.
	primaryName := result.Primary.Name
	req.Model = primaryName
	client := r.clients[primaryName]
	resp, err := client.Chat(ctx, req)
	if err == nil {
		rec := RouteRecord{
			Timestamp: start,
			Task:      task,
			CtxTokens: ctxTokens,
			Chosen:    primaryName,
			Reason:    result.Reason,
			LatencyMs: time.Since(start).Milliseconds(),
			Success:   true,
		}
		r.record(rec)
		resp.Model = primaryName
		return resp, &rec, nil
	}

	// 走 fallback 链.
	var tried []string
	tried = append(tried, primaryName)
	for _, fb := range result.Fallbacks {
		req.Model = fb.Name
		fbClient := r.clients[fb.Name]
		resp, fbErr := fbClient.Chat(ctx, req)
		if fbErr == nil {
			rec := RouteRecord{
				Timestamp: start,
				Task:      task,
				CtxTokens: ctxTokens,
				Chosen:    fb.Name,
				Fallbacks: tried,
				Reason:    result.Reason + " (fallback from " + primaryName + ")",
				LatencyMs: time.Since(start).Milliseconds(),
				Success:   true,
			}
			r.record(rec)
			resp.Model = fb.Name
			return resp, &rec, nil
		}
		tried = append(tried, fb.Name)
	}

	// 全部失败.
	rec := RouteRecord{
		Timestamp: start,
		Task:      task,
		CtxTokens: ctxTokens,
		Chosen:    primaryName,
		Fallbacks: tried,
		Reason:    result.Reason,
		LatencyMs: time.Since(start).Milliseconds(),
		Success:   false,
		Error:     fmt.Sprintf("all models failed: %v", tried),
	}
	r.record(rec)
	return ChatResponse{}, &rec, fmt.Errorf("all models failed for task=%s: %v", task, tried)
}

// Registry 返回模型注册表.
func (r *Router) Registry() *Registry {
	return r.registry
}

// Policy 返回路由策略.
func (r *Router) Policy() *Policy {
	return r.policy
}

// RecentRoutes 返回最近 N 条路由记录.
func (r *Router) RecentRoutes(limit int) []RouteRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}
	out := make([]RouteRecord, limit)
	copy(out, r.history[len(r.history)-limit:])
	// 反转: 最新的在前.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (r *Router) record(rec RouteRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append(r.history, rec)
	if len(r.history) > 200 {
		r.history = r.history[len(r.history)-200:]
	}
}

// estimateCtxTokens 粗略估算请求的上下文 token 数.
func estimateCtxTokens(req ChatRequest) int {
	total := 0
	for _, msg := range req.Messages {
		total += len(msg.Content) / 3 // 粗略: 3 字符 ≈ 1 token (混合中英)
	}
	return total
}
