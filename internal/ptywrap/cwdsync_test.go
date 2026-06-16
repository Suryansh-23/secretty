package ptywrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOSC7Path(t *testing.T) {
	cases := map[string]string{
		"file://host/Users/me/proj":  "/Users/me/proj",
		"file:///Users/me/proj":      "/Users/me/proj",
		"file://host/Users/me/a%20b": "/Users/me/a b",
		"/bare/path":                 "/bare/path",
		"":                           "",
		"file://host":                "",
		"notaurl":                    "",
	}
	for in, want := range cases {
		if got := parseOSC7Path(in); got != want {
			t.Errorf("parseOSC7Path(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOSCTerminator(t *testing.T) {
	if i, n := oscTerminator([]byte("abc\x07def")); i != 3 || n != 1 {
		t.Errorf("BEL: got (%d,%d), want (3,1)", i, n)
	}
	if i, n := oscTerminator([]byte("abc\x1b\\def")); i != 3 || n != 2 {
		t.Errorf("ST: got (%d,%d), want (3,2)", i, n)
	}
	if i, n := oscTerminator([]byte("\x07\x1b\\")); i != 0 || n != 1 {
		t.Errorf("earliest: got (%d,%d), want (0,1)", i, n)
	}
	if i, n := oscTerminator([]byte("no terminator")); i != -1 || n != 0 {
		t.Errorf("none: got (%d,%d), want (-1,0)", i, n)
	}
}

func TestCwdSyncObserverChdir(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	obs := newCwdSyncObserver(nil)

	// BEL-terminated, single chunk, surrounded by other output.
	obs([]byte("hello\x1b]7;file://host" + dir + "\x07world"))
	assertSameDir(t, dir)

	// ST-terminated sequence split across two observer calls.
	if err := os.Chdir(orig); err != nil {
		t.Fatal(err)
	}
	seq := "\x1b]7;file://host" + sub + "\x1b\\"
	obs([]byte(seq[:9]))
	obs([]byte(seq[9:]))
	assertSameDir(t, sub)
}

func assertSameDir(t *testing.T, want string) {
	t.Helper()
	got, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	gi, err1 := os.Stat(got)
	wi, err2 := os.Stat(want)
	if err1 != nil || err2 != nil || !os.SameFile(gi, wi) {
		t.Errorf("cwd = %q, want %q (stat errs: %v, %v)", got, want, err1, err2)
	}
}
