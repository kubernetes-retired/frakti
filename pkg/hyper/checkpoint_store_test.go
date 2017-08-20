/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hyper

import (
	"io/ioutil"
	"os"
	"sort"
	"testing"
)

func TestFileStore(t *testing.T) {
	path, err := ioutil.TempDir("", "FileStore")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanUpTestPath(t, path)
	store := &FileStore{path: path}

	Checkpoints := []struct {
		key  string
		data string
	}{
		{
			"key1",
			"data1",
		},
		{
			"key2",
			"data2",
		},
	}

	// Test Add Checkpoint
	for _, c := range Checkpoints {
		err = store.Add(c.key, []byte(c.data))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Test Get Checkpoint
		data, err := store.Get(c.key)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if string(data) != c.data {
			t.Errorf("Expected: %q, but got %q", c.data, data)
		}
	}
	// Test list checkpoints.
	keys, err := store.List()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	sort.Strings(keys)
	if !(keys[0] == "key1" && keys[1] == "key2") {
		t.Errorf("Expected: %q, but got %q", Checkpoints[0].key+";"+Checkpoints[1].key, keys)
	}

	// Test Delete Checkpoint
	for _, c := range Checkpoints {
		err = store.Delete(c.key)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	// Test list checkpoints.
	keys, err = store.List()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("Expected: %q, but got %q", "empty keys", "keys still has contents")
	}
}

func TestMemStore(t *testing.T) {
	mem := NewMemStore()
	Checkpoints := []struct {
		key  string
		data string
	}{
		{
			"key1",
			"data1",
		},
		{
			"key2",
			"data2",
		},
	}

	// Test Add checkpoints
	for _, c := range Checkpoints {
		err := mem.Add(c.key, []byte(c.data))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Test Get checkpoints
		data, err := mem.Get(c.key)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if string(data) != c.data {
			t.Errorf("Expected: %q, but got %q", c.data, data)
		}
	}
	// Test list checkpoints.
	keys, err := mem.List()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	sort.Strings(keys)
	if !(keys[0] == "key1" && keys[1] == "key2") {
		t.Errorf("Expected: %q, but got %q", Checkpoints[0].key+";"+Checkpoints[1].key, keys)
	}

	// Test Delete checkpoints
	for _, c := range Checkpoints {
		err = mem.Delete(c.key)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	// Test list checkpoints.
	keys, err = mem.List()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("Expected: %q, but got %q", "empty keys", "keys still has contents")
	}
}

func cleanUpTestPath(t *testing.T, path string) {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
}
