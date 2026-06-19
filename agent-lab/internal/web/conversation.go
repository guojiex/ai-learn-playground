package web

import (
	"context"
	"sync"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/store"
)

// Conversation 是一个会话条目, 用于 Web UI 的会话列表.
type Conversation struct {
	ID        string            `json:"id"`
	SellerID  string            `json:"seller_id"`
	Title     string            `json:"title"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Memory    *memory.ShortTerm `json:"memory,omitempty"`
}

// ConversationStore 是进程内的会话存储器.
//
// M4 起: 当注入 store 时, 会话变更会 write-through 到 SQLite (agent.db),
// 重启后通过 Hydrate 恢复, 实现 "刷新/重启不丢历史".
type ConversationStore struct {
	mu    sync.RWMutex
	data  map[string]*Conversation
	store *store.Store
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{data: make(map[string]*Conversation)}
}

// EnablePersistence 接入 SQLite 持久层. 之后 New/Rename/Delete/Persist 都会落库.
func (s *ConversationStore) EnablePersistence(st *store.Store) {
	s.store = st
}

// HasStore 返回是否启用了持久化 (server 用来决定是否要 hydrate / persist).
func (s *ConversationStore) HasStore() bool { return s.store != nil }

// New 创建一个新会话. systemPrompt 为该会话的角色卡.
func (s *ConversationStore) New(id, title, systemPrompt string, budget, reserve int) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = "c_" + time.Now().Format("20060102150405")
	}
	if title == "" {
		title = "新对话 " + time.Now().Format("15:04")
	}
	now := time.Now()
	c := &Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Memory:    memory.NewShortTerm(systemPrompt, budget, reserve),
	}
	s.data[id] = c
	if s.store != nil {
		_ = s.store.SaveConversation(context.Background(), c.ID, c.SellerID, c.Title, c.Memory.System(), c.Memory.Messages())
	}
	return c
}

// Restore 从 SQLite 行重建一个内存会话 (Hydrate 时使用).
func (s *ConversationStore) Restore(id, sellerID, title, system string, createdAt time.Time, msgs []llm.Message, budget, reserve int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := &Conversation{
		ID:        id,
		SellerID:  sellerID,
		Title:     title,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Memory:    memory.NewShortTerm(system, budget, reserve),
	}
	c.Memory.ResetWith(msgs)
	s.data[id] = c
}

// Persist 把一个会话的当前快照写入 SQLite (会话行 + 全量消息).
func (s *ConversationStore) Persist(ctx context.Context, c *Conversation) error {
	if s.store == nil || c == nil {
		return nil
	}
	return s.store.SaveConversation(ctx, c.ID, c.SellerID, c.Title, c.Memory.System(), c.Memory.Messages())
}

// Get 按 ID 查找. 找不到返回 nil.
func (s *ConversationStore) Get(id string) *Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[id]
}

// List 返回所有会话, 按更新时间倒序. title 为空时给一个时间兜底.
func (s *ConversationStore) List() []Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Conversation, 0, len(s.data))
	for _, c := range s.data {
		title := c.Title
		if title == "" {
			title = "新对话 " + c.UpdatedAt.Format("15:04")
		}
		out = append(out, Conversation{
			ID:        c.ID,
			SellerID:  c.SellerID,
			Title:     title,
			UpdatedAt: c.UpdatedAt,
		})
	}
	// 简易倒序 (数据量极少, 用 sort 反而要引包).
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].UpdatedAt.After(out[i].UpdatedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Rename 重命名一个会话.
func (s *ConversationStore) Rename(id, title string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data[id]
	if !ok {
		return false
	}
	c.Title = title
	c.UpdatedAt = time.Now()
	if s.store != nil {
		_ = s.store.SaveConversation(context.Background(), c.ID, c.SellerID, c.Title, c.Memory.System(), c.Memory.Messages())
	}
	return true
}

// Delete 删除一个会话.
func (s *ConversationStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[id]
	if !ok {
		// 幂等: 即使内存里没有 (重启后的幽灵会话), 也尝试删库.
		if s.store != nil {
			_ = s.store.DeleteConversation(context.Background(), id)
		}
		return false
	}
	delete(s.data, id)
	if s.store != nil {
		_ = s.store.DeleteConversation(context.Background(), id)
	}
	return true
}

// Touch 标记会话已更新.
func (s *ConversationStore) Touch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.data[id]; ok {
		c.UpdatedAt = time.Now()
	}
}
