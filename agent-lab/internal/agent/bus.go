package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"ai-learn-playground/agent-lab/internal/store"
)

// RoleName 是多 agent 系统中的角色标识.
type RoleName string

const (
	RoleResearcher  RoleName = "researcher"
	RoleWriter      RoleName = "writer"
	RoleCritic      RoleName = "critic"
	RoleCompliance  RoleName = "compliance"
	RoleCoordinator RoleName = "coordinator"
)

// BusMessage 是 agent 间传递的一条消息.
type BusMessage struct {
	ID        int64     `json:"id"`
	RunID     string    `json:"run_id"`
	Round     int       `json:"round"`
	Role      string    `json:"role"` // "system" | "user" | "assistant"
	FromAgent RoleName  `json:"from_agent"`
	ToAgent   RoleName  `json:"to_agent"`
	Content   string    `json:"content"`
	Metadata  string    `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// MessageBus 是多 agent 系统的内存消息总线 (M7).
//
// 职责:
//   - 记录所有 agent 间消息 (按 round 分组).
//   - 可选持久化到 SQLite (agent_messages 表), 供回放.
//   - 线程安全: 多个 agent 可并发写消息.
type MessageBus struct {
	mu       sync.Mutex
	messages []BusMessage
	store    *store.Store
	runID    string
	nextID   int64
}

// NewMessageBus 构造一个绑定到 runID 的消息总线.
// store 为 nil 时仅内存, 不持久化.
func NewMessageBus(runID string, st *store.Store) *MessageBus {
	return &MessageBus{
		store: st,
		runID: runID,
	}
}

// Post 发送一条消息到总线, 返回分配的消息 ID.
func (b *MessageBus) Post(ctx context.Context, round int, role string, from, to RoleName, content, metadata string) (BusMessage, error) {
	b.mu.Lock()
	b.nextID++
	msg := BusMessage{
		ID:        b.nextID,
		RunID:     b.runID,
		Round:     round,
		Role:      role,
		FromAgent: from,
		ToAgent:   to,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
	b.messages = append(b.messages, msg)
	b.mu.Unlock()

	if b.store != nil {
		row := store.AgentMessageRow{
			RunID:     msg.RunID,
			Round:     msg.Round,
			Role:      msg.Role,
			FromAgent: string(msg.FromAgent),
			ToAgent:   string(msg.ToAgent),
			Content:   msg.Content,
			Metadata:  msg.Metadata,
		}
		id, err := b.store.PutAgentMessage(ctx, row)
		if err != nil {
			return msg, fmt.Errorf("persist message: %w", err)
		}
		msg.ID = id
	}
	return msg, nil
}

// Messages 返回总线上所有消息的副本.
func (b *MessageBus) Messages() []BusMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]BusMessage, len(b.messages))
	copy(out, b.messages)
	return out
}

// MessagesFor 返回某个 agent 收到的全部消息.
func (b *MessageBus) MessagesFor(agent RoleName) []BusMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []BusMessage
	for _, m := range b.messages {
		if m.ToAgent == agent || m.FromAgent == agent {
			out = append(out, m)
		}
	}
	return out
}

// MessagesByRound 返回某一轮的全部消息.
func (b *MessageBus) MessagesByRound(round int) []BusMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []BusMessage
	for _, m := range b.messages {
		if m.Round == round {
			out = append(out, m)
		}
	}
	return out
}

// LastRound 返回最大 round 数 (0 表示还没有消息).
func (b *MessageBus) LastRound() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	max := 0
	for _, m := range b.messages {
		if m.Round > max {
			max = m.Round
		}
	}
	return max
}

// Count 返回消息总数.
func (b *MessageBus) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.messages)
}

// RunID 返回当前 run 的 ID.
func (b *MessageBus) RunID() string {
	return b.runID
}

// nextRunID 生成一个唯一的 run ID.
var runCounter int64

func NextRunID() string {
	n := atomic.AddInt64(&runCounter, 1)
	return fmt.Sprintf("run_%d_%d", time.Now().Unix(), n)
}
