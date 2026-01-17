package cache

import (
	"testing"
	"time"

	"github.com/suryansh-23/secretty/internal/types"
)

func TestCachePutGetLast(t *testing.T) {
	c := New(2, 5*time.Second)
	c.now = func() time.Time { return time.Unix(100, 0) }

	c.Put(SecretRecord{ID: 1, Type: types.SecretEvmPrivateKey, Original: []byte("a")})
	c.Put(SecretRecord{ID: 2, Type: types.SecretEvmPrivateKey, Original: []byte("b")})

	rec, ok := c.GetLast()
	if !ok {
		t.Fatalf("expected record")
	}
	if string(rec.Original) != "b" {
		t.Fatalf("last = %q", string(rec.Original))
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := New(2, 1*time.Second)
	base := time.Unix(100, 0)
	c.now = func() time.Time { return base }

	c.Put(SecretRecord{ID: 1, Type: types.SecretEvmPrivateKey, Original: []byte("a")})

	c.now = func() time.Time { return base.Add(2 * time.Second) }
	if _, ok := c.GetLast(); ok {
		t.Fatalf("expected expired record")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := New(1, 5*time.Second)
	c.now = func() time.Time { return time.Unix(100, 0) }

	c.Put(SecretRecord{ID: 1, Type: types.SecretEvmPrivateKey, Original: []byte("a")})
	c.Put(SecretRecord{ID: 2, Type: types.SecretEvmPrivateKey, Original: []byte("b")})

	if _, ok := c.Get(1); ok {
		t.Fatalf("expected record 1 to be evicted")
	}
	if _, ok := c.Get(2); !ok {
		t.Fatalf("expected record 2 to remain")
	}
}
