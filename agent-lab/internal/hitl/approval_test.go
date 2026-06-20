package hitl

import (
	"context"
	"path/filepath"
	"testing"

	"ai-learn-playground/agent-lab/internal/store"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewManager(st)
}

func TestApproval_CreateAndGet(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	a, err := mgr.Create(ctx, "ap_001", "conv_1", 2, "shopee_publish", `{"sku":"sku_001"}`, "dry-run: will create listing", RiskHigh)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Status != StatusPending {
		t.Fatalf("status=%s, want pending", a.Status)
	}
	if a.RiskLevel != RiskHigh {
		t.Fatalf("risk=%s, want high", a.RiskLevel)
	}

	got, err := mgr.Get(ctx, "ap_001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Tool != "shopee_publish" {
		t.Fatalf("tool=%s", got.Tool)
	}
	if got.Args != `{"sku":"sku_001"}` {
		t.Fatalf("args=%s", got.Args)
	}
}

func TestApproval_Approve(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	_, _ = mgr.Create(ctx, "ap_002", "conv_1", 1, "price_update", `{"price":690}`, "", RiskMedium)

	a, err := mgr.Approve(ctx, "ap_002", "admin", "looks good")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if a.Status != StatusApproved {
		t.Fatalf("status=%s, want approved", a.Status)
	}
	if a.Reviewer != "admin" {
		t.Fatalf("reviewer=%s", a.Reviewer)
	}
	if a.Note != "looks good" {
		t.Fatalf("note=%s", a.Note)
	}
}

func TestApproval_Reject(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	_, _ = mgr.Create(ctx, "ap_003", "conv_1", 1, "shopee_publish", `{}`, "", RiskHigh)

	a, err := mgr.Reject(ctx, "ap_003", "admin", "价格不对")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if a.Status != StatusRejected {
		t.Fatalf("status=%s, want rejected", a.Status)
	}
}

func TestApproval_Edit(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	_, _ = mgr.Create(ctx, "ap_004", "conv_1", 1, "price_update", `{"price":690}`, "", RiskMedium)

	a, err := mgr.Edit(ctx, "ap_004", "admin", "adjusted price", `{"price":750}`)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if a.Status != StatusEdited {
		t.Fatalf("status=%s, want edited", a.Status)
	}
	if a.EditedArgs != `{"price":750}` {
		t.Fatalf("edited_args=%s", a.EditedArgs)
	}
}

func TestApproval_ListPending(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	_, _ = mgr.Create(ctx, "ap_005", "conv_1", 1, "tool_a", `{}`, "", RiskLow)
	_, _ = mgr.Create(ctx, "ap_006", "conv_1", 2, "tool_b", `{}`, "", RiskHigh)
	_, _ = mgr.Approve(ctx, "ap_005", "admin", "ok")

	pending, err := mgr.ListPending(ctx)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ID != "ap_006" {
		t.Fatalf("pending id=%s, want ap_006", pending[0].ID)
	}

	count, _ := mgr.CountPending(ctx)
	if count != 1 {
		t.Fatalf("count=%d, want 1", count)
	}
}

func TestApproval_PersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	mgr := NewManager(st)
	_, _ = mgr.Create(ctx, "ap_007", "conv_1", 1, "tool_x", `{"a":1}`, "", RiskHigh)
	st.Close()

	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	mgr2 := NewManager(st2)
	got, err := mgr2.Get(ctx, "ap_007")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Tool != "tool_x" {
		t.Fatalf("tool=%s after reopen", got.Tool)
	}
	if got.Status != StatusPending {
		t.Fatalf("status=%s after reopen", got.Status)
	}
}

func TestApproval_DoubleApproveFails(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	_, _ = mgr.Create(ctx, "ap_008", "conv_1", 1, "tool_x", `{}`, "", RiskLow)
	_, _ = mgr.Approve(ctx, "ap_008", "admin", "ok")

	// 第二次 approve 应该不改变状态 (WHERE status='pending' 不匹配).
	a, err := mgr.Approve(ctx, "ap_008", "admin2", "second")
	if err != nil {
		t.Fatalf("second approve: %v", err)
	}
	if a.Reviewer == "admin2" {
		t.Fatal("second approve should not have changed reviewer (already not pending)")
	}
}
