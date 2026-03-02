package ipc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/clipboard"
	"github.com/suryansh-23/secretty/internal/sessioncontrol"
)

const (
	defaultTimeout = 2 * time.Second
)

var ErrUnsupportedOperation = errors.New("unsupported operation")

type request struct {
	Op       string `json:"op"`
	ID       int    `json:"id,omitempty"`
	Seconds  int64  `json:"seconds,omitempty"`
	Commands int    `json:"commands,omitempty"`
}

type response struct {
	OK                bool           `json:"ok"`
	Error             string         `json:"error,omitempty"`
	ID                int            `json:"id,omitempty"`
	RuleName          string         `json:"rule_name,omitempty"`
	Type              string         `json:"type,omitempty"`
	Label             string         `json:"label,omitempty"`
	Records           []recordOutput `json:"records,omitempty"`
	Payload           string         `json:"payload,omitempty"`
	PauseActive       bool           `json:"pause_active,omitempty"`
	PauseMode         string         `json:"pause_mode,omitempty"`
	RemainingSeconds  int64          `json:"pause_remaining_seconds,omitempty"`
	RemainingCommands int            `json:"pause_remaining_commands,omitempty"`
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

// PauseStatus describes the session-level redaction pause state.
type PauseStatus struct {
	Active            bool
	Mode              sessioncontrol.Mode
	RemainingSeconds  int64
	RemainingCommands int
}

// Server serves IPC requests for a running session.
type Server struct {
	listener net.Listener
	cache    *cache.Cache
	copyFn   func([]byte) error
	pause    *sessioncontrol.Controller
}

// StartServer starts a Unix socket server at path.
func StartServer(path string, cache *cache.Cache, copyFn func([]byte) error, pause *sessioncontrol.Controller) (*Server, error) {
	if cache == nil && pause == nil {
		return nil, errors.New("no ipc handlers available")
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
	server := &Server{listener: listener, cache: cache, copyFn: copyFn, pause: pause}
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

// FetchLast connects to the server and requests the last secret payload.
func FetchLast(socketPath string) ([]byte, CopyResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, defaultTimeout)
	if err != nil {
		return nil, CopyResponse{}, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return nil, CopyResponse{}, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(request{Op: "fetch-last"}); err != nil {
		return nil, CopyResponse{}, err
	}
	var resp response
	if err := dec.Decode(&resp); err != nil {
		return nil, CopyResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return nil, CopyResponse{}, errors.New("copy failed")
		}
		if resp.Error == "unknown operation" {
			return nil, CopyResponse{}, ErrUnsupportedOperation
		}
		return nil, CopyResponse{}, errors.New(resp.Error)
	}
	payload, err := base64.StdEncoding.DecodeString(resp.Payload)
	if err != nil {
		return nil, CopyResponse{}, fmt.Errorf("decode payload: %w", err)
	}
	return payload, CopyResponse{ID: resp.ID, RuleName: resp.RuleName, Type: resp.Type, Label: resp.Label}, nil
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

// FetchByID connects to the server and requests a secret payload by ID.
func FetchByID(socketPath string, id int) ([]byte, CopyResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, defaultTimeout)
	if err != nil {
		return nil, CopyResponse{}, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return nil, CopyResponse{}, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(request{Op: "fetch-id", ID: id}); err != nil {
		return nil, CopyResponse{}, err
	}
	var resp response
	if err := dec.Decode(&resp); err != nil {
		return nil, CopyResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return nil, CopyResponse{}, errors.New("copy failed")
		}
		if resp.Error == "unknown operation" {
			return nil, CopyResponse{}, ErrUnsupportedOperation
		}
		return nil, CopyResponse{}, errors.New(resp.Error)
	}
	payload, err := base64.StdEncoding.DecodeString(resp.Payload)
	if err != nil {
		return nil, CopyResponse{}, fmt.Errorf("decode payload: %w", err)
	}
	return payload, CopyResponse{ID: resp.ID, RuleName: resp.RuleName, Type: resp.Type, Label: resp.Label}, nil
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

// PauseFor pauses redaction for a time duration in the active wrapped session.
func PauseFor(socketPath string, d time.Duration) (PauseStatus, error) {
	if d <= 0 {
		return PauseStatus{}, errors.New("duration must be greater than zero")
	}
	seconds := int64((d + time.Second - 1) / time.Second)
	return callPause(socketPath, request{Op: "pause-for", Seconds: seconds})
}

// PauseCommands pauses redaction for the next n entered command lines.
func PauseCommands(socketPath string, n int) (PauseStatus, error) {
	if n <= 0 {
		return PauseStatus{}, errors.New("commands must be greater than zero")
	}
	return callPause(socketPath, request{Op: "pause-commands", Commands: n})
}

// PauseStatusQuery returns the current redaction pause state.
func PauseStatusQuery(socketPath string) (PauseStatus, error) {
	return callPause(socketPath, request{Op: "pause-status"})
}

// PauseResume clears any active redaction pause.
func PauseResume(socketPath string) (PauseStatus, error) {
	return callPause(socketPath, request{Op: "pause-resume"})
}

func callPause(socketPath string, req request) (PauseStatus, error) {
	conn, err := net.DialTimeout("unix", socketPath, defaultTimeout)
	if err != nil {
		return PauseStatus{}, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return PauseStatus{}, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(req); err != nil {
		return PauseStatus{}, err
	}

	var resp response
	if err := dec.Decode(&resp); err != nil {
		return PauseStatus{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return PauseStatus{}, errors.New("pause operation failed")
		}
		if strings.EqualFold(resp.Error, "unknown operation") {
			return PauseStatus{}, ErrUnsupportedOperation
		}
		return PauseStatus{}, errors.New(resp.Error)
	}

	mode := sessioncontrol.Mode(resp.PauseMode)
	if mode == "" {
		mode = sessioncontrol.ModeNone
	}
	return PauseStatus{
		Active:            resp.PauseActive,
		Mode:              mode,
		RemainingSeconds:  resp.RemainingSeconds,
		RemainingCommands: resp.RemainingCommands,
	}, nil
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
	case "fetch-last":
		if s.cache == nil {
			if err := enc.Encode(response{OK: false, Error: "copy cache unavailable"}); err != nil {
				return
			}
			return
		}
		rec, ok := s.cache.GetLast()
		if !ok {
			if err := enc.Encode(response{OK: false, Error: "no secrets cached"}); err != nil {
				return
			}
			return
		}
		payload := base64.StdEncoding.EncodeToString(rec.Original)
		if err := enc.Encode(response{OK: true, ID: rec.ID, RuleName: rec.RuleName, Type: string(rec.Type), Label: rec.Label, Payload: payload}); err != nil {
			return
		}
	case "fetch-id":
		if s.cache == nil {
			if err := enc.Encode(response{OK: false, Error: "copy cache unavailable"}); err != nil {
				return
			}
			return
		}
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
		payload := base64.StdEncoding.EncodeToString(rec.Original)
		if err := enc.Encode(response{OK: true, ID: rec.ID, RuleName: rec.RuleName, Type: string(rec.Type), Label: rec.Label, Payload: payload}); err != nil {
			return
		}
	case "copy-last":
		if s.cache == nil {
			if err := enc.Encode(response{OK: false, Error: "copy cache unavailable"}); err != nil {
				return
			}
			return
		}
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
		if s.cache == nil {
			if err := enc.Encode(response{OK: false, Error: "copy cache unavailable"}); err != nil {
				return
			}
			return
		}
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
		if s.cache == nil {
			if err := enc.Encode(response{OK: false, Error: "copy cache unavailable"}); err != nil {
				return
			}
			return
		}
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
	case "pause-for":
		if s.pause == nil {
			if err := enc.Encode(response{OK: false, Error: "pause unavailable in this session"}); err != nil {
				return
			}
			return
		}
		if req.Seconds <= 0 {
			if err := enc.Encode(response{OK: false, Error: "invalid seconds"}); err != nil {
				return
			}
			return
		}
		s.pause.PauseFor(time.Duration(req.Seconds) * time.Second)
		if err := enc.Encode(statusResponse(s.pause.Status())); err != nil {
			return
		}
	case "pause-commands":
		if s.pause == nil {
			if err := enc.Encode(response{OK: false, Error: "pause unavailable in this session"}); err != nil {
				return
			}
			return
		}
		if req.Commands <= 0 {
			if err := enc.Encode(response{OK: false, Error: "invalid commands"}); err != nil {
				return
			}
			return
		}
		s.pause.PauseCommands(req.Commands)
		if err := enc.Encode(statusResponse(s.pause.Status())); err != nil {
			return
		}
	case "pause-status":
		if s.pause == nil {
			if err := enc.Encode(response{OK: false, Error: "pause unavailable in this session"}); err != nil {
				return
			}
			return
		}
		if err := enc.Encode(statusResponse(s.pause.Status())); err != nil {
			return
		}
	case "pause-resume":
		if s.pause == nil {
			if err := enc.Encode(response{OK: false, Error: "pause unavailable in this session"}); err != nil {
				return
			}
			return
		}
		s.pause.Resume()
		if err := enc.Encode(statusResponse(s.pause.Status())); err != nil {
			return
		}
	default:
		if err := enc.Encode(response{OK: false, Error: "unknown operation"}); err != nil {
			return
		}
	}
}

func statusResponse(st sessioncontrol.Status) response {
	resp := response{
		OK:          true,
		PauseActive: st.Active,
		PauseMode:   string(st.Mode),
	}
	switch st.Mode {
	case sessioncontrol.ModeTime:
		if !st.Until.IsZero() {
			remaining := int64(time.Until(st.Until).Seconds())
			if remaining < 0 {
				remaining = 0
			}
			resp.RemainingSeconds = remaining
		}
	case sessioncontrol.ModeCommands:
		resp.RemainingCommands = st.RemainingCommands
	}
	return resp
}
