package main

import (
	"testing"

	"github.com/suryansh-23/secretty/internal/sessioncontrol"
)

func TestCommandLineObserverCountsCRLFOnce(t *testing.T) {
	ctrl := sessioncontrol.NewController()
	ctrl.PauseCommands(2)
	observe := commandLineObserver(ctrl)

	observe([]byte("echo hello\r\n"))
	st := ctrl.Status()
	if !st.Active || st.RemainingCommands != 1 {
		t.Fatalf("unexpected status after first line: %+v", st)
	}

	observe([]byte("echo again\n"))
	st = ctrl.Status()
	if st.Active {
		t.Fatalf("expected pause consumed, got %+v", st)
	}
}

func TestCommandLineObserverIgnoresNonNewlineBytes(t *testing.T) {
	ctrl := sessioncontrol.NewController()
	ctrl.PauseCommands(1)
	observe := commandLineObserver(ctrl)

	observe([]byte("abc123"))
	st := ctrl.Status()
	if !st.Active || st.RemainingCommands != 1 {
		t.Fatalf("unexpected status after plain bytes: %+v", st)
	}
}
