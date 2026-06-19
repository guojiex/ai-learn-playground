package memory

import (
	"context"
	"path/filepath"
	"testing"

	"ai-learn-playground/agent-lab/internal/store"
)

func TestKV_PutGetListDelete(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	kv := NewKV(st)
	ctx := context.Background()

	ns := SellerNamespace("A001")
	if ns != "seller:A001" {
		t.Fatalf("SellerNamespace=%q", ns)
	}

	// miss
	if _, found, _ := kv.Get(ctx, ns, "tone"); found {
		t.Fatal("expected miss")
	}
	// put + get
	if err := kv.Put(ctx, ns, "tone", `{"style":"girlfriend"}`); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, found, err := kv.Get(ctx, ns, "tone")
	if err != nil || !found || got != `{"style":"girlfriend"}` {
		t.Fatalf("get: got=%q found=%v err=%v", got, found, err)
	}
	// list + namespaces
	if err := kv.Put(ctx, ns, "keywords", `["現貨"]`); err != nil {
		t.Fatalf("put keywords: %v", err)
	}
	entries, err := kv.List(ctx, ns)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}
	nss, err := kv.Namespaces(ctx)
	if err != nil {
		t.Fatalf("namespaces: %v", err)
	}
	if len(nss) != 1 || nss[0] != ns {
		t.Fatalf("namespaces=%v", nss)
	}
	// delete
	if err := kv.Delete(ctx, ns, "tone"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, found, _ := kv.Get(ctx, ns, "tone"); found {
		t.Fatal("still found after delete")
	}
}

func TestKV_RejectsEmpty(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	kv := NewKV(st)
	ctx := context.Background()
	if err := kv.Put(ctx, "", "k", "v"); err == nil {
		t.Fatal("expected error for empty namespace")
	}
	if err := kv.Put(ctx, "ns", "", "v"); err == nil {
		t.Fatal("expected error for empty key")
	}
}
