package ptywrap

import (
	"context"
	"os/exec"
	"testing"
)

func TestRunCommandExitCode(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exit 7")
	code, err := RunCommand(context.Background(), cmd, Options{RawMode: false})
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
}
