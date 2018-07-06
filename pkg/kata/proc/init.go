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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"k8s.io/frakti/pkg/kata/server"

	vc "github.com/kata-containers/runtime/virtcontainers"
)

// InitPidFile name of the file that contains the init pid
const InitPidFile = "init.pid"

// Init represents an initial process for a container
type Init struct {
	wg sync.WaitGroup
	initState
	mu sync.Mutex

	waitBlock chan struct{}

	workDir string

	id     string
	bundle string

	exitStatus int
	exited     time.Time
	pid        int
	stdin      io.WriteCloser
	stdout     io.Reader
	stderr     io.Reader
	stdio      Stdio
	rootfs     string
	IoUID      int
	IoGID      int

	sandbox vc.VCSandbox
}

// NewInit returns a new init process
func NewInit(ctx context.Context, path, workDir, namespace string, pid int, config *InitConfig) (*Init, error) {
	var (
		success bool
		err     error
	)

	rootfs := filepath.Join(path, "rootfs")
	defer func() {
		if success {
			return
		}
		if err2 := mount.UnmountAll(rootfs, 0); err2 != nil {
			log.G(ctx).WithError(err2).Warn("Failed to cleanup rootfs mount")
		}
	}()

	for _, rm := range config.Rootfs {
		m := &mount.Mount{
			Type:    rm.Type,
			Source:  rm.Source,
			Options: rm.Options,
		}
		if err := m.Mount(rootfs); err != nil {
			return nil, errors.Wrapf(err, "failed to mount rootfs component %v", m)
		}
	}

	p := &Init{
		id:  config.ID,
		pid: pid,
		stdio: Stdio{
			Stdin:    config.Stdin,
			Stdout:   config.Stdout,
			Stderr:   config.Stderr,
			Terminal: config.Terminal,
		},
		rootfs:     rootfs,
		bundle:     path,
		workDir:    workDir,
		exitStatus: 0,
		waitBlock:  make(chan struct{}),
		IoUID:      os.Getuid(),
		IoGID:      os.Getuid(),
	}
	p.initState = &createdState{p: p}

	// create kata container
	p.sandbox, err = server.CreateSandbox(ctx, config.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create sandbox")
	}

	stdin, stdout, stderr, err := p.sandbox.IOStream(config.ID, config.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a container's stdio streams from kata")
	}
	p.stdin = stdin
	p.stdout = stdout
	p.stderr = stderr

	// TODO(ZeroMagic): create with checkpoint

	success = true
	return p, nil
}

// ID of the process
func (p *Init) ID() string {
	return p.id
}

// Pid of the process
func (p *Init) Pid() int {
	return p.pid
}

// ExitStatus of the process
func (p *Init) ExitStatus() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitStatus
}

// ExitedAt at time when the process exited
func (p *Init) ExitedAt() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

// Stdin of the process
func (p *Init) Stdin() io.Closer {
	return p.stdin
}

// Stdio of the process
func (p *Init) Stdio() Stdio {
	return p.stdio
}

// Status of the process
func (p *Init) Status(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	status, err := server.StatusContainer(p.sandbox.ID(), p.sandbox.ID())
	if err != nil {
		if os.IsNotExist(err) {
			return "stopped", nil
		}
		return "", errors.Wrap(err, "OCI runtime state failed")
	}
	return string(status.State.State), nil
}

// Wait for the process to exit
func (p *Init) Wait() {
	<-p.waitBlock
}

func (p *Init) resize(ws console.WinSize) error {
	return p.sandbox.WinsizeProcess(p.sandbox.ID(), p.id, uint32(ws.Height), uint32(ws.Width))
}

func (p *Init) start(ctx context.Context) error {
	err := server.StartSandbox(ctx, p.sandbox.ID())
	if err != nil {
		return errors.Wrap(err, "failed to start sandbox")
	}

	return nil
}

func (p *Init) delete(ctx context.Context) error {
	return fmt.Errorf("init process delete is not implemented")
}

func (p *Init) kill(ctx context.Context, signal uint32, all bool) error {

	err := server.KillContainer(p.sandbox.ID(), p.sandbox.ID(), syscall.Signal(signal), all)
	if err != nil {
		return errors.Wrap(err, "failed to kill container")
	}

	return nil
}

func (p *Init) setExited(status int) {
	p.exited = time.Now()
	p.exitStatus = status
	close(p.waitBlock)
}

// Metrics return the stats of a container
func (p *Init) Metrics(ctx context.Context) (vc.ContainerStats, error) {
	stats, err := p.sandbox.StatsContainer(p.sandbox.ID())
	if err != nil {
		return vc.ContainerStats{}, errors.Wrap(err, "failed to get the stats of a container")
	}

	return stats, nil
}

func (p *Init) pause(ctx context.Context) error {
	err := p.sandbox.Pause()
	if err != nil {
		return errors.Wrap(err, "failed to pause container")
	}
	return nil
}

func (p *Init) resume(ctx context.Context) error {
	err := p.sandbox.Resume()
	if err != nil {
		return errors.Wrap(err, "failed to resume container")
	}
	return nil
}

// exec returns a new exec'd process
func (p *Init) exec(context context.Context, id string, conf *ExecConfig) (Process, error) {
	var spec specs.Process
	if err := json.Unmarshal(conf.Spec.Value, &spec); err != nil {
		return nil, err
	}
	spec.Terminal = conf.Terminal

	capabilities := vc.LinuxCapabilities{
		Bounding:    spec.Capabilities.Bounding,
		Effective:   spec.Capabilities.Effective,
		Inheritable: spec.Capabilities.Inheritable,
		Permitted:   spec.Capabilities.Permitted,
		Ambient:     spec.Capabilities.Ambient,
	}

	cmd := vc.Cmd{
		Args:            spec.Args,
		Envs:            []vc.EnvVar{},
		User:            string(spec.User.UID),
		PrimaryGroup:    string(spec.User.GID),
		WorkDir:         spec.Cwd,
		Capabilities:    capabilities,
		Interactive:     spec.Terminal,
		Detach:          !spec.Terminal,
		NoNewPrivileges: spec.NoNewPrivileges,
	}

	_, process, err := p.sandbox.EnterContainer(p.sandbox.ID(), cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot enter container %s", p.sandbox.ID())
	}

	stdin, stdout, stderr, err := p.sandbox.IOStream(p.sandbox.ID(), process.Token)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get %s IOStream", p.sandbox.ID())
	}

	e := &ExecProcess{
		id:     conf.ID,
		pid:    process.Pid,
		token:  process.Token,
		parent: p,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		stdio: Stdio{
			Stdin:    conf.Stdin,
			Stdout:   conf.Stdout,
			Stderr:   conf.Stderr,
			Terminal: conf.Terminal,
		},
		spec:      spec,
		waitBlock: make(chan struct{}),
	}
	e.State = &execCreatedState{p: e}
	return e, nil
}
