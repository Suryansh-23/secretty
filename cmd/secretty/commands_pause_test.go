package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/suryansh-23/secretty/internal/ipc"
)

func TestValidatePauseFlags(t *testing.T) {
	tests := []struct {
		name      string
		pauseFor  string
		commands  int
		status    bool
		resume    bool
		shouldErr bool
	}{
		{name: "default", shouldErr: false},
		{name: "for", pauseFor: "3m", shouldErr: false},
		{name: "commands", commands: 3, shouldErr: false},
		{name: "status", status: true, shouldErr: false},
		{name: "resume", resume: true, shouldErr: false},
		{name: "negative commands", commands: -1, shouldErr: true},
		{name: "for and commands", pauseFor: "3m", commands: 2, shouldErr: true},
		{name: "status and resume", status: true, resume: true, shouldErr: true},
	}
	for _, tc := range tests {
		err := validatePauseFlags(tc.pauseFor, tc.commands, tc.status, tc.resume)
		if tc.shouldErr && err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !tc.shouldErr && err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}
}

func TestMapPauseIPCError(t *testing.T) {
	err := mapPauseIPCError(ipc.ErrUnsupportedOperation)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "refreshed SecreTTY wrapper") {
		t.Fatalf("unexpected message: %v", err)
	}

	plain := errors.New("boom")
	if got := mapPauseIPCError(plain); !errors.Is(got, plain) {
		t.Fatalf("expected passthrough error, got %v", got)
	}
}
