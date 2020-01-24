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

	r1 = runBashScript("s4.zsh", env)
	if got, want := r1, "new data"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	r2 = runBashScript("s4.zsh", env)
	if got, want := r2, "already processed"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	dump := runBashScript("dump.bash", env)
	for _, line := range []string{
		`"8adc3205a6ba550ac20c8d228463593acb01957ebc653082633e63686f6d56c7"`,
		`"Name": "s1"`,
		`"Name": "s2"`,
		`"Name": "s3"`,
	} {
		if got, want := dump, line; !strings.Contains(got, want) {
			t.Errorf("got %v, does not contain %v", got, want)
		}
	}
	if got, want := dump, `"Completed": "0001-01-01T00:00:00Z"
}`; !strings.HasSuffix(got, want) {
		t.Errorf("got %v, does not end with %v", got, want)
	}

	state := runBashScript("state.bash", env)
	for _, line := range []string{
		"state.bash: 6b2bd8411dfc68fa79960ae7619f78b74fb40cbb8ea699ffd66186c091fffdd1",
		"s1",
		"s2",
		"s3: current",
	} {
		if got, want := state, line; !strings.Contains(got, want) {
			t.Errorf("got %v, does not contain %v", got, want)
		}
	}
	lines := strings.Split(state, "\n")
	if got, want := lines[len(lines)-1], "s3: current"; !strings.HasPrefix(got, want) {
		t.Errorf("got %v, last line does not start with %v", got, want)
	}
}
