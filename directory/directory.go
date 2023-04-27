// Copyright 2020 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

// Package directory contains an implementation of checkpointstate.Manager
// and checkpointstate.Session that uses a local file system directory
// and files therein to represent checkpoints.
package directory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cosnicolaou/checkpoint/checkpointstate"
	"golang.org/x/sys/unix"
)

type directoryManager struct {
	root string
}

const (
	currentStepFile = "in-progress"
	metadataFile    = "metadata"
	timeFormat      = time.RFC3339Nano
)

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
	if reset {
		if err := os.Mkdir(sessionDir, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
		if err := os.Remove(filepath.Join(sessionDir, currentStepFile)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return &directorySession{session: sessionDir}, nil
}

// List implements checkpointstate.Manager.
func (dm *directoryManager) List(ctx context.Context) ([]string, error) {
	dirs := []string{}
	err := filepath.Walk(dm.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != dm.root {
			dirs = append(dirs, info.Name())
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs, err
}

type stepState struct {
	Step     string
	StepFile string
	// RFC3339Nano formatted times.
	Created   string
	Completed string
}

// Step implements checkpointstate.Session
func (ds *directorySession) Step(ctx context.Context, step string) (bool, error) {
	unlock, err := lock(ds.session)
	defer unlock()
	if err != nil {
		return false, err
	}

	// Mark the prior step, if any, as done.
	if err := ds.markDone(ctx, step); err != nil {
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
	buf, _ := json.Marshal(stepState{
		Step:     step,
		Created:  time.Now().Format(timeFormat),
		StepFile: stepFile,
	})
	// Mark the requested step as in process.
	return false, ioutil.WriteFile(filepath.Join(ds.session, currentStepFile), buf, 0600)
}

func (ds *directorySession) Steps(ctx context.Context) ([]checkpointstate.Step, error) {
	steps := []checkpointstate.Step{}
	err := filepath.Walk(ds.session, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() == metadataFile {
			return nil
		}
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		var state stepState
		if err := json.Unmarshal(buf, &state); err != nil {
			return nil
		}
		var created, completed time.Time
		created, _ = time.Parse(timeFormat, state.Created)
		if len(state.Completed) > 0 {
			completed, _ = time.Parse(timeFormat, state.Completed)
		}
		steps = append(steps, checkpointstate.Step{
			Name:      state.Step,
			Created:   created,
			Completed: completed,
		})
		return nil
	})
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Created.Before(steps[j].Created)
	})
	return steps, err
}

func (ds *directorySession) markDone(ctx context.Context, step string) error {
	current := filepath.Join(ds.session, currentStepFile)
	buf, err := ioutil.ReadFile(current)
	if err != nil {
		if os.IsNotExist(err) {
			// treat a non-existent step as success.
			return nil
		}
		return err
	}
	var state stepState
	if err := json.Unmarshal(buf, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state for current step %v", err)
	}
	if state.StepFile == filepath.Join(ds.session, step) {
		return nil
	}
	if _, err := os.Stat(state.StepFile); err == nil || !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("step %v is being reused", state.StepFile)
		}
		return fmt.Errorf("step %v is being reused or it could not be accessed: %v", state.StepFile, err)
	}
	state.Completed = time.Now().Format(timeFormat)
	err = os.Rename(current, state.StepFile)
	buf, _ = json.Marshal(state)
	ioutil.WriteFile(state.StepFile, buf, 0400)
	return err
}

// Delete implements checkpointstate.Session,
func (ds *directorySession) Delete(ctx context.Context, steps ...string) error {
	unlock, err := lock(ds.session)
	defer unlock()
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		// Note that this will delete the underlying directory before the
		// lock on it is released.
		return os.RemoveAll(ds.session)
	}
	for _, step := range steps {
		if err := os.Remove(filepath.Join(ds.session, step)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// SetMetadata implements checkpointstate.Session,
func (ds *directorySession) SetMetadata(ctx context.Context, metadata map[string]interface{}) error {
	unlock, err := lock(ds.session)
	defer unlock()
	if err != nil {
		return err
	}
	buf, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to json encode metadata: %v", err)
	}
	return ioutil.WriteFile(filepath.Join(ds.session, metadataFile), buf, 0600)
}

// Metadata implements checkpointstate.Session,
func (ds *directorySession) Metadata(ctx context.Context) (map[string]interface{}, error) {
	unlock, err := lock(ds.session)
	defer unlock()
	if err != nil {
		return nil, err
	}
	filename := filepath.Join(ds.session, metadataFile)
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var md map[string]interface{}
	if err := json.Unmarshal(buf, &md); err != nil {
		return nil, fmt.Errorf("failed to decode json data from %v: %v", filename, err)
	}
	return md, nil
}
