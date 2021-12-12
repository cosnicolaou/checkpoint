// Copyright 2020 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosnicolaou/checkpoint/checkpointstate"
	"github.com/cosnicolaou/checkpoint/directory"
)

type factory func() checkpointstate.Manager

var (
	managers = map[string]factory{}
)

func must(err error) {
	if err != nil {
		log.Fatalf("failed: %v", err)
	}
}

func init() {
	// For now only support directory based checkpoints, but in the future
	// it should be possible to support different ones such as dynamodb for use
	// from within AWS lambda's. Choice of the factory will be made via an environment
	// variable or some other out-of-band mechanism.
	managers["directory"] = func() checkpointstate.Manager {
		return directory.NewManager(filepath.Join(os.ExpandEnv("$HOME/.checkpointstate")))
	}
}

const (
	checkpointSessionIDEnvVar = "CHECKPOINT_SESSION_ID"
)

const usage = `
checkpoint: a simple means of recording and acting
on checkpoints in shell scripts (https://github.com/cosnicolaou/checkpoint).

Checkpoints are grouped into sessions and represent steps in
some sequential computation. Each step implicitly acknowledges the
previous one provided that the exit status of the last command
or pipeline ($?) represents success.

Example:

source <(checkpoint use $0)
completed step1 || <action>
completed step2 || <action>
completed
completed state

Sessions and checkpoints may be managed as follows:
 list        - list all checkpoints
 state       - display summary state of current checkpoint
 state <id>  - display summary state of specified checkpoint
 dump        - display full state, in json format
 dump <id>   - display full state, in json format, of specified checkpoint
 delete      - delete current checkpoint
 delete <id> - delete the specified session
 delete <id> step... -- delete the specified steps from the specified session

`

func main() {
	ctx := context.Background()
	fn := managers["directory"]
	mgr := fn()
	if ok, err := runCmd(ctx, mgr); ok {
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
			os.Exit(2)
		}
		return
	}

	step := ""
	switch len(os.Args) {
	case 0, 1:
	case 2:
		step = os.Args[1]
	default:
		fmt.Fprintf(os.Stderr, "FAILED: zero or one step must be specified\n")
		os.Exit(2)
	}

	ok, err := runStep(ctx, mgr, step)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(2)
	}
	if ok {
		// done
		os.Exit(0)
	}
	// not done.
	os.Exit(1)
}

func deleteSession(ctx context.Context, mgr checkpointstate.Manager, id string, steps ...string) error {
	sess, err := mgr.Use(ctx, id, false)
	if err != nil {
		return fmt.Errorf("failed to access session for %q: %v", id, err)
	}
	return sess.Delete(ctx, steps...)
}

func runListCmd(ctx context.Context, mgr checkpointstate.Manager) (bool, error) {
	sessions, err := mgr.List(ctx)
	if err != nil {
		return true, fmt.Errorf("failed to list sessions: %v", err)
	}
	for _, id := range sessions {
		sess, err := mgr.Use(ctx, id, false)
		if err != nil {
			return true, fmt.Errorf("failed to use session %v: %v", id, err)
		}
		md, err := sess.Metadata(ctx)
		if err != nil {
			return true, fmt.Errorf("failed to obtain metadata for session %v: %v", id, err)

		}
		buf, _ := json.MarshalIndent(md, "  ", "    ")
		fmt.Printf("%v: %s\n", id, buf)
	}
	return true, nil
}

func runStatusCmds(ctx context.Context, mgr checkpointstate.Manager, verb string) (bool, error) {
	nargs := len(os.Args)
	id := os.Getenv(checkpointSessionIDEnvVar)
	if nargs == 3 {
		id = os.Args[2]
	}
	if len(id) == 0 {
		return true, fmt.Errorf("no session found either as an argument or as environment variable %v", checkpointSessionIDEnvVar)
	}
	sess, err := mgr.Use(ctx, id, false)
	if err != nil {
		return true, fmt.Errorf("failed to use session %v: %v", id, err)
	}
	md, err := sess.Metadata(ctx)
	if err != nil {
		return true, fmt.Errorf("failed to get session metadata %v: %v", id, err)
	}
	steps, err := sess.Steps(ctx)
	if err != nil {
		return true, fmt.Errorf("failed to get session steps %v: %v", id, err)
	}
	if verb == "dump" {
		buf, _ := json.MarshalIndent(md, "", " ")
		fmt.Println(string(buf))
		for _, step := range steps {
			buf, _ := json.MarshalIndent(step, "", " ")
			fmt.Println(string(buf))
		}
		return true, nil
	}
	tags := []string{}
	for _, v := range md["Tags"].([]interface{}) {
		tags = append(tags, v.(string))
	}
	fmt.Printf("%v: %v\n", strings.Join(tags, ", "), md["ID"])
	for _, step := range steps {
		if step.Completed.IsZero() {
			fmt.Printf("%v: current: %v... %v\n", step.Name, step.Created, time.Now().Sub(step.Created))
			continue
		}
		fmt.Printf("%v: %v\n", step.Name, step.Completed.Sub(step.Created))
	}
	return true, nil
}

func runUseCmd(ctx context.Context, mgr checkpointstate.Manager) (bool, error) {
	nargs := len(os.Args)
	if nargs == 2 {
		return true, fmt.Errorf("no session name provided")
	}
	tags := os.Args[2:]
	id := mgr.SessionID(tags...)
	sess, err := mgr.Use(ctx, id, true)
	if err != nil {
		return true, fmt.Errorf("failed to use/create session for %v", tags)
	}
	metadata, err := sess.Metadata(ctx)
	if err != nil {
		return true, fmt.Errorf("failed to access metadata for %v: %v", tags, id)
	}
	if metadata == nil {
		metadata = map[string]interface{}{
			"Tags":    tags,
			"ID":      id,
			"Created": time.Now(),
		}
	}
	metadata["Accessed"] = time.Now()
	if err := sess.SetMetadata(ctx, metadata); err != nil {
		return true, fmt.Errorf("failed to write metadata for %v: %v: %v", tags, id, err)
	}
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "bash"):
		if err := checkBashVersion(); err != nil {
			return true, err
		}
	case strings.Contains(shell, "zsh"):
		if err := checkZshVersion(); err != nil {
			return true, err
		}
	default:
		return true, fmt.Errorf("unsupported shell: %q", shell)
	}
	fmt.Printf("export %s=%s\n", checkpointSessionIDEnvVar, id)
	fmt.Printf(`function completed() {
if [[ $? -ne 0 ]]; then
CHECKPOINT_ERROR=true
return 0
fi
[[ "$CHECKPOINT_ERROR" = "true" ]] && return 0
%s "$@"
}
`, os.Args[0])
	return true, nil
}

func checkBashVersion() error {
	out, err := exec.Command("bash", "--version").CombinedOutput()
	if err != nil {
		return err
	}
	if strings.Contains(string(out), "2007") {
		return fmt.Errorf("it looks like you are running an old version of bash on a mac, please upgrade to a more recent one")
	}
	return nil
}

func checkZshVersion() error {
	// All versions of zsh in common use as of 2021 support work.
	return nil
}

func runDeleteCmd(ctx context.Context, mgr checkpointstate.Manager) (bool, error) {
	id := os.Getenv(checkpointSessionIDEnvVar)
	nargs := len(os.Args)
	if nargs >= 3 {
		id = os.Args[2]
	}
	var steps []string
	if nargs >= 4 {
		steps = os.Args[3:]
	}
	if len(id) == 0 {
		return true, fmt.Errorf("no session found either as an argument or as environment variable %v", checkpointSessionIDEnvVar)
	}
	return true, deleteSession(ctx, mgr, id, steps...)
}

func runCmd(ctx context.Context, mgr checkpointstate.Manager) (bool, error) {
	if nargs := len(os.Args); nargs >= 2 {
		verb := os.Args[1]
		switch verb {
		case "help", "--help", "-help":
			fmt.Fprintf(os.Stderr, "Usage: %v\n", usage)
			os.Exit(0)
		case "list":
			return runListCmd(ctx, mgr)
		case "state", "status", "dump":
			return runStatusCmds(ctx, mgr, verb)
		case "use":
			return runUseCmd(ctx, mgr)
		case "delete":
			return runDeleteCmd(ctx, mgr)
		}
	}
	return false, nil
}

func runStep(ctx context.Context, mgr checkpointstate.Manager, name string) (bool, error) {
	id := os.Getenv(checkpointSessionIDEnvVar)
	sess, err := mgr.Use(ctx, id, false)
	if err != nil {
		return false, fmt.Errorf("failed to access session for %q: %v", id, err)
	}

	ok, err := sess.Step(ctx, name)
	if err != nil {
		return false, fmt.Errorf("failed to execute step %v: %v", name, err)
	}
	return ok, nil
}
