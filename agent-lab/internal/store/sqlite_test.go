package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestOpen_Idempotent 验证重复 Open (二次迁移) 不报错, schema 仍在.
func TestOpen_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	ctx := context.Background()
	if err := st.PutKV(ctx, "ns", "k", "v"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 再次打开: migration 应幂等, 之前写入的数据还在.
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer st2.Close()
	got, found, err := st2.GetKV(ctx, "ns", "k")
	if err != nil || !found || got != "v" {
		t.Fatalf("after reopen: got=%q found=%v err=%v", got, found, err)
	}
}

func TestKV_PutGetDelete(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// 未命中.
	if _, found, err := st.GetKV(ctx, "seller:A001", "tone"); err != nil || found {
		t.Fatalf("expected miss, got found=%v err=%v", found, err)
	}
	// 写入.
	if err := st.PutKV(ctx, "seller:A001", "tone", `{"style":"girlfriend"}`); err != nil {
		t.Fatalf("put: %v", err)
	}
	// 命中.
	got, found, err := st.GetKV(ctx, "seller:A001", "tone")
	if err != nil || !found || got != `{"style":"girlfriend"}` {
		t.Fatalf("get: got=%q found=%v err=%v", got, found, err)
	}
	// 覆盖.
	if err := st.PutKV(ctx, "seller:A001", "tone", `{"style":"pro"}`); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	got, _, _ = st.GetKV(ctx, "seller:A001", "tone")
	if got != `{"style":"pro"}` {
		t.Fatalf("overwrite not applied: %q", got)
	}
	// List + Namespaces.
	if err := st.PutKV(ctx, "seller:A001", "keywords", `["現貨"]`); err != nil {
		t.Fatalf("put keywords: %v", err)
	}
	entries, err := st.ListKV(ctx, "seller:A001")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	nss, err := st.Namespaces(ctx)
	if err != nil {
		t.Fatalf("namespaces: %v", err)
	}
	if len(nss) != 1 || nss[0] != "seller:A001" {
		t.Fatalf("namespaces=%v", nss)
	}
	// 删除.
	if err := st.DeleteKV(ctx, "seller:A001", "tone"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, found, _ := st.GetKV(ctx, "seller:A001", "tone"); found {
		t.Fatalf("still found after delete")
	}
}

// TestKV_ConcurrentWrites 验证并发写入不会破坏 schema / 丢数据.
func TestKV_ConcurrentWrites(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ns := fmt.Sprintf("seller:A%03d", i%5)
			key := fmt.Sprintf("k%d", i)
			if err := st.PutKV(ctx, ns, key, fmt.Sprintf(`{"i":%d}`, i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent put: %v", err)
	}
	// 抽查若干条都在.
	for i := 0; i < n; i += 7 {
		ns := fmt.Sprintf("seller:A%03d", i%5)
		key := fmt.Sprintf("k%d", i)
		got, found, err := st.GetKV(ctx, ns, key)
		if err != nil || !found {
			t.Fatalf("missing %s/%s after concurrent writes: found=%v err=%v", ns, key, found, err)
		}
		if got != fmt.Sprintf(`{"i":%d}`, i) {
			t.Fatalf("value mismatch %s/%s: %q", ns, key, got)
		}
	}
	// schema 仍可查询 (没有损坏).
	if _, err := st.Namespaces(ctx); err != nil {
		t.Fatalf("namespaces after concurrent writes: %v", err)
	}
}

func TestConversation_SaveLoadRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	st1, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "你是电商文案助理"},
		{Role: llm.RoleUser, Content: "帮我写蝦皮標題"},
		{Role: llm.RoleAssistant, Content: "現貨免運 日本製毛巾"},
	}
	if err := st1.SaveConversation(ctx, "c_1", "A001", "第一单", "你是电商文案助理", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 同进程 Load: 100% 还原.
	row, loaded, err := st1.LoadConversation(ctx, "c_1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if row.SellerID != "A001" || row.Title != "第一单" || row.System != "你是电商文案助理" {
		t.Fatalf("row mismatch: %+v", row)
	}
	if len(loaded) != len(msgs) {
		t.Fatalf("msg count: %d vs %d", len(loaded), len(msgs))
	}
	for i, m := range loaded {
		if m.Role != msgs[i].Role || m.Content != msgs[i].Content {
			t.Fatalf("msg %d mismatch: %+v vs %+v", i, m, msgs[i])
		}
	}

	// 关库重开: 持久化生效.
	if err := st1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	rows, err := st2.ListConversations(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "c_1" {
		t.Fatalf("list rows: %+v", rows)
	}
	_, loaded2, err := st2.LoadConversation(ctx, "c_1")
	if err != nil {
		t.Fatalf("load after reopen: %v", err)
	}
	if len(loaded2) != len(msgs) {
		t.Fatalf("restore count mismatch: %d", len(loaded2))
	}
	// 模拟多次对话追加后再保存, 验证全量替换不残留旧消息.
	msgs2 := append([]llm.Message{}, msgs[:2]...) // 故意变少
	if err := st2.SaveConversation(ctx, "c_1", "A001", "第一单", "你是电商文案助理", msgs2); err != nil {
		t.Fatalf("resave: %v", err)
	}
	_, loaded3, err := st2.LoadConversation(ctx, "c_1")
	if err != nil {
		t.Fatalf("load after resave: %v", err)
	}
	if len(loaded3) != len(msgs2) {
		t.Fatalf("expected %d msgs after replace, got %d", len(msgs2), len(loaded3))
	}
}

func TestConversation_Delete(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.SaveConversation(ctx, "c_x", "A001", "t", "sys", []llm.Message{{Role: llm.RoleUser, Content: "hi"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := st.DeleteConversation(ctx, "c_x"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, _, err := st.LoadConversation(ctx, "c_x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
