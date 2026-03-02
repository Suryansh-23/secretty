package sessioncontrol

import (
	"testing"
	"time"
)

func TestPauseForExpires(t *testing.T) {
	ctrl := NewController()
	ctrl.PauseFor(25 * time.Millisecond)
	if !ctrl.IsPausedNow() {
		t.Fatal("expected paused immediately")
	}
	time.Sleep(40 * time.Millisecond)
	if ctrl.IsPausedNow() {
		t.Fatal("expected pause to expire")
	}
}

func TestPauseCommandsConsumes(t *testing.T) {
	ctrl := NewController()
	ctrl.PauseCommands(2)
	st := ctrl.Status()
	if !st.Active || st.Mode != ModeCommands || st.RemainingCommands != 2 {
		t.Fatalf("unexpected status: %+v", st)
	}

	ctrl.ConsumeCommandLine()
	st = ctrl.Status()
	if !st.Active || st.RemainingCommands != 1 {
		t.Fatalf("unexpected status after first consume: %+v", st)
	}

	ctrl.ConsumeCommandLine()
	st = ctrl.Status()
	if st.Active || st.Mode != ModeNone {
		t.Fatalf("expected no active pause, got %+v", st)
	}
}

func TestResumeClearsState(t *testing.T) {
	ctrl := NewController()
	ctrl.PauseFor(2 * time.Minute)
	ctrl.Resume()
	st := ctrl.Status()
	if st.Active || st.Mode != ModeNone || st.RemainingCommands != 0 || !st.Until.IsZero() {
		t.Fatalf("unexpected status: %+v", st)
	}
}
