package agent

import (
	"context"
	"path/filepath"
	"testing"

	"ai-learn-playground/agent-lab/internal/store"
)

func TestMessageBus_PostAndRetrieve(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	bus := NewMessageBus("test_run", st)
	ctx := context.Background()

	_, err = bus.Post(ctx, 1, "user", RoleCoordinator, RoleResearcher, "收集 towel 的卖点", "")
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_, err = bus.Post(ctx, 1, "assistant", RoleResearcher, RoleCoordinator, `{"facts":["吸水","蓬鬆"]}`, "")
	if err != nil {
		t.Fatalf("post2: %v", err)
	}
	_, _ = bus.Post(ctx, 2, "user", RoleCoordinator, RoleWriter, "写文案", "")

	msgs := bus.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	round1 := bus.MessagesByRound(1)
	if len(round1) != 2 {
		t.Fatalf("expected 2 messages in round 1, got %d", len(round1))
	}

	researcherMsgs := bus.MessagesFor(RoleResearcher)
	if len(researcherMsgs) != 2 {
		t.Fatalf("expected 2 messages for researcher, got %d", len(researcherMsgs))
	}

	if bus.LastRound() != 2 {
		t.Fatalf("last round=%d, want 2", bus.LastRound())
	}

	if bus.Count() != 3 {
		t.Fatalf("count=%d, want 3", bus.Count())
	}
}

func TestMessageBus_PersistToSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	bus := NewMessageBus("persist_run", st)
	_, _ = bus.Post(ctx, 1, "user", RoleCoordinator, RoleResearcher, "hello", "")
	_, _ = bus.Post(ctx, 1, "assistant", RoleResearcher, RoleCoordinator, "world", "")
	st.Close()

	// 重开: 消息应持久化.
	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	rows, err := st2.LoadAgentMessages(ctx, "persist_run")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 persisted messages, got %d", len(rows))
	}
	if rows[0].FromAgent != "coordinator" || rows[0].ToAgent != "researcher" {
		t.Fatalf("row 0 mismatch: %+v", rows[0])
	}
}

func TestMessageBus_NoStore(t *testing.T) {
	bus := NewMessageBus("mem_only", nil)
	ctx := context.Background()
	_, err := bus.Post(ctx, 1, "user", RoleCoordinator, RoleWriter, "test", "")
	if err != nil {
		t.Fatalf("post without store: %v", err)
	}
	if bus.Count() != 1 {
		t.Fatalf("count=%d, want 1", bus.Count())
	}
}
