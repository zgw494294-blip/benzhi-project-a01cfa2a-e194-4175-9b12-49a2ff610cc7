package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

type FilePayloadStore struct{ root string }

func NewFilePayloadStore(dataDir string) (*FilePayloadStore, error) {
	root := filepath.Join(dataDir, "radiographs")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &FilePayloadStore{root: root}, nil
}

func (s *FilePayloadStore) Put(ctx context.Context, expectedDigest string, source io.Reader, maxBytes int64) (string, string, int64, error) {
	expectedDigest = strings.ToLower(strings.TrimSpace(expectedDigest))
	if len(expectedDigest) != 64 {
		return "", "", 0, domain.Invalid("contentDigest", "必须提供 SHA-256 摘要")
	}
	temp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return "", "", 0, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	hash := sha256.New()
	limited := io.LimitReader(source, maxBytes+1)
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), limited)
	closeErr := temp.Close()
	if copyErr != nil {
		return "", "", 0, copyErr
	}
	if closeErr != nil {
		return "", "", 0, closeErr
	}
	if err := ctx.Err(); err != nil {
		return "", "", 0, err
	}
	if written == 0 {
		return "", "", 0, domain.Invalid("payload", "底片载荷不能为空")
	}
	if written > maxBytes {
		return "", "", 0, domain.Invalid("payload", "底片载荷超过 32 MiB 限制")
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if digest != expectedDigest {
		return "", "", 0, fmt.Errorf("%w: 声明摘要与载荷不一致", domain.ErrIntegrity)
	}
	directory := filepath.Join(s.root, digest[:2])
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", "", 0, err
	}
	key := filepath.Join(digest[:2], digest)
	destination := filepath.Join(s.root, key)
	if _, err := os.Stat(destination); err == nil {
		return "", "", 0, domain.ErrDuplicate
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", 0, err
	}
	if err := os.Rename(tempName, destination); err != nil {
		return "", "", 0, err
	}
	if err := os.Chmod(destination, 0o640); err != nil {
		return "", "", 0, err
	}
	return filepath.ToSlash(key), digest, written, nil
}

func (s *FilePayloadStore) Open(ctx context.Context, storageKey string) (io.ReadCloser, int64, error) {
	path, err := s.safePath(storageKey)
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, domain.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, info.Size(), nil
}

func (s *FilePayloadStore) Verify(ctx context.Context, storageKey, digest string, size int64) error {
	reader, actualSize, err := s.Open(ctx, storageKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	if actualSize != size {
		return fmt.Errorf("载荷大小不一致")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("载荷摘要不一致")
	}
	return nil
}

func (s *FilePayloadStore) safePath(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", domain.Invalid("storageKey", "存储键无效")
	}
	return filepath.Join(s.root, clean), nil
}
