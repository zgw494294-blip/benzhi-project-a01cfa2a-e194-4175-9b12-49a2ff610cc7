package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func TestPayloadStoreValidatesDigestSizeAndDuplicate(t *testing.T) {
	store, err := NewFilePayloadStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("immutable-radiograph-payload")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	key, actual, size, err := store.Put(context.Background(), digest, bytes.NewReader(payload), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if actual != digest || size != int64(len(payload)) {
		t.Fatalf("载荷结果异常: %s %d", actual, size)
	}
	if err := store.Verify(context.Background(), key, digest, size); err != nil {
		t.Fatal(err)
	}
	reader, openedSize, err := store.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, _ := io.ReadAll(reader)
	if openedSize != size || !bytes.Equal(data, payload) {
		t.Fatal("读取载荷与写入内容不一致")
	}
	if _, _, _, err := store.Put(context.Background(), digest, bytes.NewReader(payload), 1024); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("重复载荷应被拒绝: %v", err)
	}
	wrong := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if _, _, _, err := store.Put(context.Background(), wrong, bytes.NewReader([]byte("new")), 1024); !errors.Is(err, domain.ErrIntegrity) {
		t.Fatalf("摘要不一致应被拒绝: %v", err)
	}
}
