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
	"context"

	"github.com/containerd/console"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/runtime"
	vc "github.com/kata-containers/runtime/virtcontainers"
	errors "github.com/pkg/errors"

	"k8s.io/frakti/pkg/kata/proc"
)

// Process implements containerd.Process and containerd.State
type Process struct {
	id string
	t  *Task
}

// ID returns the process id
func (p *Process) ID() string {
	return p.id
}

// State returns the process state
func (p *Process) State(ctx context.Context) (runtime.State, error) {
	process := p.t.processList[p.t.id]
	state, err := process.Status(ctx)
	if err != nil {
		return runtime.State{}, errors.Wrap(err, "process state error")
	}

	var status runtime.Status
	switch state {
	case string(vc.StateReady):
		status = runtime.CreatedStatus
	case string(vc.StateRunning):
		status = runtime.RunningStatus
	case string(vc.StatePaused):
		status = runtime.PausedStatus
	case string(vc.StateStopped):
		status = runtime.StoppedStatus
	}

	stdio := process.Stdio()

	return runtime.State{
		Status:     status,
		Pid:        p.t.pid,
		Stdin:      stdio.Stdin,
		Stdout:     stdio.Stdout,
		Stderr:     stdio.Stderr,
		Terminal:   stdio.Terminal,
		ExitStatus: uint32(process.ExitStatus()),
		ExitedAt:   process.ExitedAt(),
	}, nil
}

// Kill signals a container
func (p *Process) Kill(ctx context.Context, signal uint32, _ bool) error {
	process := p.t.processList[p.t.id]
	err := process.Kill(ctx, signal, false)
	if err != nil {
		return errors.Wrap(err, "process kill error")
	}

	return nil
}

// ResizePty resizes the processes pty/console
func (p *Process) ResizePty(ctx context.Context, size runtime.ConsoleSize) error {
	ws := console.WinSize{
		Width:  uint16(size.Width),
		Height: uint16(size.Height),
	}

	process := p.t.processList[p.t.id]
	err := process.Resize(ws)
	if err != nil {
		return errors.Wrap(err, "process ResizePty error")
	}

	return nil
}

// CloseIO closes the processes stdin
func (p *Process) CloseIO(ctx context.Context) error {
	process := p.t.processList[p.t.id]
	if stdin := process.Stdin(); stdin != nil {
		if err := stdin.Close(); err != nil {
			return errors.Wrap(err, "process close stdin error")
		}
	}
	return nil
}

// Start the container's user defined process
func (p *Process) Start(ctx context.Context) error {
	process := p.t.processList[p.id]
	err := process.(*proc.Init).Start(ctx)
	if err != nil {
		return errors.Wrapf(err, "process start error")
	}

	p.t.events.Publish(ctx, runtime.TaskExecStartedEventTopic, &eventstypes.TaskExecStarted{
		ContainerID: p.t.id,
		Pid:         p.t.pid,
		ExecID:      p.id,
	})

	return nil
}

// Wait for the process to exit
func (p *Process) Wait(ctx context.Context) (*runtime.Exit, error) {
	init := p.t.processList[p.t.id]
	init.Wait()

	return &runtime.Exit{
		Timestamp: init.ExitedAt(),
		Status:    uint32(init.ExitStatus()),
	}, nil
}
