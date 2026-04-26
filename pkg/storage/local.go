package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// localFileStorage menyimpan file ke disk lokal dan serve via HTTP.
// Gunakan hanya untuk development — tidak cocok untuk multi-instance production.
type localFileStorage struct {
	dir     string // direktori penyimpanan, e.g. "./uploads"
	baseURL string // URL publik backend, e.g. "http://localhost:8080"
}

// NewLocalStorage membuat storage berbasis filesystem lokal.
//   - dir: folder tempat file disimpan (akan dibuat otomatis jika belum ada)
//   - baseURL: URL publik backend tanpa trailing slash
func NewLocalStorage(dir, baseURL string) (Storage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload dir: %w", err)
	}
	return &localFileStorage{dir: dir, baseURL: baseURL}, nil
}

func (l *localFileStorage) Upload(_ context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	dest := filepath.Join(l.dir, filepath.FromSlash(key))

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("failed to create dir: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return l.GetURL(key), nil
}

func (l *localFileStorage) Delete(_ context.Context, key string) error {
	path := filepath.Join(l.dir, filepath.FromSlash(key))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (l *localFileStorage) GetURL(key string) string {
	return fmt.Sprintf("%s/uploads/%s", l.baseURL, key)
}
