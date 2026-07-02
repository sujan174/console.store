package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"consolestore/internal/localstore"
)

// stubSeams overrides the destructive seams (agentsRemove, removeSelf, execPath)
// with recorders and restores them via t.Cleanup. It returns pointers to the
// call flags + the fake exec path used.
type seamRec struct {
	agentsCalled bool
	removeCalled bool
	removeArg    string
}

func stubSeams(t *testing.T) (*seamRec, string) {
	t.Helper()
	rec := &seamRec{}
	fakeExec := "/fake/bin/console"

	origAgents, origRemove, origExec := agentsRemove, removeSelf, execPath
	t.Cleanup(func() {
		agentsRemove, removeSelf, execPath = origAgents, origRemove, origExec
	})

	agentsRemove = func(out io.Writer) error {
		rec.agentsCalled = true
		return nil
	}
	removeSelf = func(path string) error {
		rec.removeCalled = true
		rec.removeArg = path
		return nil
	}
	execPath = func() (string, error) { return fakeExec, nil }
	return rec, fakeExec
}

// seedConfigDir creates the isolated config dir + a dummy file inside it and
// returns the dir path. Callers must have already set XDG_CONFIG_HOME.
func seedConfigDir(t *testing.T) string {
	t.Helper()
	dir, err := localstore.ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "presets.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestUninstallYesFullPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rec, fakeExec := stubSeams(t)
	dir := seedConfigDir(t)

	var out bytes.Buffer
	be := &fakeBackend{}
	code := runUninstall(Deps{SignedIn: true, Out: &out, Backend: be, Color: false}, []string{"--yes"})

	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out.String())
	}
	if !rec.agentsCalled {
		t.Error("agentsRemove not called")
	}
	if be.logoutN != 1 {
		t.Errorf("Logout called %d times, want 1", be.logoutN)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("config dir still exists (err=%v)", err)
	}
	if !rec.removeCalled {
		t.Error("removeSelf not called")
	}
	if rec.removeArg != fakeExec {
		t.Errorf("removeSelf arg = %q, want %q", rec.removeArg, fakeExec)
	}
}

func TestUninstallKeepBinary(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rec, _ := stubSeams(t)
	dir := seedConfigDir(t)

	var out bytes.Buffer
	be := &fakeBackend{}
	code := runUninstall(Deps{SignedIn: true, Out: &out, Backend: be}, []string{"--yes", "--keep-binary"})

	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out.String())
	}
	if rec.removeCalled {
		t.Error("removeSelf must NOT be called with --keep-binary")
	}
	if !rec.agentsCalled {
		t.Error("agentsRemove should still be called")
	}
	if be.logoutN != 1 {
		t.Errorf("Logout called %d times, want 1", be.logoutN)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("config dir still exists (err=%v)", err)
	}
}

func TestUninstallNotSignedIn(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, _ = stubSeams(t)
	dir := seedConfigDir(t)

	var out bytes.Buffer
	be := &fakeBackend{}
	code := runUninstall(Deps{SignedIn: false, Out: &out, Backend: be}, []string{"--yes"})

	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out.String())
	}
	if be.logoutN != 0 {
		t.Errorf("Logout must NOT be called when signed out, got %d", be.logoutN)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("config dir should still be removed (err=%v)", err)
	}
}

func TestUninstallNonInteractiveNoYes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rec, _ := stubSeams(t)
	dir := seedConfigDir(t)

	var out bytes.Buffer
	be := &fakeBackend{}
	code := runUninstall(Deps{SignedIn: true, Out: &out, Backend: be, Interactive: false}, nil)

	if code != 1 {
		t.Fatalf("exit = %d, want 1:\n%s", code, out.String())
	}
	if rec.agentsCalled || rec.removeCalled || be.logoutN != 0 {
		t.Error("nothing must be destroyed without confirmation")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("config dir must still exist (err=%v)", err)
	}
}

func TestUninstallInteractiveConfirm(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		proceed bool
	}{
		{"yes proceeds", "yes\n", true},
		{"y proceeds", "y\n", true},
		{"n aborts", "n\n", false},
		{"empty aborts", "\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			rec, _ := stubSeams(t)
			dir := seedConfigDir(t)

			var out bytes.Buffer
			be := &fakeBackend{}
			d := Deps{
				SignedIn:    true,
				Out:         &out,
				Backend:     be,
				Interactive: true,
				In:          strings.NewReader(tc.input),
				Ctx:         context.Background(),
			}
			code := runUninstall(d, nil)

			if code != 0 {
				t.Fatalf("exit = %d, want 0:\n%s", code, out.String())
			}
			if tc.proceed {
				if !rec.agentsCalled || !rec.removeCalled || be.logoutN != 1 {
					t.Errorf("should have proceeded (agents=%v remove=%v logout=%d)", rec.agentsCalled, rec.removeCalled, be.logoutN)
				}
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Errorf("config dir should be removed (err=%v)", err)
				}
			} else {
				if rec.agentsCalled || rec.removeCalled || be.logoutN != 0 {
					t.Error("aborted run must not destroy anything")
				}
				if _, err := os.Stat(dir); err != nil {
					t.Errorf("config dir must still exist (err=%v)", err)
				}
			}
		})
	}
}

func TestUninstallUnknownFlag(t *testing.T) {
	var out bytes.Buffer
	code := runUninstall(Deps{Out: &out}, []string{"--bogus"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}
