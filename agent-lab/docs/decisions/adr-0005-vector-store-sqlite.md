# ADR-0005 · 向量存储用 SQLite (sqlite-vec)

- 状态：Accepted
- 日期：2026-06-13

## 背景

M5 (RAG) 与 M4 (长期记忆) 都需要向量检索。候选包括内存 / `sqlite-vec` / `chromem-go` / `pgvector` / `Milvus` 等。

## 决策

- **M5 起步**：纯内存暴力检索 (cosine + top-k)，零依赖。
- **M5 中段切换**：`sqlite-vec` (SQLite 扩展)，与 SQLite 主库同文件 (`agent.db`)。
- **不采用 (此阶段)**：`pgvector` / `Milvus` / `Qdrant`。

## 理由

- **零外部依赖**：本项目所有持久化都在一个 `agent.db` 里 (会话、approval、trace、向量)，便于备份与回放。
- **学习成本低**：sqlite-vec 是一个 SQL 扩展，无需额外服务进程，与 Go 的 `database/sql` 无缝。
- **演进路径清晰**：先在 `VectorStore` 接口下做内存版，再做 sqlite-vec 实现，调用方代码零改动。
- **生产替代留 hook**：`VectorStore` 接口刻意设计成与 pgvector / Qdrant API 类似，未来需要规模化时切实现即可。

## 反方意见

- `chromem-go` 是纯 Go 嵌入式库，零 SQL 扩展。 → 选 sqlite-vec 是因为持久化与其它状态共库；纯 Go 库还得自己解决持久化。
- 数据量上去后 sqlite-vec 性能不如专用向量库。 → 学习项目数据量在万级以下，sqlite-vec 完全够；超出时切实现。
- Windows 上加载 SQLite 扩展需要确认驱动支持 (`mattn/go-sqlite3` 默认禁用 load_extension)。 → 选用 `modernc.org/sqlite` (纯 Go) 或自行启用扩展加载，详见 M5 文档。

## 影响

- M4 SQLite 主库与 M5 向量库同文件，schema 共存。
- 单测/Demo 默认走内存版，CI 与日常开发不强制装扩展。
- M9 trace 也落同一 SQLite 文件，统一查询入口 (`cmd/trace`)。
