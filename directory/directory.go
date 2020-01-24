// Copyright 2020 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.
package directory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/cosnicolaou/checkpoint/checkpointstate"
	"golang.org/x/sys/unix"
)

type directoryManager struct {
	root string
}

const currentStepFile = "in-progress"

// NewManager returns a new instance of a checkpointstate.Manager that
// manages checkpoints in a local, POSIX-compliant, filesystem directory.
func NewManager(dir string) checkpointstate.Manager {
	if err := os.MkdirAll(dir, 0777); err != nil {
		log.Fatalf("failed to create directory: %v", dir)
	}
	return &directoryManager{root: dir}
}

type directorySession struct {
	session string
	err     error
}

func lock(name string) (func(), error) {
	f, err := os.Open(name)
	if err != nil {
		return func() {}, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return func() {}, err
	}
	return func() {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
	}, nil
}

// SessionID implements checkpointstate.Manager.
func (dm *directoryManager) SessionID(keys ...string) string {
	h := sha256.New()
	for _, k := range keys {
		dgst := sha256.Sum256([]byte(k))
		h.Write(dgst[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Use implements checkpointstate.Manager.
func (dm *directoryManager) Use(ctx context.Context, id string, reset bool) (checkpointstate.Session, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("empty session id")
	}
	unlock, err := lock(dm.root)
	defer unlock()
	if err != nil {
		return nil, err
	}
	sessionDir := filepath.Join(dm.root, id)
	if err := os.Mkdir(sessionDir, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	if reset {
		if err := os.Remove(filepath.Join(sessionDir, currentStepFile)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return &directorySession{session: sessionDir}, nil
}

// Step implements checkpointstate.Session
func (ds *directorySession) Step(ctx context.Context, step string) (bool, error) {
	unlock, err := lock(ds.session)
	defer unlock()
	if err != nil {
		return false, err
	}

	// Mark the prior step, if any, as done.
	if err := ds.markDone(step); err != nil {
		return false, err
	}

	// No next step was requested.
	if len(step) == 0 {
		return true, nil
	}

	// Determine if the requested step has been completed,
	// ie. the associated file exists.
	stepFile := filepath.Join(ds.session, step)
	_, err = ioutil.ReadFile(stepFile)
	if err == nil {
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	// Mark the requested step as in process.
	return false, ioutil.WriteFile(filepath.Join(ds.session, currentStepFile), []byte(stepFile), 0400)
}

func (ds *directorySession) markDone(step string) error {
	current := filepath.Join(ds.session, currentStepFile)
	buf, err := ioutil.ReadFile(current)
	if err != nil {
		if os.IsNotExist(err) {
			// treat a non-existent step as success.
			return nil
		}
		return err
	}
	if string(buf) == filepath.Join(ds.session, step) {
		return nil
	}
	stepFile := string(buf)
	if _, err := os.Stat(stepFile); err == nil || !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("step %v is being reused", stepFile)
		}
		return fmt.Errorf("step %v is being reused or it could not be accessed: %v", stepFile, err)
	}
	return os.Rename(current, stepFile)
}

// Delete implements checkpointstate.Session,
func (ds *directorySession) Delete(ctx context.Context) error {
	unlock, err := lock(ds.session)
	defer unlock()
	if err != nil {
		return err
	}
	// Note that this will delete the underlying directory before the
	// lock on it is released.
	return os.RemoveAll(ds.session)
}
