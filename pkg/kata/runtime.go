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
	"os"
	"path/filepath"

	eventstypes "github.com/containerd/containerd/api/events"
	types "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/events/exchange"
	identifiers "github.com/containerd/containerd/identifiers"
	log "github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/cri/pkg/annotations"
	"github.com/containerd/typeurl"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	errors "github.com/pkg/errors"
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
	pidPool *pidPool

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
		pidPool: newPidPool(),

		monitor: monitor.(runtime.TaskMonitor),
		tasks:   runtime.NewTaskList(),
		db:      m.(*metadata.DB),
		events:  ic.Events,
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
	log.G(ctx).Infof("Runtime: ContainerType is %s\n", containerType)

	// 6. new task. Init the vm, sandbox, and necessary container.
	t, err := newTask(ctx, id, namespace, pid, r.monitor, r.events, opts, bundle)
	if err != nil {
		return nil, err
	}

	if err := r.tasks.Add(ctx, t); err != nil {
		return nil, err
	}
	// 7. after the task is created, add it to the monitor if it has a cgroup
	// this can be different on a checkpoint/restore
	if t.cg != nil {
		if err = r.monitor.Monitor(t); err != nil {
			if _, err := r.Delete(ctx, t); err != nil {
				log.G(ctx).WithError(err).Error("deleting task after failed monitor")
			}
			return nil, err
		}
	}

	// 8. publish create event
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

	// TODO(ZeroMagic): delete a task

	return nil, fmt.Errorf("not implemented")
}
