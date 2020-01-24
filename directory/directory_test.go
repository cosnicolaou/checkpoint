// Copyright 2020 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.
package directory_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/cosnicolaou/checkpoint/directory"
)

func list(root string) []string {
	r := []string{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path != root {
			r = append(r, path)
		}
		return nil
	})
	sort.Strings(r)
	return r
}

func TestIDs(t *testing.T) {
	dir, err := ioutil.TempDir("", "local-file")
	if err != nil {
		t.Fatal(err)
	}
	mgr := directory.NewManager(dir)
	for i, tc := range []struct {
		input []string
		id    string
	}{
		{nil, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{[]string{}, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{[]string{"a", "b"}, "e5a01fee14e0ed5c48714f22180f25ad8365b53f9779f79dc4a3d7e93963f94a"},
		{[]string{"b", "a"}, "18d79cb747ea174c59f3a3b41768672526d56fecc58360a99d283d0f9b0a3cc0"},
	} {
		if got, want := mgr.SessionID(tc.input...), tc.id; got != want {
			t.Errorf("%v: %v, want %v", i, got, want)
		}
	}
}

func TestDirectory(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "local-file")
	if err != nil {
		t.Fatal(err)
	}
	mgr := directory.NewManager(dir)
	id := mgr.SessionID("/a/b/c")
	fmt.Printf("ID: %v", id)
	sess, err := mgr.Use(ctx, id, true)
	if got, want := list(dir), []string{filepath.Join(dir, id)}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	var gotOk bool
	var gotError error

	expect := func(ticked bool, err string) {
		_, file, line, _ := runtime.Caller(1)
		loc := fmt.Sprintf("%v:%v", filepath.Base(file), line)

		if got, want := gotOk, ticked; got != want {
			t.Errorf("%v: %v, want %v", loc, got, want)
		}
		if len(err) == 0 {
			if gotError != nil {
				t.Errorf("%v: unexpected error: %v", loc, gotError)
			}
			return
		}
		if got, want := gotError, err; !strings.Contains(got.Error(), err) {
			t.Errorf("%v: %v does not contain %v", loc, got, want)
		}
	}

	gotOk, gotError = sess.Step(ctx, "a")
	expect(false, "")

	gotOk, gotError = sess.Step(ctx, "b")
	expect(false, "")

	gotOk, gotError = sess.Step(ctx, "a")
	expect(true, "")

	gotOk, gotError = sess.Step(ctx, "b")
	expect(true, "")

	sess, err = mgr.Use(ctx, id, true)
	if err != nil {
		t.Fatal(err)
	}
	gotOk, gotError = sess.Step(ctx, "a")
	expect(true, "")

	gotOk, gotError = sess.Step(ctx, "b")
	expect(true, "")

	// make sure that reseting the current step can be overriden.
	sess, err = mgr.Use(ctx, id, false)
	if err != nil {
		t.Fatal(err)
	}

	gotOk, gotError = sess.Step(ctx, "c")
	expect(false, "")

	gotOk, gotError = sess.Step(ctx, "")
	expect(true, "")

	gotOk, gotError = sess.Step(ctx, "c")
	expect(true, "")

	sess.Delete(ctx)

	sess, err = mgr.Use(ctx, id, true)
	if err != nil {
		t.Fatal(err)
	}

	gotOk, gotError = sess.Step(ctx, "a")
	expect(false, "")

	gotOk, gotError = sess.Step(ctx, "b")
	expect(false, "")

	sess, err = mgr.Use(ctx, id, true)
	if err != nil {
		t.Fatal(err)
	}
	gotOk, gotError = sess.Step(ctx, "a")
	expect(true, "")

	gotOk, gotError = sess.Step(ctx, "b")
	expect(false, "")

	gotOk, gotError = sess.Step(ctx, "")
	expect(true, "")

	sess, err = mgr.Use(ctx, id, true)
	if err != nil {
		t.Fatal(err)
	}
	gotOk, gotError = sess.Step(ctx, "a")
	expect(true, "")

	gotOk, gotError = sess.Step(ctx, "b")
	expect(true, "")

}
