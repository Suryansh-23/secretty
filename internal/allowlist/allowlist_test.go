package allowlist

import "testing"

func TestMatchBasename(t *testing.T) {
	ok, err := Match([]string{"vim", "ssh"}, "vim", "")
	if err != nil {
		t.Fatalf("match error: %v", err)
	}
	if !ok {
		t.Fatalf("expected basename match")
	}
}

func TestMatchGlob(t *testing.T) {
	ok, err := Match([]string{"kubectl*"}, "kubectl", "/usr/local/bin/kubectl")
	if err != nil {
		t.Fatalf("match error: %v", err)
	}
	if !ok {
		t.Fatalf("expected glob match")
	}
}

func TestMatchFullPath(t *testing.T) {
	ok, err := Match([]string{"/usr/bin/less"}, "less", "/usr/bin/less")
	if err != nil {
		t.Fatalf("match error: %v", err)
	}
	if !ok {
		t.Fatalf("expected full path match")
	}
}

func TestMatchMiss(t *testing.T) {
	ok, err := Match([]string{"ssh"}, "git", "/usr/bin/git")
	if err != nil {
		t.Fatalf("match error: %v", err)
	}
	if ok {
		t.Fatalf("expected no match")
	}
}
