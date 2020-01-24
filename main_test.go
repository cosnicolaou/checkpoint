package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"v.io/x/lib/gosh"
)

var (
	sh        *gosh.Shell
	cmd       string
	tmpDir    string
	setupOnce sync.Once
)

func setup(t *testing.T) {
	setupOnce.Do(func() {
		sh = gosh.NewShell(t)
		tmpDir = sh.MakeTempDir()
		cmd = gosh.BuildGoPkg(sh, tmpDir, "github.com/cosnicolaou/checkpoint")
	})
}

func TestLocal(t *testing.T) {
	setup(t)
	testScripts(t, nil)
}

func TestMain(m *testing.M) {
	rc := m.Run()
	if sh != nil {
		sh.Cleanup()
	}
	os.Exit(rc)
}

func runBashScript(script string, env map[string]string) string {
	cmd := sh.Cmd("bash", filepath.Join("testdata", script))
	for k, v := range env {
		cmd.Vars[k] = v
	}
	cmd.Vars["HOME"] = tmpDir
	cmd.Vars["PATH"] += ":" + tmpDir
	return strings.TrimSpace(cmd.CombinedOutput())
}

func testScripts(t *testing.T, env map[string]string) {
	sessionID := runBashScript("id.bash", env)
	if got, want := sessionID, "2139b237e3f2fc08bf7e9265b24e22af4f10fd98439009fb847f43e2e0ee335b"; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
	r1 := runBashScript("s2.bash", env)
	if got, want := r1, "1\n2"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	r2 := runBashScript("s2.bash", env)
	if got, want := r2, "2"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	r1 = runBashScript("s3.bash", env)
	if got, want := r1, "1\n2"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	r2 = runBashScript("s3.bash", env)
	if got, want := r2, ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
