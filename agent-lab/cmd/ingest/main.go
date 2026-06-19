// Command ingest 把文档 (Markdown/纯文本) 切块、embed、写入向量库 (M5).
//
// 用法:
//
//	# 先启动 embedding 后端 (或用 fake-openai)
//	export OPENAI_BASE_URL=http://127.0.0.1:18080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_DB_PATH=agent-lab/data/agent.db
//
//	# 导入 platform_rules 目录下所有 .md
//	go run ./agent-lab/cmd/ingest -dir agent-lab/data/platform_rules
//
//	# 导入单个文件并指定 source 名
//	go run ./agent-lab/cmd/ingest -file path/to/rules.md -source my_rules
//
//	# 查看当前向量库统计
//	go run ./agent-lab/cmd/ingest -list
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/rag"
	"ai-learn-playground/agent-lab/internal/store"
)

func main() {
	var (
		dir     string
		file    string
		source  string
		list    bool
		delSrc  string
		chunkSz int
		overlap int
		batch   int
	)
	flag.StringVar(&dir, "dir", "", "导入此目录下所有 .md/.txt 文件")
	flag.StringVar(&file, "file", "", "导入单个文件")
	flag.StringVar(&source, "source", "", "source 名 (默认用文件名不含扩展名); -dir 模式下每文件各自用文件名")
	flag.BoolVar(&list, "list", false, "只列出当前向量库统计")
	flag.StringVar(&delSrc, "delete", "", "删除指定 source 的全部文档块")
	flag.IntVar(&chunkSz, "chunk-size", 500, "每块目标字符数 (rune)")
	flag.IntVar(&overlap, "overlap", 50, "相邻块重叠字符数")
	flag.IntVar(&batch, "batch", 16, "embedding 批量大小")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open store:", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx := context.Background()

	if list {
		sources, err := st.ListDocSources(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list sources:", err)
			os.Exit(1)
		}
		total := 0
		fmt.Printf("%-40s %s\n", "SOURCE", "CHUNKS")
		fmt.Println(strings.Repeat("-", 50))
		for _, s := range sources {
			fmt.Printf("%-40s %d\n", s.Source, s.Chunks)
			total += s.Chunks
		}
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("%-40s %d\n", "TOTAL", total)
		return
	}

	if delSrc != "" {
		vs, err := memory.NewVectorStore(st)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vector store:", err)
			os.Exit(1)
		}
		if err := vs.DeleteBySource(ctx, delSrc); err != nil {
			fmt.Fprintln(os.Stderr, "delete:", err)
			os.Exit(1)
		}
		fmt.Printf("deleted source=%s (remaining chunks=%d)\n", delSrc, vs.Count())
		return
	}

	if dir == "" && file == "" {
		fmt.Fprintln(os.Stderr, "用法: -dir <目录> | -file <文件> | -list | -delete <source>")
		os.Exit(1)
	}

	// 构造 embedder.
	embedder := llm.NewOpenAIEmbedder(cfg.EmbedBaseURL, cfg.EmbedAPIKey, cfg.ModelEmbed, cfg.RequestTimeout, 0)

	// 先 embed 一条探测维度.
	probe, err := embedder.Embed(ctx, []string{"probe"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "embed probe (检查 embedding 后端是否可用):", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[ingest] embedder ok: model=%s dim=%d\n", cfg.ModelEmbed, len(probe[0]))

	vs, err := memory.NewVectorStore(st)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vector store:", err)
		os.Exit(1)
	}

	cfg2 := rag.ChunkConfig{ChunkSize: chunkSz, Overlap: overlap}

	files := collectFiles(dir, file)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "没有找到可导入的文件")
		os.Exit(1)
	}

	totalChunks := 0
	for _, f := range files {
		src := source
		if src == "" {
			src = strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		}
		n, err := ingestFile(ctx, vs, embedder, f, src, cfg2, batch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ingest] %s: %v\n", f, err)
			continue
		}
		fmt.Printf("[ingest] %s → source=%s chunks=%d\n", f, src, n)
		totalChunks += n
	}
	fmt.Printf("\n完成: %d 个文件, %d 个文档块, 向量库总计 %d 块\n", len(files), totalChunks, vs.Count())
}

func collectFiles(dir, file string) []string {
	if file != "" {
		return []string{file}
	}
	if dir == "" {
		return nil
	}
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".txt" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func ingestFile(ctx context.Context, vs *memory.VectorStore, emb llm.Embedder, path, source string, cfg rag.ChunkConfig, batchSize int) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	chunks := rag.Chunk(string(raw), cfg)
	if len(chunks) == 0 {
		return 0, nil
	}
	// 先删旧 (同 source), 保证幂等.
	if err := vs.DeleteBySource(ctx, source); err != nil {
		return 0, fmt.Errorf("clear old: %w", err)
	}

	total := 0
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		vecs, err := emb.Embed(ctx, batch)
		if err != nil {
			return total, fmt.Errorf("embed batch %d: %w", i, err)
		}
		for j, text := range batch {
			idx := i + j
			id := fmt.Sprintf("%s#%04d", source, idx)
			meta, _ := json.Marshal(map[string]any{
				"source":      source,
				"chunk_index": idx,
				"file":        filepath.Base(path),
				"ingested_at": time.Now().Format(time.RFC3339),
			})
			if err := vs.Add(ctx, id, source, idx, text, vecs[j], string(meta)); err != nil {
				return total, fmt.Errorf("add chunk %d: %w", idx, err)
			}
			total++
		}
		fmt.Fprintf(os.Stderr, "\r[ingest] %s: %d/%d chunks", source, total, len(chunks))
	}
	fmt.Fprintln(os.Stderr)
	return total, nil
}
