package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/store"
)

func newTestKV(t *testing.T) *memory.KV {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return memory.NewKV(st)
}

func TestMemoryPutAndGet_RoundTrip(t *testing.T) {
	kv := newTestKV(t)
	put := NewMemoryPut(kv)
	get := NewMemoryGet(kv)
	ctx := context.Background()

	if put.Schema().Function.Name != "memory_put" {
		t.Fatal("schema name")
	}
	if get.Schema().Function.Name != "memory_get" {
		t.Fatal("schema name")
	}

	// put: value 是合法 JSON 字符串
	out, err := put.Invoke(ctx, json.RawMessage(`{"namespace":"seller:A001","key":"tone","value":"{\"style\":\"girlfriend\",\"emoji\":\"high\"}"}`))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	var pres map[string]any
	if err := json.Unmarshal([]byte(out), &pres); err != nil {
		t.Fatal(err)
	}
	if pres["ok"] != true {
		t.Fatalf("put out: %s", out)
	}

	// get: 命中
	out, err = get.Invoke(ctx, json.RawMessage(`{"namespace":"seller:A001","key":"tone"}`))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var gres map[string]any
	if err := json.Unmarshal([]byte(out), &gres); err != nil {
		t.Fatal(err)
	}
	if gres["found"] != true {
		t.Fatalf("expected found=true: %s", out)
	}
	if gres["value"] != `{"style":"girlfriend","emoji":"high"}` {
		t.Fatalf("value mismatch: %s", out)
	}
}

func TestMemoryGet_NotFound(t *testing.T) {
	kv := newTestKV(t)
	get := NewMemoryGet(kv)
	out, err := get.Invoke(context.Background(), json.RawMessage(`{"namespace":"seller:X","key":"tone"}`))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var res map[string]any
	_ = json.Unmarshal([]byte(out), &res)
	if res["found"] != false {
		t.Fatalf("expected found=false: %s", out)
	}
}

func TestMemoryPut_RejectsInvalidJSON(t *testing.T) {
	kv := newTestKV(t)
	put := NewMemoryPut(kv)
	_, err := put.Invoke(context.Background(), json.RawMessage(`{"namespace":"seller:A001","key":"tone","value":"not json"}`))
	if err == nil {
		t.Fatal("expected error for non-JSON value")
	}
}

func TestMemoryPut_RequiresAllFields(t *testing.T) {
	kv := newTestKV(t)
	put := NewMemoryPut(kv)
	// 缺 value
	if _, err := put.Invoke(context.Background(), json.RawMessage(`{"namespace":"seller:A001","key":"tone"}`)); err == nil {
		t.Fatal("expected error for missing value")
	}
	// 缺 namespace
	if _, err := put.Invoke(context.Background(), json.RawMessage(`{"key":"tone","value":"{}"}`)); err == nil {
		t.Fatal("expected error for missing namespace")
	}
}
