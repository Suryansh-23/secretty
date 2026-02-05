package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/clipboard"
)

const (
	defaultTimeout = 2 * time.Second
)

var ErrUnsupportedOperation = errors.New("unsupported operation")

type request struct {
	Op string `json:"op"`
	ID int    `json:"id,omitempty"`
}

type response struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error,omitempty"`
	ID       int            `json:"id,omitempty"`
	RuleName string         `json:"rule_name,omitempty"`
	Type     string         `json:"type,omitempty"`
	Label    string         `json:"label,omitempty"`
	Records  []recordOutput `json:"records,omitempty"`
}

// CopyResponse describes the copy-last response.
type CopyResponse struct {
	ID       int
	RuleName string
	Type     string
	Label    string
}

// SecretInfo describes a cached secret for selection.
type SecretInfo struct {
	ID        int
	RuleName  string
	Type      string
	Label     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type recordOutput struct {
	ID        int    `json:"id"`
	RuleName  string `json:"rule_name,omitempty"`
	Type      string `json:"type,omitempty"`
	Label     string `json:"label,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

// Server serves IPC requests for a running session.
type Server struct {
	listener net.Listener
	cache    *cache.Cache
	copyFn   func([]byte) error
}

// StartServer starts a Unix socket server at path.
func StartServer(path string, cache *cache.Cache, copyFn func([]byte) error) (*Server, error) {
	if cache == nil {
		return nil, errors.New("no cache available")
	}
	if copyFn == nil {
		copyFn = func(payload []byte) error {
			return clipboard.CopyBytes(string(clipboard.BackendAuto), payload)
		}
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	server := &Server{listener: listener, cache: cache, copyFn: copyFn}
	go server.serve()
	return server, nil
}

// Close shuts down the server.
func (s *Server) Close() error {
	if s == nil || s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

// TempSocketPath creates a unique socket path under the OS temp dir.
func TempSocketPath() (string, error) {
	dir := os.TempDir()
	if len(dir) > 60 {
		dir = "/tmp"
	}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("secretty-%d-%d.sock", os.Getpid(), time.Now().UnixNano()+int64(i))
		path := filepath.Join(dir, name)
		if len(path) >= 100 {
			if dir != "/tmp" {
				dir = "/tmp"
				continue
			}
			return "", fmt.Errorf("socket path too long")
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
	}
	return "", errors.New("unable to allocate socket path")
}

// CopyLast connects to the server and requests a copy of the last secret.
func CopyLast(socketPath string) (CopyResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, defaultTimeout)
	if err != nil {
		return CopyResponse{}, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return CopyResponse{}, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(request{Op: "copy-last"}); err != nil {
		return CopyResponse{}, err
	}
	var resp response
	if err := dec.Decode(&resp); err != nil {
		return CopyResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return CopyResponse{}, errors.New("copy failed")
		}
		if resp.Error == "unknown operation" {
			return CopyResponse{}, ErrUnsupportedOperation
		}
		return CopyResponse{}, errors.New(resp.Error)
	}
	return CopyResponse{ID: resp.ID, RuleName: resp.RuleName, Type: resp.Type, Label: resp.Label}, nil
}

// CopyByID connects to the server and requests a copy of a specific secret.
func CopyByID(socketPath string, id int) (CopyResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, defaultTimeout)
	if err != nil {
		return CopyResponse{}, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return CopyResponse{}, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(request{Op: "copy-id", ID: id}); err != nil {
		return CopyResponse{}, err
	}
	var resp response
	if err := dec.Decode(&resp); err != nil {
		return CopyResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return CopyResponse{}, errors.New("copy failed")
		}
		if resp.Error == "unknown operation" {
			return CopyResponse{}, ErrUnsupportedOperation
		}
		return CopyResponse{}, errors.New(resp.Error)
	}
	return CopyResponse{ID: resp.ID, RuleName: resp.RuleName, Type: resp.Type, Label: resp.Label}, nil
}

// ListSecrets returns cached secrets for selection.
func ListSecrets(socketPath string) ([]SecretInfo, error) {
	conn, err := net.DialTimeout("unix", socketPath, defaultTimeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return nil, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(request{Op: "list"}); err != nil {
		return nil, err
	}
	var resp response
	if err := dec.Decode(&resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return nil, errors.New("list failed")
		}
		if resp.Error == "unknown operation" {
			return nil, ErrUnsupportedOperation
		}
		return nil, errors.New(resp.Error)
	}
	out := make([]SecretInfo, 0, len(resp.Records))
	for _, rec := range resp.Records {
		info := SecretInfo{
			ID:       rec.ID,
			RuleName: rec.RuleName,
			Type:     rec.Type,
			Label:    rec.Label,
		}
		if rec.CreatedAt > 0 {
			info.CreatedAt = time.Unix(rec.CreatedAt, 0)
		}
		if rec.ExpiresAt > 0 {
			info.ExpiresAt = time.Unix(rec.ExpiresAt, 0)
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return
	}

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	var req request
	if err := dec.Decode(&req); err != nil {
		if err := enc.Encode(response{OK: false, Error: "invalid request"}); err != nil {
			return
		}
		return
	}
	switch req.Op {
	case "copy-last":
		rec, ok := s.cache.GetLast()
		if !ok {
			if err := enc.Encode(response{OK: false, Error: "no secrets cached"}); err != nil {
				return
			}
			return
		}
		if err := s.copyFn(rec.Original); err != nil {
			if err := enc.Encode(response{OK: false, Error: err.Error()}); err != nil {
				return
			}
			return
		}
		if err := enc.Encode(response{OK: true, ID: rec.ID, RuleName: rec.RuleName, Type: string(rec.Type), Label: rec.Label}); err != nil {
			return
		}
	case "copy-id":
		if req.ID == 0 {
			if err := enc.Encode(response{OK: false, Error: "missing id"}); err != nil {
				return
			}
			return
		}
		rec, ok := s.cache.Get(req.ID)
		if !ok {
			if err := enc.Encode(response{OK: false, Error: "secret not found"}); err != nil {
				return
			}
			return
		}
		if err := s.copyFn(rec.Original); err != nil {
			if err := enc.Encode(response{OK: false, Error: err.Error()}); err != nil {
				return
			}
			return
		}
		if err := enc.Encode(response{OK: true, ID: rec.ID, RuleName: rec.RuleName, Type: string(rec.Type), Label: rec.Label}); err != nil {
			return
		}
	case "list":
		records := s.cache.List()
		out := make([]recordOutput, 0, len(records))
		for _, rec := range records {
			item := recordOutput{
				ID:       rec.ID,
				RuleName: rec.RuleName,
				Type:     string(rec.Type),
				Label:    rec.Label,
			}
			if !rec.CreatedAt.IsZero() {
				item.CreatedAt = rec.CreatedAt.Unix()
			}
			if !rec.ExpiresAt.IsZero() {
				item.ExpiresAt = rec.ExpiresAt.Unix()
			}
			out = append(out, item)
		}
		if err := enc.Encode(response{OK: true, Records: out}); err != nil {
			return
		}
	default:
		if err := enc.Encode(response{OK: false, Error: "unknown operation"}); err != nil {
			return
		}
	}
}
