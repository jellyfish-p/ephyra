package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

type Store interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
}

type Memory struct {
	mu   sync.Mutex
	data map[string]memObj
}

type memObj struct {
	b           []byte
	contentType string
}

func NewMemory() *Memory {
	return &Memory{data: map[string]memObj{}}
}

func (m *Memory) Put(_ context.Context, key string, r io.Reader, size int64, contentType string) error {
	b, err := io.ReadAll(io.LimitReader(r, size+1))
	if err != nil {
		return err
	}
	if size >= 0 && int64(len(b)) != size {
		return fmt.Errorf("object size mismatch")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = memObj{b: b, contentType: contentType}
	return nil
}

func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, int64, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.data[key]
	if !ok {
		return nil, 0, "", fmt.Errorf("object not found")
	}
	return io.NopCloser(bytes.NewReader(o.b)), int64(len(o.b)), o.contentType, nil
}
