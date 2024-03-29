package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

/*
func TestVersion(t *testing.T) {
	setup(t)
	cmd := exec.Command("bash", "-c", cmd+" use x")
	cmd.Env = append(cmd.Env,
		"SHELL=bash",
		"HOME="+tmpDir,
		"PATH="+os.Getenv("PATH")+":"+tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Log(string(out))
		t.Fatal(err)
	}
}*/

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
	bash, err := exec.LookPath("bash")
	if err != nil {
		return err.Error()
	}
	cmd.Vars["BASH"] = bash
	cmd.Vars["HOME"] = tmpDir
	cmd.Vars["PATH"] += ":" + tmpDir
	return strings.TrimSpace(cmd.CombinedOutput())
}

func testScripts(t *testing.T, env map[string]string) {
	sessionID := runBashScript("id.bash", env)
	if got, want := sessionID, "2139b237e3f2fc08bf7e9265b24e22af4f10fd98439009fb847f43e2e0ee335b"; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}

	runner := func(script, gr1, gr2 string) {
		_, file, line, _ := runtime.Caller(1)
		loc := fmt.Sprintf("%v:%v", filepath.Base(file), line)
		r1 := runBashScript(script, env)
		if got, want := r1, gr1; got != want {
			t.Errorf("%v: pass 1: got %v, want %v", loc, got, want)
		}
		r2 := runBashScript(script, env)
		if got, want := r2, gr2; got != want {
			t.Errorf("%v: pass 2: got %v, want %v", loc, got, want)
		}
	}

	// step 2 will be rerun
	runner("s2.bash", "1\n2", "2")
	// step 2 will not be rerun since the trap marks it as complete
	runner("s3.bash", "1\n2", "")
	// show how to use a date as a session name
	runner("s4.zsh", "new data", "already processed")
	// test that s1 is not marked complet if there's
	runner("s5.bash", "1", "1")

	type pair struct {
		line     int
		contains string
	}
	dumper := func(script string, pairs []pair) {
		_, file, line, _ := runtime.Caller(1)
		loc := fmt.Sprintf("%v:%v", filepath.Base(file), line)
		output := runBashScript(script, env)
		lines := strings.Split(output, "\n")
		for _, p := range pairs {
			if p.line > len(lines) {
				t.Errorf("%v: line %v: does not exist in %v", loc, p.line, lines)
			}
			if got, want := lines[p.line], p.contains; !strings.Contains(got, want) {
				t.Errorf("%v: line %v: got %v, does not contain %v", loc, p.line, got, want)
			}
		}
	}

	dumper("dump.bash", []pair{
		{0, "1"},
		{1, "2"},
		{2, "3"},
		{6, `"8adc3205a6ba550ac20c8d228463593acb01957ebc653082633e63686f6d56c7"`},
		{12, `"Name": "s1"`},
		{17, `"Name": "s2"`},
		{22, `"Name": "s3"`},
		{24, `"Completed": "0001-01-01T00:00:00Z"`},
	})

	dumper("state.bash", []pair{
		{0, "1"},
		{1, "2"},
		{2, "3"},
		{3, "state.bash: 6b2bd8411dfc68fa79960ae7619f78b74fb40cbb8ea699ffd66186c091fffdd1"},
		{4, "s1"},
		{5, "s2"},
		{6, "s3: current"},
	})

	runner("s6.bash", "1\n2\n3", "")
	dumper("s6-delete.bash", []pair{
		{0, `s6.bash: 01b2ad98e69c47b473c54c0e15cfc0ce62d3e209a9b23f8f39ec37bc4a587b9d`},
		{1, "s1:"},
		{2, "s2:"},
		{3, "s3:"},
		{4, "s6.bash: 01b2ad98e69c47b473c54c0e15cfc0ce62d3e209a9b23f8f39ec37bc4a587b9d"},
		{5, "s1:"},
		{6, "s3:"},
		{7, "s6.bash: 01b2ad98e69c47b473c54c0e15cfc0ce62d3e209a9b23f8f39ec37bc4a587b9d"},
		{8, "s3:"},
	})

	// 2 will be redone
	runner("s7.bash", "1\n2", "2")
}
