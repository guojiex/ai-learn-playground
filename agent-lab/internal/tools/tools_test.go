package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestProducts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `[
  {"id":"sku_001","name":"今治毛巾","brand":"今治本舗","price_twd":690,"shipping":"現貨","highlights":["蓬鬆","日本製"],"platforms":["shopee_tw"]},
  {"id":"sku_002","name":"保溫瓶","brand":"鐵漢","price_twd":480,"shipping":"限時免運","highlights":["316","保溫"],"platforms":["pchome"]}
]`
	if err := os.WriteFile(filepath.Join(dir, "products.json"), []byte(body), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

func TestProductLookup_ByID(t *testing.T) {
	dir := writeTestProducts(t)
	tool := NewProductLookup(dir)
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"id":"sku_002"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	var arr []Product
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arr) != 1 || arr[0].ID != "sku_002" {
		t.Fatalf("unexpected hits: %s", out)
	}
}

func TestProductLookup_ByQuery(t *testing.T) {
	dir := writeTestProducts(t)
	tool := NewProductLookup(dir)
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"query":"毛巾"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !strings.Contains(out, "sku_001") {
		t.Fatalf("expected sku_001 in: %s", out)
	}
}

func TestProductLookup_RequiresIDOrQuery(t *testing.T) {
	dir := writeTestProducts(t)
	tool := NewProductLookup(dir)
	if _, err := tool.Invoke(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for empty args")
	}
}

func TestPriceFormat_Display(t *testing.T) {
	out, err := NewPriceFormat().Invoke(context.Background(), json.RawMessage(`{"price_twd":690,"shipping":"現貨","badges":["限時免運","滿千折百"]}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !strings.Contains(out, "NT$690") {
		t.Fatalf("missing NT$ prefix: %s", out)
	}
	if !strings.Contains(out, "限時免運") {
		t.Fatalf("missing badge: %s", out)
	}
}

func TestPriceFormat_RejectNonPositive(t *testing.T) {
	if _, err := NewPriceFormat().Invoke(context.Background(), json.RawMessage(`{"price_twd":0}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestPlatformLint_Detects(t *testing.T) {
	body := `{"platform":"shopee_tw","text":"最高品質的台灣茶葉, 第一名 #茶 #台灣 #好茶 #送禮 #限量 #現貨 #免運 #必買 #推薦","kind":"title"}`
	out, err := NewPlatformLint().Invoke(context.Background(), json.RawMessage(body))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !strings.Contains(out, "banned_word") {
		t.Fatalf("expected banned_word: %s", out)
	}
	if !strings.Contains(out, "too_many_tags") {
		t.Fatalf("expected too_many_tags: %s", out)
	}
}

func TestPlatformLint_Ok(t *testing.T) {
	body := `{"platform":"shopee_tw","text":"日本製今治毛巾 蓬鬆吸水 #毛巾 #日本","kind":"title"}`
	out, err := NewPlatformLint().Invoke(context.Background(), json.RawMessage(body))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !strings.Contains(out, `"ok":true`) {
		t.Fatalf("expected ok=true: %s", out)
	}
}

func TestSlangCheck_Counts(t *testing.T) {
	body := `{"text":"限時下殺 現貨免運, CP值高的必買神器"}`
	out, err := NewSlangCheck().Invoke(context.Background(), json.RawMessage(body))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !strings.Contains(out, `"total_hits"`) {
		t.Fatalf("missing total_hits: %s", out)
	}
}

func TestRegistry_RegisterAndSchemas(t *testing.T) {
	r := NewRegistry()
	r.Register(NewPriceFormat())
	r.Register(NewSlangCheck())
	if names := r.Names(); len(names) != 2 || names[0] != "price_format" || names[1] != "slang_check" {
		t.Fatalf("unexpected names: %v", names)
	}
	schemas := r.Schemas()
	if len(schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(schemas))
	}
	if schemas[0].Type != "function" {
		t.Fatalf("type field should be 'function', got %q", schemas[0].Type)
	}
	if _, ok := r.Get("price_format"); !ok {
		t.Fatal("missing price_format in registry")
	}
	if _, ok := r.Get("nope"); ok {
		t.Fatal("nope should not be registered")
	}
}
