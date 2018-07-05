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
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/containerd/console"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	vc "github.com/kata-containers/runtime/virtcontainers"
)

type ExecProcess struct {
	wg sync.WaitGroup

	State

	mu    sync.Mutex
	id    string
	pid   int
	token string

	exitStatus int
	exited     time.Time
	stdin      io.WriteCloser
	stdout     io.Reader
	stderr     io.Reader
	stdio      Stdio
	path       string
	spec       specs.Process

	parent    *Init
	waitBlock chan struct{}

	sandbox vc.VCSandbox
}

func (e *ExecProcess) ID() string {
	return e.id
}

func (e *ExecProcess) Pid() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pid
}

func (e *ExecProcess) ExitStatus() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exitStatus
}

func (e *ExecProcess) ExitedAt() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exited
}

func (e *ExecProcess) Stdin() io.Closer {
	return e.stdin
}

func (e *ExecProcess) Stdio() Stdio {
	return e.stdio
}

func (e *ExecProcess) Status(ctx context.Context) (string, error) {
	s, err := e.parent.Status(ctx)
	if err != nil {
		return "", err
	}

	return s, nil
}

func (e *ExecProcess) Wait() {
	<-e.waitBlock
}

func (e *ExecProcess) resize(ws console.WinSize) error {
	sandbox := e.parent.sandbox
	err := sandbox.WinsizeProcess(sandbox.ID(), e.token, uint32(ws.Height), uint32(ws.Width))
	if err != nil {
		return errors.Wrap(err, "failed to resize")
	}
	return nil
}

func (e *ExecProcess) delete(ctx context.Context) error {
	return fmt.Errorf("exec process delete is not implemented")
}

func (e *ExecProcess) kill(ctx context.Context, sig uint32, _ bool) error {
	return fmt.Errorf("exec process kill is not implemented")
}

func (e *ExecProcess) setExited(status int) {
	e.exitStatus = status
	e.exited = time.Now()
	close(e.waitBlock)
}
