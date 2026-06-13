// Command fake-openai 启动一个最小的 OpenAI 兼容 fake server,
// 仅用于在没有本地大模型时跑通 cmd/chat 的端到端流程.
//
// 用法:
//   go run ./agent-lab/scripts/fake-openai
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type chatReq struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req chatReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var lastUser string
		for _, m := range req.Messages {
			if m.Role == "user" {
				lastUser = m.Content
			}
		}
		reply := "echo: " + lastUser
		if req.Stream {
			streamReply(w, reply)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": req.Model,
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": reply},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{
				"prompt_tokens":     len(lastUser),
				"completion_tokens": len(reply),
				"total_tokens":      len(lastUser) + len(reply),
			},
		})
	})

	addr := "127.0.0.1:18080"
	log.Printf("fake-openai listening on http://%s/v1", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func streamReply(w http.ResponseWriter, reply string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(payload string) {
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	for _, ch := range strings.Split(reply, "") {
		chunk, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{"role": "assistant", "content": ch},
				"finish_reason": "",
			}},
		})
		send(string(chunk))
		time.Sleep(15 * time.Millisecond)
	}
	final, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{"prompt_tokens": 8, "completion_tokens": len(reply), "total_tokens": 8 + len(reply)},
	})
	send(string(final))
	send("[DONE]")
}
