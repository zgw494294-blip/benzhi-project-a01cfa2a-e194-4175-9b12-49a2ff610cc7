package concurrent_payload_verification_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
)

func TestConcurrentPayloadVerificationIsolated(t *testing.T) {
	payload := bytes.Repeat([]byte("radiograph-frame-"), 1<<19)
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	payloadStore, err := store.NewFilePayloadStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storageKey, _, size, err := payloadStore.Put(context.Background(), digest, bytes.NewReader(payload), int64(len(payload)+1))
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	start := make(chan struct{})
	results := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer group.Done()
			<-start
			results <- payloadStore.Verify(context.Background(), storageKey, digest, size)
		}()
	}
	close(start)
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("并发核验不应因共享摘要状态失败: %v", err)
		}
	}
}
