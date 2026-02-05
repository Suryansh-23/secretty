package ipc

import (
	"os"
	"testing"
	"time"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/types"
)

func TestFetchLast(t *testing.T) {
	store := cache.New(10, time.Minute)
	store.Put(cache.SecretRecord{
		ID:       1,
		Type:     types.SecretEvmPrivateKey,
		RuleName: "env_private_key",
		Label:    "PRIVATE_KEY",
		Original: []byte("secret"),
	})

	socketPath, err := TempSocketPath()
	if err != nil {
		t.Fatalf("temp socket: %v", err)
	}
	server, err := StartServer(socketPath, store, func([]byte) error { return nil })
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = server.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	payload, resp, err := FetchLast(socketPath)
	if err != nil {
		t.Fatalf("fetch last: %v", err)
	}
	if string(payload) != "secret" {
		t.Fatalf("payload = %q", string(payload))
	}
	if resp.ID != 1 {
		t.Fatalf("id = %d", resp.ID)
	}
}

func TestFetchByID(t *testing.T) {
	store := cache.New(10, time.Minute)
	store.Put(cache.SecretRecord{
		ID:       7,
		Type:     types.SecretEvmPrivateKey,
		RuleName: "env_private_key",
		Label:    "PRIVATE_KEY",
		Original: []byte("secret"),
	})

	socketPath, err := TempSocketPath()
	if err != nil {
		t.Fatalf("temp socket: %v", err)
	}
	server, err := StartServer(socketPath, store, func([]byte) error { return nil })
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = server.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	payload, resp, err := FetchByID(socketPath, 7)
	if err != nil {
		t.Fatalf("fetch by id: %v", err)
	}
	if string(payload) != "secret" {
		t.Fatalf("payload = %q", string(payload))
	}
	if resp.ID != 7 {
		t.Fatalf("id = %d", resp.ID)
	}
}
