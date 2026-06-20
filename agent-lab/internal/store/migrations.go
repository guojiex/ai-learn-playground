// Package store 是 agent-lab 的单文件持久层 (SQLite).
//
// 设计目标 (M4):
//   - 所有持久状态 (长期 KV / 会话 / 后续 trace / approval) 都进同一个 agent.db,
//     便于备份与回放 (见 ADR-0005).
//   - migration 幂等: 每次 Open 都重新执行 CREATE TABLE IF NOT EXISTS, 不维护版本号
//     也能安全重复启动; 后续里程碑新增表时只需往 migrations 追加一条.
//   - 驱动用 modernc.org/sqlite (纯 Go, 无 CGO), 与 "纯 Go" 哲学一致.
package store

// migrations 是按顺序执行的 schema 语句. 每条都必须幂等 (IF NOT EXISTS),
// 这样 Open 重复调用不会报错, 也不需要单独的版本表.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS memory_kv (
		namespace  TEXT NOT NULL,
		key        TEXT NOT NULL,
		value      TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY (namespace, key)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_memory_kv_namespace ON memory_kv(namespace)`,
	`CREATE TABLE IF NOT EXISTS conversations (
		id         TEXT PRIMARY KEY,
		seller_id  TEXT NOT NULL DEFAULT '',
		title      TEXT NOT NULL DEFAULT '',
		system     TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_conversations_seller ON conversations(seller_id)`,
	`CREATE TABLE IF NOT EXISTS conversation_messages (
		conv_id TEXT NOT NULL,
		idx     INTEGER NOT NULL,
		role    TEXT NOT NULL,
		content TEXT NOT NULL,
		PRIMARY KEY (conv_id, idx)
	)`,
	`CREATE TABLE IF NOT EXISTS documents (
		id          TEXT PRIMARY KEY,
		source      TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		text        TEXT NOT NULL,
		embedding   BLOB NOT NULL,
		metadata    TEXT NOT NULL DEFAULT '{}',
		created_at  INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source)`,
	`CREATE TABLE IF NOT EXISTS agent_messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id     TEXT NOT NULL,
		round      INTEGER NOT NULL,
		role       TEXT NOT NULL,
		from_agent TEXT NOT NULL,
		to_agent   TEXT NOT NULL,
		content    TEXT NOT NULL,
		metadata   TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_agent_messages_run ON agent_messages(run_id)`,
	`CREATE TABLE IF NOT EXISTS approvals (
		id          TEXT PRIMARY KEY,
		conv_id     TEXT NOT NULL,
		step_idx    INTEGER NOT NULL DEFAULT 0,
		tool        TEXT NOT NULL,
		args        TEXT NOT NULL,
		payload     TEXT NOT NULL DEFAULT '',
		risk_level  TEXT NOT NULL DEFAULT 'low',
		status      TEXT NOT NULL,
		reviewer    TEXT NOT NULL DEFAULT '',
		note        TEXT NOT NULL DEFAULT '',
		edited_args TEXT NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL,
		reviewed_at INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status)`,
	`CREATE INDEX IF NOT EXISTS idx_approvals_conv ON approvals(conv_id)`,
	`CREATE TABLE IF NOT EXISTS traces (
		trace_id   TEXT PRIMARY KEY,
		conv_id    TEXT NOT NULL DEFAULT '',
		goal       TEXT NOT NULL DEFAULT '',
		started_at INTEGER NOT NULL,
		ended_at   INTEGER NOT NULL DEFAULT 0,
		status     TEXT NOT NULL DEFAULT 'running'
	)`,
	`CREATE TABLE IF NOT EXISTS spans (
		span_id    TEXT PRIMARY KEY,
		trace_id   TEXT NOT NULL,
		parent_id  TEXT NOT NULL DEFAULT '',
		kind       TEXT NOT NULL,
		name       TEXT NOT NULL,
		started_at INTEGER NOT NULL,
		ended_at   INTEGER NOT NULL DEFAULT 0,
		attrs      TEXT NOT NULL DEFAULT '{}',
		input      TEXT NOT NULL DEFAULT '',
		output     TEXT NOT NULL DEFAULT '',
		tokens_in  INTEGER NOT NULL DEFAULT 0,
		tokens_out INTEGER NOT NULL DEFAULT 0,
		error      TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_spans_trace ON spans(trace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_traces_started ON traces(started_at)`,
}
