/*
Copyright 2016 The Kubernetes Authors.

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

package service

import (
	"fmt"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// CreateContainer creates a new container in specified PodSandbox
func (u *UnikernelRuntime) CreateContainer(podSandboxID string, config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// StartContainer starts the container.
func (u *UnikernelRuntime) StartContainer(rawContainerID string) error {
	return fmt.Errorf("not implemented")
}

// StopContainer stops a running container with a grace period (i.e. timeout).
func (u *UnikernelRuntime) StopContainer(rawContainerID string, timeout int64) error {
	return fmt.Errorf("not implemented")
}

// RemoveContainer removes the container. If the container is running, the container
// should be force removed.
func (u *UnikernelRuntime) RemoveContainer(rawContainerID string) error {
	return fmt.Errorf("not implemented")
}

// ListContainers lists all containers by filters.
func (u *UnikernelRuntime) ListContainers(filter *kubeapi.ContainerFilter) ([]*kubeapi.Container, error) {
	return nil, fmt.Errorf("not implemented")
}

// ContainerStatus returns the container status.
func (u *UnikernelRuntime) ContainerStatus(containerID string) (*kubeapi.ContainerStatus, error) {
	return nil, fmt.Errorf("not implemented")
}
