// Package assets 负责解析 Vite build 产生的 manifest.json，并把入口 JS/CSS
// 暴露给 HTTP handler 以便注入到 studio.html。
package assets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Entry 描述 Vite manifest 里一个 entry 点的静态资源路径。
//
//	{
//	  "src/main.tsx": {
//	    "file": "assets/main-abc.js",
//	    "css": ["assets/main-xyz.css"]
//	  }
//	}
type Entry struct {
	File string
	CSS  []string
}

// Manifest 缓存解析后的 manifest。线程安全。
type Manifest struct {
	mu      sync.RWMutex
	distDir string
	entry   string
	cached  *Entry
}

// NewManifest 构造一个 Manifest；distDir 为 `web/static/dist` 绝对路径，
// entry 为源文件相对路径（例如 `src/main.tsx`，与 vite.config 中 rollup 入口一致）。
func NewManifest(distDir, entry string) *Manifest {
	return &Manifest{distDir: distDir, entry: entry}
}

// Reload 强制重新读取 manifest.json。manifest 不存在返回明确错误。
func (m *Manifest) Reload() (*Entry, error) {
	path := filepath.Join(m.distDir, ".vite", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"vite manifest not found at %s: please run `npm run build` under web-studio first",
				path,
			)
		}
		return nil, fmt.Errorf("read vite manifest: %w", err)
	}
	var raw map[string]struct {
		File string   `json:"file"`
		CSS  []string `json:"css"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse vite manifest: %w", err)
	}
	entry, ok := raw[m.entry]
	if !ok || entry.File == "" {
		return nil, fmt.Errorf("vite manifest missing entry %q (got %d entries)", m.entry, len(raw))
	}
	result := &Entry{
		File: ensureLeadingSlash(entry.File),
		CSS:  make([]string, 0, len(entry.CSS)),
	}
	for _, css := range entry.CSS {
		result.CSS = append(result.CSS, ensureLeadingSlash(css))
	}
	m.mu.Lock()
	m.cached = result
	m.mu.Unlock()
	return result, nil
}

// Resolve 返回当前缓存，如未加载则触发一次 Reload。
func (m *Manifest) Resolve() (*Entry, error) {
	m.mu.RLock()
	if m.cached != nil {
		defer m.mu.RUnlock()
		return m.cached, nil
	}
	m.mu.RUnlock()
	return m.Reload()
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
