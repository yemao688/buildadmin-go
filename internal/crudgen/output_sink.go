package crudgen

import (
	"sort"
	"sync"
)

// OutputSink 接收生成器本应写盘的文件内容。默认 nil 表示直接写盘；预览与
// 定制点校验时换成内存实现，从而拿到"如果生成会写出什么"。
type OutputSink interface {
	WriteFile(path, content string) error
}

// MemorySink 把产物收进内存，供 diff、定制点校验与离线预览使用。
type MemorySink struct {
	mu    sync.Mutex
	files map[string]string
}

// NewMemorySink 创建一个空的内存 sink。
func NewMemorySink() *MemorySink {
	return &MemorySink{files: make(map[string]string)}
}

// WriteFile 记录一次产物写入；同一路径多次写入以最后一次为准（与真实生成
// 的最终状态一致）。
func (s *MemorySink) WriteFile(path, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = content
	return nil
}

// Files 返回产物快照（路径 → 内容）。
func (s *MemorySink) Files() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make(map[string]string, len(s.files))
	for path, content := range s.files {
		copied[path] = content
	}
	return copied
}

// Paths 返回排序后的产物路径，保证输出稳定。
func (s *MemorySink) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths := make([]string, 0, len(s.files))
	for path := range s.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
