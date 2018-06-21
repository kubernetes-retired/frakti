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

package server

import (
	"context"
	"fmt"
	"syscall"

	"github.com/containerd/containerd/runtime"
	vc "github.com/kata-containers/runtime/virtcontainers"
	errors "github.com/pkg/errors"
)

// CreateContainer creates a kata-runtime container
func CreateContainer(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("create container not implemented")
}

// StartContainer starts a kata-runtime container
func StartContainer(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("start container not implemented")
}

// StopContainer stops a kata-runtime container
func StopContainer(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("stop container not implemented")
}

// DeleteContainer deletes a kata-runtime container
func DeleteContainer(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("delete container not implemented")
}

// KillContainer kills one or more kata-runtime containers
func KillContainer(sandboxID, containerID string, signal syscall.Signal, all bool) error {
	err := vc.KillContainer(sandboxID, containerID, signal, all)
	if err != nil {
		return errors.Wrapf(err, "Could not kill container")
	}

	return nil
}

// StatusContainer returns the virtcontainers container status.
func StatusContainer(sandboxID, containerID string) (vc.ContainerStatus, error) {
	status, err := vc.StatusContainer(sandboxID, containerID)
	if err != nil {
		return vc.ContainerStatus{}, errors.Wrapf(err, "Could not kill container")
	}

	return status, nil
}
