package memory

import (
	"context"
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/store"
)

// KV 是长期记忆的对外门面: 在 store 之上加上 namespace 约定与 JSON value 约定.
//
// 业务约定 (见 M4):
//
//	seller:{id}:tone     -> {"style":"girlfriend","emoji":"high","price_position":"end"}
//	seller:{id}:keywords -> ["現貨","免運","限時"]
//
// 这里的 namespace = "seller:{id}", key = "tone" / "keywords", value 是 JSON 字符串.
type KV struct {
	s *store.Store
}

// NewKV 用一个已打开的 store 构造长期记忆.
func NewKV(s *store.Store) *KV {
	return &KV{s: s}
}

// Put 写入一个键. value 直接以字符串存储 (调用方负责 JSON 化).
func (k *KV) Put(ctx context.Context, namespace, key, value string) error {
	if k == nil || k.s == nil {
		return fmt.Errorf("kv: store not configured")
	}
	namespace = strings.TrimSpace(namespace)
	key = strings.TrimSpace(key)
	if namespace == "" || key == "" {
		return fmt.Errorf("kv: namespace and key are required")
	}
	return k.s.PutKV(ctx, namespace, key, value)
}

// Get 读取一个键. found=false 表示不存在.
func (k *KV) Get(ctx context.Context, namespace, key string) (value string, found bool, err error) {
	if k == nil || k.s == nil {
		return "", false, fmt.Errorf("kv: store not configured")
	}
	return k.s.GetKV(ctx, namespace, key)
}

// Delete 删除一个键.
func (k *KV) Delete(ctx context.Context, namespace, key string) error {
	if k == nil || k.s == nil {
		return fmt.Errorf("kv: store not configured")
	}
	return k.s.DeleteKV(ctx, namespace, key)
}

// List 返回某 namespace 下所有条目.
func (k *KV) List(ctx context.Context, namespace string) ([]store.KVEntry, error) {
	if k == nil || k.s == nil {
		return nil, fmt.Errorf("kv: store not configured")
	}
	return k.s.ListKV(ctx, namespace)
}

// Namespaces 返回所有 namespace, 用于 Memory 面板的折叠树.
func (k *KV) Namespaces(ctx context.Context) ([]string, error) {
	if k == nil || k.s == nil {
		return nil, fmt.Errorf("kv: store not configured")
	}
	return k.s.Namespaces(ctx)
}

// SellerNamespace 返回某个卖家的长期记忆 namespace, 例如 "seller:A001".
func SellerNamespace(sellerID string) string {
	return "seller:" + strings.TrimSpace(sellerID)
}
