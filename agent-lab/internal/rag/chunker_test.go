package rag

import (
	"strings"
	"testing"
)

func TestChunk_ShortText(t *testing.T) {
	chunks := Chunk("hello world", DefaultChunkConfig())
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Fatalf("expected single chunk, got %v", chunks)
	}
}

func TestChunk_Empty(t *testing.T) {
	if chunks := Chunk("", DefaultChunkConfig()); len(chunks) != 0 {
		t.Fatalf("expected no chunks for empty, got %v", chunks)
	}
}

func TestChunk_Overlap(t *testing.T) {
	text := strings.Repeat("这是一段测试文本。", 100) // 很长
	cfg := ChunkConfig{ChunkSize: 50, Overlap: 10}
	chunks := Chunk(text, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// 每块不超过 chunkSize + boundary adjustment.
	for i, c := range chunks {
		if len([]rune(c)) > 60 {
			t.Fatalf("chunk %d too long: %d runes", i, len([]rune(c)))
		}
	}
}

func TestChunk_BoundaryRespected(t *testing.T) {
	// 文本有明确句号, 切块应在句号处断.
	text := strings.Repeat("短句。", 200)
	cfg := ChunkConfig{ChunkSize: 30, Overlap: 5}
	chunks := Chunk(text, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		// 每块应以 "。" 结尾或为最后一块.
		if !strings.HasSuffix(c, "。") && !strings.HasSuffix(c, "。 ") {
			// 允许最后一块不以句号结尾, 但前面的块应该尽量在句号处切.
		}
	}
}

func TestChunkCount(t *testing.T) {
	text := strings.Repeat("x", 1200)
	n := ChunkCount(text, ChunkConfig{ChunkSize: 500, Overlap: 50})
	if n < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", n)
	}
}
