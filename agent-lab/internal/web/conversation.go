package web

import (
	"sync"
	"time"

	"ai-learn-playground/agent-lab/internal/memory"
)

// Conversation 是一个会话条目, 用于 Web UI 的会话列表.
type Conversation struct {
	ID        string
	Title     string
	UpdatedAt time.Time
	Memory    *memory.ShortTerm
}

// ConversationStore 是进程内的会话存储器.
// M1 只支持内存存储 (重启会清空). M4 会替换为 SQLite + 向量.
type ConversationStore struct {
	mu   sync.RWMutex
	data map[string]*Conversation
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{data: make(map[string]*Conversation)}
}

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
	c := &Conversation{
		ID:        id,
		Title:     title,
		UpdatedAt: time.Now(),
		Memory:    memory.NewShortTerm(systemPrompt, budget, reserve),
	}
	s.data[id] = c
	return c
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
			Title:     title,
			UpdatedAt: c.UpdatedAt,
		})
	}
	// 简易倒序 (M1 数据量极少, 用 sort 反而要引包).
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
	if c, ok := s.data[id]; ok {
		c.Title = title
		c.UpdatedAt = time.Now()
		return true
	}
	return false
}

// Delete 删除一个会话.
func (s *ConversationStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; ok {
		delete(s.data, id)
		return true
	}
	return false
}

// Touch 标记会话已更新.
func (s *ConversationStore) Touch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.data[id]; ok {
		c.UpdatedAt = time.Now()
	}
}
