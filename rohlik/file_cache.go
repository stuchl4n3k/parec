package rohlik

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Generic JSON-file cache (type-agnostic).
// Disk format: map[string]json.RawMessage
//
//		{
//	  "<key>": <any JSON value>,
//		  ...
//		}
type FileCache struct {
	path string

	mu   sync.RWMutex
	data map[string]json.RawMessage
}

func NewFileCache(path string) (*FileCache, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("cache path is required")
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir cache dir: %w", err)
		}
	}

	fc := &FileCache{
		path: path,
		data: make(map[string]json.RawMessage, 1024),
	}

	if err := fc.load(); err != nil {
		return nil, err
	}

	// Ensure file exists (optional, but convenient)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := fc.flushLocked(); err != nil {
			return nil, err
		}
	}

	return fc, nil
}

func (fc *FileCache) load() error {
	b, err := os.ReadFile(fc.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read cache json: %w", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("decode cache json: %w", err)
	}
	if m == nil {
		m = make(map[string]json.RawMessage, 1024)
	}

	fc.data = m
	return nil
}

func (fc *FileCache) GetRaw(key string) (json.RawMessage, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	raw, ok := fc.data[key]
	if !ok {
		return nil, false
	}

	// return a copy to prevent external mutation
	cp := append(json.RawMessage(nil), raw...)
	return cp, true
}

func (fc *FileCache) PutRaw(key string, raw json.RawMessage) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("cache key is required")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Avoid duplicate writes
	if _, exists := fc.data[key]; exists {
		return nil
	}

	// store a copy
	fc.data[key] = append(json.RawMessage(nil), raw...)

	// Persist full JSON store atomically
	return fc.flushLocked()
}

func (fc *FileCache) Get(key string, dst any) bool {
	raw, ok := fc.GetRaw(key)
	if !ok {
		return false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false
	}
	return true
}

func (fc *FileCache) Put(key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	return fc.PutRaw(key, raw)
}

func (fc *FileCache) flushLocked() error {
	// fc.mu must be held by caller

	b, err := json.Marshal(fc.data)
	if err != nil {
		return fmt.Errorf("marshal cache json: %w", err)
	}

	dir := filepath.Dir(fc.path)
	base := filepath.Base(fc.path)

	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp cache file: %w", err)
	}

	if err := os.Rename(tmpName, fc.path); err != nil {
		return fmt.Errorf("atomic rename cache file: %w", err)
	}
	return nil
}

func (fc *FileCache) Close() error {
	// No open handles in JSON mode.
	return nil
}
