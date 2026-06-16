package ptywrap

import (
	"bytes"
	"net/url"
	"os"
	"strings"

	"github.com/suryansh-23/secretty/internal/debug"
)

// OSC 7 working-directory report markers.
var (
	osc7Prefix = []byte("\x1b]7;") // ESC ] 7 ;
	oscST      = []byte("\x1b\\")  // String Terminator (ESC \)
)

// cwdWatchMaxBuffer caps the partial-sequence buffer so a stream that never
// contains a terminator can't grow memory without bound.
const cwdWatchMaxBuffer = 1 << 16 // 64 KiB

// cwdSyncDisabled reports whether OSC 7 cwd syncing has been turned off via env.
func cwdSyncDisabled() bool {
	v := os.Getenv("SECRETTY_DISABLE_CWD_SYNC")
	return v == "1" || strings.EqualFold(v, "true")
}

// newCwdSyncObserver returns an output observer that watches the child's output
// stream for OSC 7 working-directory reports — ESC ] 7 ; file://host/path (BEL|ST)
// — and chdir()s the secretty process to the reported directory.
//
// Why: secretty runs the shell under its own PTY and proxies IO, so secretty is
// the process a parent terminal or multiplexer reads the working directory FROM.
// secretty never changes its own cwd, so it stays at its launch directory forever
// and the parent always sees a stale dir. Inside tmux in particular this means
// #{pane_current_path} is frozen (usually /), which breaks new-window -c, the
// automatic window name, and tmux-resurrect's saved cwd. Following the child's
// OSC 7 reports keeps secretty's cwd in sync so the parent sees the truth.
//
// The observer only READS the stream (it never mutates bytes), consistent with
// secretty's ANSI no-mutation invariant. It requires the shell to emit OSC 7,
// which most modern shell integrations / prompts already do (zsh: add-zsh-hook
// chpwd; bash: PROMPT_COMMAND). Disable via SECRETTY_DISABLE_CWD_SYNC=1.
//
// The returned closure is invoked from a single goroutine (the output copy loop),
// so it needs no synchronization. os.Chdir is process-global; secretty loads its
// config before the child starts, so changing cwd mid-session is safe.
func newCwdSyncObserver(logger *debug.Logger) func([]byte) {
	var buf []byte
	last := ""
	return func(p []byte) {
		buf = append(buf, p...)
		if len(buf) > cwdWatchMaxBuffer {
			buf = buf[len(buf)-cwdWatchMaxBuffer:]
		}
		for {
			start := bytes.Index(buf, osc7Prefix)
			if start < 0 {
				// No prefix yet; retain only a short tail in case a prefix is
				// split across reads.
				if keep := len(osc7Prefix) - 1; len(buf) > keep {
					buf = append(buf[:0], buf[len(buf)-keep:]...)
				}
				return
			}
			rest := buf[start+len(osc7Prefix):]
			term, termLen := oscTerminator(rest)
			if term < 0 {
				// Incomplete sequence; keep from the prefix and wait for more.
				buf = append(buf[:0], buf[start:]...)
				return
			}
			if dir := parseOSC7Path(string(rest[:term])); dir != "" && dir != last {
				if err := os.Chdir(dir); err == nil {
					last = dir
					if logger != nil {
						logger.Infof("ptywrap: cwd_sync dir=%s", dir)
					}
				} else if logger != nil {
					logger.Infof("ptywrap: cwd_sync_chdir_failed dir=%s err=%v", dir, err)
				}
			}
			buf = append(buf[:0], rest[term+termLen:]...)
		}
	}
}

// oscTerminator returns the index and length of the earliest OSC terminator
// (BEL 0x07, or ST = ESC \) in b, or (-1, 0) if none is present yet.
func oscTerminator(b []byte) (int, int) {
	bel := bytes.IndexByte(b, 0x07)
	st := bytes.Index(b, oscST)
	switch {
	case bel < 0 && st < 0:
		return -1, 0
	case bel < 0:
		return st, len(oscST)
	case st < 0:
		return bel, 1
	case bel < st:
		return bel, 1
	default:
		return st, len(oscST)
	}
}

// parseOSC7Path extracts a filesystem path from an OSC 7 payload, conventionally
// "file://host/path" (path may be percent-encoded), tolerating a bare path too.
// Returns "" when nothing usable is found.
func parseOSC7Path(payload string) string {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return ""
	}
	if strings.HasPrefix(payload, "file://") {
		if u, err := url.Parse(payload); err == nil && u.Path != "" {
			return u.Path
		}
		// Fallback: strip scheme+authority, unescape the remaining path.
		rest := strings.TrimPrefix(payload, "file://")
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			if p, err := url.PathUnescape(rest[i:]); err == nil {
				return p
			}
			return rest[i:]
		}
		return ""
	}
	if strings.HasPrefix(payload, "/") {
		return payload
	}
	return ""
}
