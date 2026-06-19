// embed.go 是 fake-openai 的 embedding 模拟器.
//
// 用途: 让 M5 RAG 在没有真实 embedding 模型时也能跑通.
// 策略: 对文本做字符 bigram 哈希, 投影到固定维度 (128), 再 L2 归一化.
// 这保证: 相同文本 → 相同向量; 共享 bigram 的文本 → 高余弦相似度, 足以演示 top-k 检索.
package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"unicode"
)

const fakeEmbedDim = 128

type embedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

func handleEmbed(w http.ResponseWriter, r *http.Request) {
	var req embedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data := make([]embedData, len(req.Input))
	for i, text := range req.Input {
		data[i] = embedData{Index: i, Embedding: fakeEmbed(text)}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model": req.Model,
		"data":  data,
		"usage": map[string]int{"prompt_tokens": 0, "total_tokens": 0},
	})
}

// fakeEmbed 把文本编码成 fakeEmbedDim 维的归一化向量.
func fakeEmbed(text string) []float32 {
	vec := make([]float32, fakeEmbedDim)
	if text == "" {
		vec[0] = 1
		return vec
	}
	// 字符 bigram: 把相邻两个字符作为一个 token, FNV 哈希到某个维度.
	runes := []rune(normalizeText(text))
	for i := 0; i < len(runes)-1; i++ {
		bigram := string(runes[i]) + string(runes[i+1])
		h := fnv.New32a()
		h.Write([]byte(bigram))
		idx := int(h.Sum32()) % fakeEmbedDim
		vec[idx] += 1
	}
	// 单字符也加一轮 (捕获短词).
	for _, r := range runes {
		h := fnv.New32a()
		h.Write([]byte(string(r)))
		idx := int(h.Sum32()) % fakeEmbedDim
		vec[idx] += 0.5
	}
	// L2 归一化, 使余弦相似度 = 点积.
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}
	return vec
}

// normalizeText 统一大小写 + 去标点, 让 "Shopee" 和 "shopee" 编码一致.
func normalizeText(s string) string {
	var b []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b = append(b, unicode.ToLower(r))
		} else {
			b = append(b, ' ')
		}
	}
	return string(b)
}

var _ = fmt.Sprintf
