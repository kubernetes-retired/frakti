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

	vc "github.com/kata-containers/runtime/virtcontainers"
)

type execProcess struct {
	wg sync.WaitGroup

	State

	mu         sync.Mutex
	id         string
	pid        int
	console    console.Console
	io         IO
	exitStatus int
	exited     time.Time
	closers    []io.Closer
	stdin      io.Closer
	stdio      Stdio
	path       string
	spec       specs.Process

	parent    *Init
	waitBlock chan struct{}

	sandbox vc.VCSandbox
}

func (e *execProcess) ID() string {
	return e.id
}

func (e *execProcess) Pid() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pid
}

func (e *execProcess) ExitStatus() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exitStatus
}

func (e *execProcess) ExitedAt() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exited
}

func (e *execProcess) Stdin() io.Closer {
	return e.stdin
}

func (e *execProcess) Stdio() Stdio {
	return e.stdio
}

func (e *execProcess) Status(ctx context.Context) (string, error) {
	return "", fmt.Errorf("exec process status is not implemented")
}

func (e *execProcess) Wait() {
	<-e.waitBlock
}

func (e *execProcess) resize(ws console.WinSize) error {
	if e.console == nil {
		return nil
	}
	return e.console.Resize(ws)
}

func (e *execProcess) start(ctx context.Context) error {
	return fmt.Errorf("exec process start is not implemented")
}

func (e *execProcess) delete(ctx context.Context) error {
	return fmt.Errorf("exec process delete is not implemented")
}

func (e *execProcess) kill(ctx context.Context, sig uint32, _ bool) error {
	return fmt.Errorf("exec process kill is not implemented")
}

func (e *execProcess) setExited(status int) {
	e.exitStatus = status
	e.exited = time.Now()
	e.parent.platform.ShutdownConsole(context.Background(), e.console)
	close(e.waitBlock)
}
