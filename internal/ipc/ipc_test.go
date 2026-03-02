package ipc

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/sessioncontrol"
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
	server, err := StartServer(socketPath, store, func([]byte) error { return nil }, nil)
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
	server, err := StartServer(socketPath, store, func([]byte) error { return nil }, nil)
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

func TestPauseLifecycle(t *testing.T) {
	store := cache.New(10, time.Minute)
	ctrl := sessioncontrol.NewController()

	socketPath, err := TempSocketPath()
	if err != nil {
		t.Fatalf("temp socket: %v", err)
	}
	server, err := StartServer(socketPath, store, func([]byte) error { return nil }, ctrl)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = server.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	st, err := PauseFor(socketPath, 2*time.Second)
	if err != nil {
		t.Fatalf("pause for: %v", err)
	}
	if !st.Active || st.Mode != sessioncontrol.ModeTime || st.RemainingSeconds <= 0 {
		t.Fatalf("unexpected pause status: %+v", st)
	}

	st, err = PauseCommands(socketPath, 3)
	if err != nil {
		t.Fatalf("pause commands: %v", err)
	}
	if !st.Active || st.Mode != sessioncontrol.ModeCommands || st.RemainingCommands != 3 {
		t.Fatalf("unexpected command pause status: %+v", st)
	}

	st, err = PauseResume(socketPath)
	if err != nil {
		t.Fatalf("pause resume: %v", err)
	}
	if st.Active || st.Mode != sessioncontrol.ModeNone {
		t.Fatalf("expected inactive after resume, got %+v", st)
	}
}

func TestPauseUnsupportedOnLegacyServer(t *testing.T) {
	store := cache.New(10, time.Minute)
	socketPath, err := TempSocketPath()
	if err != nil {
		t.Fatalf("temp socket: %v", err)
	}
	server, err := StartServer(socketPath, store, func([]byte) error { return nil }, nil)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = server.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	if _, err := PauseStatusQuery(socketPath); err == nil {
		t.Fatal("expected error")
	} else if errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected explicit unavailable error, got unsupported operation: %v", err)
	}
}

func TestPauseUnknownOperationMapsToUnsupported(t *testing.T) {
	socketPath, err := TempSocketPath()
	if err != nil {
		t.Fatalf("temp socket: %v", err)
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		var req request
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			return
		}
		if err := json.NewEncoder(conn).Encode(response{OK: false, Error: "unknown operation"}); err != nil {
			return
		}
	}()

	if _, err := callPause(socketPath, request{Op: "pause-status"}); !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected ErrUnsupportedOperation, got %v", err)
	}
	<-done
}

func TestPauseWorksWithoutCopyCache(t *testing.T) {
	ctrl := sessioncontrol.NewController()
	socketPath, err := TempSocketPath()
	if err != nil {
		t.Fatalf("temp socket: %v", err)
	}
	server, err := StartServer(socketPath, nil, nil, ctrl)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = server.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	st, err := PauseCommands(socketPath, 2)
	if err != nil {
		t.Fatalf("pause commands: %v", err)
	}
	if !st.Active || st.Mode != sessioncontrol.ModeCommands || st.RemainingCommands != 2 {
		t.Fatalf("unexpected status: %+v", st)
	}

	if _, _, err := FetchLast(socketPath); err == nil {
		t.Fatal("expected copy fetch error without cache")
	}
}
