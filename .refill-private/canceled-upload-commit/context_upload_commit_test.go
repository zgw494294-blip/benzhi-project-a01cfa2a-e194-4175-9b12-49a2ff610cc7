package context_upload_commit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

type cancelingReader struct {
	data   []byte
	cancel context.CancelFunc
	read   bool
}

func (r *cancelingReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	n := copy(p, r.data)
	r.cancel()
	return n, io.EOF
}

func TestCanceledUploadDoesNotCommitPayload(t *testing.T) {
	dataDir := t.TempDir()
	payload := []byte("radiograph-cancel-boundary")
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	payloadStore, err := store.NewFilePayloadStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = payloadStore.Put(ctx, digest, &cancelingReader{data: payload, cancel: cancel}, 1024)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消上传应返回 context.Canceled，得到 %v", err)
	}

	path := filepath.Join(dataDir, "radiographs", digest[:2], digest)
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("取消的上传不应提交载荷，路径=%s statErr=%v", path, statErr)
	}
}
