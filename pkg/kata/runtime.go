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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	types "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/events/exchange"
	identifiers "github.com/containerd/containerd/identifiers"
	log "github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/cri/pkg/annotations"
	"github.com/containerd/typeurl"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	errors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/frakti/pkg/kata/proc"
)

const (
	// RuntimeName is the name of new runtime
	RuntimeName = "kata-runtime"
)

var (
	pluginID = fmt.Sprintf("%s.%s", plugin.RuntimePlugin, RuntimeName)
)

// Runtime for kata containers
type Runtime struct {
	root    string
	state   string
	address string

	monitor runtime.TaskMonitor
	tasks   *runtime.TaskList
	db      *metadata.DB
	events  *exchange.Exchange
}

// New returns a new runtime
func New(ic *plugin.InitContext) (interface{}, error) {
	ic.Meta.Platforms = []ocispec.Platform{platforms.DefaultSpec()}

	if err := os.MkdirAll(ic.Root, 0711); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(ic.State, 0711); err != nil {
		return nil, err
	}
	monitor, err := ic.Get(plugin.TaskMonitorPlugin)
	if err != nil {
		return nil, err
	}
	m, err := ic.Get(plugin.MetadataPlugin)
	if err != nil {
		return nil, err
	}
	r := &Runtime{
		root:    ic.Root,
		state:   ic.State,
		address: ic.Address,

		monitor: monitor.(runtime.TaskMonitor),
		tasks:   runtime.NewTaskList(),
		db:      m.(*metadata.DB),
		events:  ic.Events,
	}

	tasks, err := r.restoreTasks(ic.Context)
	if err != nil {
		return nil, err
	}

	// TODO: need to add the tasks to the monitor
	for _, t := range tasks {
		if err := r.tasks.AddWithNamespace(t.namespace, t); err != nil {
			return nil, err
		}
	}

	log.G(ic.Context).Infoln("Runtime: start containerd-kata plugin")

	// TODO(ZeroMagic): reconnect the existing kata containers

	return r, nil
}

// ID returns ID of  kata-runtime.
func (r *Runtime) ID() string {
	return pluginID
}

// Create creates a task with the provided id and options.
func (r *Runtime) Create(ctx context.Context, id string, opts runtime.CreateOpts) (runtime.Task, error) {

	// 1. get namespace
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	log.G(ctx).Infof("Runtime: Namespace is %s\n", namespace)

	if err := identifiers.Validate(id); err != nil {
		return nil, errors.Wrapf(err, "invalid task id")
	}

	// 2. create bundle to store local image. Generate the rootfs dir and config.json
	bundle, err := newBundle(id,
		filepath.Join(r.state, namespace),
		filepath.Join(r.root, namespace),
		opts.Spec.Value)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			bundle.Delete()
		}
	}()

	// 3. get pid for vm. Now we use the specify pid.
	var pid uint32
	pid = 10244

	// 4. mount rootfs
	var eventRootfs []*types.Mount
	for _, m := range opts.Rootfs {
		eventRootfs = append(eventRootfs, &types.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
		})
	}

	// 5. With containerType, we can tell sandbox from container. In the future, we will use the variable.
	s, err := typeurl.UnmarshalAny(opts.Spec)
	if err != nil {
		return nil, err
	}
	spec := s.(*runtimespec.Spec)
	containerType := spec.Annotations[annotations.ContainerType]
	log.G(ctx).Infof("Runtime: %s ContainerType is %s", id, containerType)
	sandboxID := spec.Annotations[annotations.SandboxID]

	// 6. new task. Init the vm, sandbox, and necessary container.
	t, err := newTask(id, namespace, pid, r.monitor, r.events)
	if err != nil {
		return nil, err
	}
	t.containerType = containerType
	t.sandboxID = sandboxID

	// 7. create sandbox or container
	config := &proc.InitConfig{
		ID:            id,
		SandboxID:     sandboxID,
		ContainerType: containerType,
		Rootfs:        opts.Rootfs,
		Terminal:      opts.IO.Terminal,
		Stdin:         opts.IO.Stdin,
		Stdout:        opts.IO.Stdout,
		Stderr:        opts.IO.Stderr,
	}
	init, err := proc.NewInit(ctx, bundle.path, bundle.workDir, namespace, int(pid), config)
	if err != nil {
		return nil, errors.Wrap(err, "new init process error")
	}
	t.processList[id] = init

	// 8. add the task to taskList
	if err := r.tasks.Add(ctx, t); err != nil {
		return nil, err
	}

	// 9. publish create event
	r.events.Publish(ctx, runtime.TaskCreateEventTopic, &eventstypes.TaskCreate{
		ContainerID: id,
		Bundle:      bundle.path,
		Rootfs:      eventRootfs,
		IO: &eventstypes.TaskIO{
			Stdin:    opts.IO.Stdin,
			Stdout:   opts.IO.Stdout,
			Stderr:   opts.IO.Stderr,
			Terminal: opts.IO.Terminal,
		},
		Checkpoint: opts.Checkpoint,
		Pid:        t.pid,
	})

	return t, nil
}

// Get a specific task by task id.
func (r *Runtime) Get(ctx context.Context, id string) (runtime.Task, error) {
	return r.tasks.Get(ctx, id)
}

// Tasks returns all the current tasks for the runtime.
func (r *Runtime) Tasks(ctx context.Context) ([]runtime.Task, error) {
	return r.tasks.GetAll(ctx)
}

// Delete removes the task in the runtime.
func (r *Runtime) Delete(ctx context.Context, t runtime.Task) (*runtime.Exit, error) {

	// monitor will be handled

	taskID := t.ID()
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	bundle := loadBundle(
		taskID,
		filepath.Join(r.state, namespace, taskID),
		filepath.Join(r.root, namespace, taskID),
	)

	// unmount
	if err := mount.Unmount(filepath.Join(bundle.path, "rootfs"), 0); err != nil {
		log.G(ctx).WithError(err).WithFields(logrus.Fields{
			"path": bundle.path,
			"id":   taskID,
		}).Warnf("unmount task rootfs")
	}

	// delete process
	p := t.(*Task).GetProcess(taskID)
	if err := p.Delete(ctx); err != nil {
		return nil, err
	}

	// remove the task
	r.tasks.Delete(ctx, taskID)

	// delete the bundle
	if err := bundle.Delete(); err != nil {
		log.G(ctx).WithError(err).Error("failed to delete bundle")
	}

	r.events.Publish(ctx, runtime.TaskDeleteEventTopic, &eventstypes.TaskDelete{
		ContainerID: taskID,
		Pid:         uint32(10244),
		ExitStatus:  128 + uint32(unix.SIGKILL),
		ExitedAt:    time.Now(),
	})

	return &runtime.Exit{
		Pid:       uint32(p.Pid()),
		Status:    uint32(p.ExitStatus()),
		Timestamp: time.Now(),
	}, nil
}

func (r *Runtime) restoreTasks(ctx context.Context) ([]*Task, error) {
	dir, err := ioutil.ReadDir(r.state)
	if err != nil {
		return nil, err
	}
	var o []*Task
	for _, namespace := range dir {
		if !namespace.IsDir() {
			continue
		}
		name := namespace.Name()
		log.G(ctx).WithField("namespace", name).Debug("loading tasks in namespace")
		tasks, err := r.loadTasks(ctx, name)
		if err != nil {
			return nil, err
		}
		o = append(o, tasks...)
	}
	return o, nil
}

func (r *Runtime) loadTasks(ctx context.Context, ns string) ([]*Task, error) {
	dir, err := ioutil.ReadDir(filepath.Join(r.state, ns))
	if err != nil {
		return nil, err
	}
	var o []*Task
	for _, path := range dir {
		if !path.IsDir() {
			continue
		}
		id := path.Name()
		bundle := loadBundle(
			id,
			filepath.Join(r.state, ns, id),
			filepath.Join(r.root, ns, id),
		)
		pidByte, _ := ioutil.ReadFile(filepath.Join(bundle.path, proc.InitPidFile))
		pidStr := string(pidByte)
		pid64, err := strconv.ParseInt(pidStr, 10, 32)
		pid := uint32(pid64)

		ctx = namespaces.WithNamespace(ctx, ns)

		t, err := newTask(id, ns, pid, r.monitor, r.events)
		if err != nil {
			log.G(ctx).WithError(err).Error("loading task type")
			continue
		}
		o = append(o, t)
	}
	return o, nil
}
