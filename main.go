// Copyright 2020 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	CHECKPOINT_SESSION_ID = "CHECKPOINT_SESSION_ID"
)

func main() {
	ctx := context.Background()
	fn := managers["directory"]
	mgr := fn()
	if ok, err := useOrDelete(ctx, mgr); ok {
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
		os.Exit(1)
	}
	// not done.
	os.Exit(0)
}

func deleteSession(ctx context.Context, mgr checkpointstate.Manager, id string) error {
	sess, err := mgr.Use(ctx, id, false)
	if err != nil {
		return fmt.Errorf("failed to access session for %q: %v\n", id, err)
	}
	return sess.Delete(ctx)
}

func useOrDelete(ctx context.Context, mgr checkpointstate.Manager) (bool, error) {
	if nargs := len(os.Args); nargs >= 2 {
		switch os.Args[1] {
		case "use":
			if nargs == 2 {
				return true, fmt.Errorf("no session name provided\n")
			}
			id := mgr.SessionID(os.Args[2:]...)
			if _, err := mgr.Use(ctx, id, true); err != nil {
				return true, fmt.Errorf("failed to use/create session for %v\n", os.Args[2:])
			}
			shell := os.Getenv("SHELL")
			switch {
			case strings.Contains(shell, "bash") || strings.Contains(shell, "zsh"):
			default:
				return true, fmt.Errorf("unsupported shell: %v", shell)
			}
			fmt.Printf("export %s=%s\n", CHECKPOINT_SESSION_ID, id)
			fmt.Printf(`function step() {
	%s "$@"
}
`, os.Args[0])
			return true, nil
		case "delete":
			id := os.Getenv(CHECKPOINT_SESSION_ID)
			if len(id) > 0 {
				return true, deleteSession(ctx, mgr, id)
			}
			if nargs == 2 {
				return true, fmt.Errorf("no session name provided\n")
			}
			return true, deleteSession(ctx, mgr, mgr.SessionID(os.Args[3:]...))
		}
	}
	return false, nil
}

func runStep(ctx context.Context, mgr checkpointstate.Manager, name string) (bool, error) {
	id := os.Getenv(CHECKPOINT_SESSION_ID)
	sess, err := mgr.Use(ctx, id, false)
	if err != nil {
		return false, fmt.Errorf("failed to access session for %q: %v\n", id, err)
	}
	ok, err := sess.Step(ctx, name)
	if err != nil {
		return false, fmt.Errorf("failed to execute step %v: %v\n", name, err)
	}
	return ok, nil
}
