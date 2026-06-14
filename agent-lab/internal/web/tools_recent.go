package web

import (
	"sync"
	"time"
)

// ToolInvocation 是 Web /api/tools/invoke 一次调用的留痕.
type ToolInvocation struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Args       string    `json:"args"`
	Result     string    `json:"result,omitempty"`
	Err        string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int64     `json:"duration_ms"`
}

// ToolRecentBuffer 是工具调用历史的环形缓冲区, 默认保留最近 50 条.
type ToolRecentBuffer struct {
	mu  sync.RWMutex
	max int
	buf []ToolInvocation
}

// NewToolRecentBuffer 构造一个保留最近 max 条记录的缓冲.
func NewToolRecentBuffer(max int) *ToolRecentBuffer {
	if max <= 0 {
		max = 50
	}
	return &ToolRecentBuffer{max: max}
}

// Add 追加一条记录, 满则丢最旧一条.
func (b *ToolRecentBuffer) Add(inv ToolInvocation) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, inv)
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
}

// Snapshot 返回按时间倒序的副本.
func (b *ToolRecentBuffer) Snapshot() []ToolInvocation {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]ToolInvocation, len(b.buf))
	for i := range b.buf {
		out[len(b.buf)-1-i] = b.buf[i]
	}
	return out
}

// Len 返回当前条数.
func (b *ToolRecentBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.buf)
}
