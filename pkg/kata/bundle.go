/*
Copyright 2018 The Kubernetes Authors.

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

package kata

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	configFilename = "config.json"
)

// loadBundle loads an existing bundle from disk
func loadBundle(id, path, workdir string) *bundle {
	return &bundle{
		id:      id,
		path:    path,
		workDir: workdir,
	}
}

// newBundle creates a new bundle on disk at the provided path for the given id
func newBundle(id, path, workDir string, spec []byte) (b *bundle, err error) {
	if err := os.MkdirAll(path, 0711); err != nil {
		return nil, err
	}
	path = filepath.Join(path, id)
	defer func() {
		if err != nil {
			os.RemoveAll(path)
		}
	}()
	workDir = filepath.Join(workDir, id)
	if err := os.MkdirAll(workDir, 0711); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(workDir)
		}
	}()

	if err := os.Mkdir(path, 0711); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(path, "rootfs"), 0711); err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(filepath.Join(path, configFilename), spec, 0666)
	return &bundle{
		id:      id,
		path:    path,
		workDir: workDir,
	}, err
}

type bundle struct {
	id      string
	path    string
	workDir string
}

// Delete deletes the bundle from disk
func (b *bundle) Delete() error {
	err := os.RemoveAll(b.path)
	if err == nil {
		return os.RemoveAll(b.workDir)
	}
	// error removing the bundle path; still attempt removing work dir
	err2 := os.RemoveAll(b.workDir)
	if err2 == nil {
		return err
	}
	return errors.Wrapf(err, "Failed to remove both bundle and workdir locations: %v", err2)
}
