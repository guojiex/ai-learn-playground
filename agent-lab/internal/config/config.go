// Package config 加载 agent-lab 的运行时配置.
//
// 配置来源固定为环境变量, 后续里程碑会扩展为 env + yaml.
// 当前仅覆盖 M0 所需的字段.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 是 agent-lab 进程级配置.
type Config struct {
	// Profile 决定一组默认值, 取值 S / M / L / XL. 不影响代码路径.
	Profile string

	// BaseURL 指向本地 OpenAI 兼容 server, 例如 http://127.0.0.1:8080/v1.
	BaseURL string
	// APIKey 在本地后端通常是占位字符串.
	APIKey string

	// ModelChat 是默认 chat 模型名, 来自 AGENTLAB_MODEL_CHAT.
	ModelChat string

	// DBPath 是 SQLite 持久层文件路径 (M4 起), 来自 AGENTLAB_DB_PATH.
	// 默认 agent-lab/data/agent.db. 为 ":memory:" 时走纯内存库.
	DBPath string

	// HTTP 超时与重试.
	RequestTimeout time.Duration
	MaxRetries     int
}

// Load 从环境变量读出配置. 缺失字段按 profile 默认值填充.
func Load() (Config, error) {
	cfg := Config{
		Profile:        getenv("AGENTLAB_PROFILE", "L"),
		BaseURL:        getenv("OPENAI_BASE_URL", ""),
		APIKey:         getenv("OPENAI_API_KEY", "sk-local"),
		ModelChat:      getenv("AGENTLAB_MODEL_CHAT", ""),
		DBPath:         getenv("AGENTLAB_DB_PATH", "agent-lab/data/agent.db"),
		RequestTimeout: getenvDuration("AGENTLAB_REQUEST_TIMEOUT", 120*time.Second),
		MaxRetries:     getenvInt("AGENTLAB_MAX_RETRIES", 3),
	}
	cfg.applyProfileDefaults()
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyProfileDefaults() {
	if c.ModelChat != "" {
		return
	}
	switch c.Profile {
	case "S":
		c.ModelChat = "qwen1.5-1.8b-chat"
	case "M":
		c.ModelChat = "qwen2.5-3b-instruct"
	case "XL":
		c.ModelChat = "qwen2.5-14b-instruct"
	default: // L 与未知值都落到 7B
		c.ModelChat = "qwen2.5-7b-instruct"
	}
}

func (c Config) validate() error {
	if c.BaseURL == "" {
		return errors.New("OPENAI_BASE_URL is required (e.g. http://127.0.0.1:8080/v1)")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("AGENTLAB_REQUEST_TIMEOUT must be positive")
	}
	if c.MaxRetries < 0 {
		return errors.New("AGENTLAB_MAX_RETRIES must be >= 0")
	}
	return nil
}

// String 返回可打印的简要摘要 (不含 APIKey).
func (c Config) String() string {
	return fmt.Sprintf("profile=%s base_url=%s model=%s timeout=%s retries=%d",
		c.Profile, c.BaseURL, c.ModelChat, c.RequestTimeout, c.MaxRetries)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
