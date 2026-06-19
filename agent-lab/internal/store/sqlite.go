package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"

	// 纯 Go SQLite 驱动, 注册 "sqlite" 到 database/sql.
	_ "modernc.org/sqlite"
)

// ErrNotFound 表示按主键查询未命中. 调用方用它区分 "没有" 与 "出错".
var ErrNotFound = errors.New("store: not found")

// Store 包装一个 *sql.DB, 提供长期 KV 与会话持久化.
type Store struct {
	db *sql.DB
}

// Open 打开 (或创建) path 处的 SQLite 文件并执行 migration.
// path 为 ":memory:" 时走纯内存库 (测试用).
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// SQLite 只允许一个写者. 用单连接让 database/sql 串行化所有操作,
	// 既避免 SQLITE_BUSY (多连接竞争), 也让 ":memory:" 在池里共享同一个库.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// WAL + busy_timeout: 即便单连接, 也设上保险 (例如外部进程同时打开同一文件).
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	for _, stmt := range migrations {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration: %w\nstmt: %s", err, stmt)
		}
	}
	return nil
}

// Close 关闭底层连接池.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ---------------- 长期 KV ----------------

// KVEntry 是 memory_kv 的一行.
type KVEntry struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at"`
}

// PutKV 写入 (或覆盖) 一个键. value 应为 JSON 字符串.
func (s *Store) PutKV(ctx context.Context, namespace, key, value string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_kv(namespace, key, value, updated_at) VALUES(?,?,?,?)
		 ON CONFLICT(namespace, key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		namespace, key, value, now)
	if err != nil {
		return fmt.Errorf("put kv %s/%s: %w", namespace, key, err)
	}
	return nil
}

// GetKV 读取一个键. 未命中时返回 ("", false, nil).
func (s *Store) GetKV(ctx context.Context, namespace, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM memory_kv WHERE namespace=? AND key=?`, namespace, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get kv %s/%s: %w", namespace, key, err)
	}
	return value, true, nil
}

// DeleteKV 删除一个键. 不存在不算错误.
func (s *Store) DeleteKV(ctx context.Context, namespace, key string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM memory_kv WHERE namespace=? AND key=?`, namespace, key)
	if err != nil {
		return fmt.Errorf("delete kv %s/%s: %w", namespace, key, err)
	}
	return nil
}

// ListKV 返回某 namespace 下所有键, 按 key 字典序.
func (s *Store) ListKV(ctx context.Context, namespace string) ([]KVEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT namespace, key, value, updated_at FROM memory_kv WHERE namespace=? ORDER BY key`,
		namespace)
	if err != nil {
		return nil, fmt.Errorf("list kv %s: %w", namespace, err)
	}
	defer rows.Close()
	var out []KVEntry
	for rows.Next() {
		var e KVEntry
		if err := rows.Scan(&e.Namespace, &e.Key, &e.Value, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Namespaces 返回所有出现过的 namespace, 按字典序.
func (s *Store) Namespaces(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT namespace FROM memory_kv ORDER BY namespace`)
	if err != nil {
		return nil, fmt.Errorf("namespaces: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return nil, err
		}
		out = append(out, ns)
	}
	return out, rows.Err()
}

// ---------------- 会话持久化 ----------------

// ConvRow 是 conversations 表的一行 (不含消息体).
type ConvRow struct {
	ID        string `json:"id"`
	SellerID  string `json:"seller_id"`
	Title     string `json:"title"`
	System    string `json:"system"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// SaveConversation 写入 (或覆盖) 一个会话: upsert 会话行 + 全量替换消息行.
// created_at 在首次插入时设为当前时间, 后续更新保持不变.
func (s *Store) SaveConversation(ctx context.Context, id, sellerID, title, system string, msgs []llm.Message) error {
	if id == "" {
		return errors.New("SaveConversation: empty id")
	}
	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO conversations(id, seller_id, title, system, created_at, updated_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   seller_id=excluded.seller_id,
		   title=excluded.title,
		   system=excluded.system,
		   updated_at=excluded.updated_at`,
		id, sellerID, title, system, now, now); err != nil {
		return fmt.Errorf("upsert conversation %s: %w", id, err)
	}
	// 全量替换消息: 先删后插, 保证 100% 还原 (含被编辑/删除的轮次).
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_messages WHERE conv_id=?`, id); err != nil {
		return fmt.Errorf("clear messages %s: %w", id, err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO conversation_messages(conv_id, idx, role, content) VALUES(?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare msg insert: %w", err)
	}
	defer stmt.Close()
	for i, m := range msgs {
		raw, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("encode msg %d: %w", i, err)
		}
		if _, err := stmt.ExecContext(ctx, id, i, string(m.Role), string(raw)); err != nil {
			return fmt.Errorf("insert msg %d: %w", i, err)
		}
	}
	return tx.Commit()
}

// LoadConversation 读取一个会话的元信息与全部消息 (按 idx 升序).
func (s *Store) LoadConversation(ctx context.Context, id string) (ConvRow, []llm.Message, error) {
	var row ConvRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, seller_id, title, system, created_at, updated_at FROM conversations WHERE id=?`, id).
		Scan(&row.ID, &row.SellerID, &row.Title, &row.System, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ConvRow{}, nil, ErrNotFound
	}
	if err != nil {
		return ConvRow{}, nil, fmt.Errorf("load conversation %s: %w", id, err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT content FROM conversation_messages WHERE conv_id=? ORDER BY idx`, id)
	if err != nil {
		return row, nil, fmt.Errorf("load messages %s: %w", id, err)
	}
	defer rows.Close()
	var msgs []llm.Message
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return row, nil, err
		}
		var m llm.Message
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return row, nil, fmt.Errorf("decode msg: %w", err)
		}
		msgs = append(msgs, m)
	}
	return row, msgs, rows.Err()
}

// ListConversations 返回所有会话元信息, 按 updated_at 倒序.
func (s *Store) ListConversations(ctx context.Context) ([]ConvRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, seller_id, title, system, created_at, updated_at FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	var out []ConvRow
	for rows.Next() {
		var r ConvRow
		if err := rows.Scan(&r.ID, &r.SellerID, &r.Title, &r.System, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteConversation 删除一个会话及其全部消息.
func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_messages WHERE conv_id=?`, id); err != nil {
		return fmt.Errorf("delete messages %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete conversation %s: %w", id, err)
	}
	return tx.Commit()
}
