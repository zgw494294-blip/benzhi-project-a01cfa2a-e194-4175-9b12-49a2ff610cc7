package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

type RandomIDGenerator struct{ counter atomic.Uint64 }

func (g *RandomIDGenerator) NewID(prefix string) string {
	var random [6]byte
	if _, err := rand.Read(random[:]); err == nil {
		return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UTC().UnixMilli(), hex.EncodeToString(random[:]))
	}
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UTC().UnixNano(), g.counter.Add(1))
}
