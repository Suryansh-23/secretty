package sessioncontrol

import (
	"sync"
	"time"
)

// Mode describes the active pause strategy.
type Mode string

const (
	ModeNone     Mode = "none"
	ModeTime     Mode = "time"
	ModeCommands Mode = "commands"
)

// Status reports the current pause state.
type Status struct {
	Active            bool
	Mode              Mode
	Until             time.Time
	RemainingCommands int
}

// Controller tracks session-scoped pause state.
type Controller struct {
	mu                sync.Mutex
	mode              Mode
	until             time.Time
	remainingCommands int
}

// NewController returns a ready-to-use pause controller.
func NewController() *Controller {
	return &Controller{mode: ModeNone}
}

// PauseFor pauses redaction until now+d.
func (c *Controller) PauseFor(d time.Duration) {
	if c == nil {
		return
	}
	if d <= 0 {
		c.Resume()
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = ModeTime
	c.until = time.Now().Add(d)
	c.remainingCommands = 0
}

// PauseCommands pauses redaction for the next n command lines.
func (c *Controller) PauseCommands(n int) {
	if c == nil {
		return
	}
	if n <= 0 {
		c.Resume()
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = ModeCommands
	c.until = time.Time{}
	c.remainingCommands = n
}

// Resume clears any active pause.
func (c *Controller) Resume() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = ModeNone
	c.until = time.Time{}
	c.remainingCommands = 0
}

// Status returns the active state and remaining values.
func (c *Controller) Status() Status {
	if c == nil {
		return Status{Active: false, Mode: ModeNone}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.normalizeLocked(time.Now())
	return c.statusLocked(time.Now())
}

// IsPausedNow reports whether redaction should currently be paused.
func (c *Controller) IsPausedNow() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.normalizeLocked(time.Now())
	return c.mode != ModeNone
}

// ConsumeCommandLine decrements command-based pauses.
func (c *Controller) ConsumeCommandLine() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.normalizeLocked(time.Now())
	if c.mode != ModeCommands || c.remainingCommands <= 0 {
		return
	}
	c.remainingCommands--
	if c.remainingCommands <= 0 {
		c.mode = ModeNone
		c.until = time.Time{}
		c.remainingCommands = 0
	}
}

func (c *Controller) normalizeLocked(now time.Time) {
	if c.mode == ModeTime && !c.until.IsZero() && !now.Before(c.until) {
		c.mode = ModeNone
		c.until = time.Time{}
		c.remainingCommands = 0
	}
	if c.mode == ModeCommands && c.remainingCommands <= 0 {
		c.mode = ModeNone
		c.until = time.Time{}
		c.remainingCommands = 0
	}
}

func (c *Controller) statusLocked(now time.Time) Status {
	st := Status{
		Active:            c.mode != ModeNone,
		Mode:              c.mode,
		Until:             c.until,
		RemainingCommands: c.remainingCommands,
	}
	if c.mode == ModeNone {
		st.Mode = ModeNone
	}
	if c.mode == ModeTime && !st.Until.IsZero() && !now.Before(st.Until) {
		return Status{Active: false, Mode: ModeNone}
	}
	return st
}
