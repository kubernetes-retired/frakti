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

package proc

import (
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func checkKillError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "os: process already finished") || err == unix.ESRCH {
		return errors.Wrapf(errdefs.ErrNotFound, "process already finished")
	}
	return errors.Wrapf(err, "unknown error after kill")
}

func hasNoIO(r *InitConfig) bool {
	return r.Stdin == "" && r.Stdout == "" && r.Stderr == ""
}
