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

package hyper

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"
	"github.com/hyperhq/hyperd/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	hyperRuntimeName    = "hyper"
	minimumHyperVersion = "0.6.0"

	// timeout in second for interacting with hyperd's gRPC API.
	hyperConnectionTimeout = 300 * time.Second

	//timeout in second for creating context with timeout.
	hyperContextTimeout = 15 * time.Second

	//response code of PodRemove, when the pod can not be found.
	E_NOT_FOUND = -2
)

// Runtime is the HyperContainer implementation of kubelet runtime API
type Runtime struct {
	client *Client
}

// NewHyperRuntime creates a new Runtime
func NewHyperRuntime(hyperEndpoint string) (*Runtime, error) {
	hyperClient, err := NewClient(hyperEndpoint, hyperConnectionTimeout)
	if err != nil {
		glog.Fatalf("Initialize hyper client failed: %v", err)
		return nil, err
	}

	return &Runtime{client: hyperClient}, nil
}

// Version returns the runtime name, runtime version and runtime API version
func (h *Runtime) Version() (string, string, string, error) {
	return hyperRuntimeName, "", "", fmt.Errorf("Not implemented")
}

// CreatePodSandbox creates a pod-level sandbox.
func (h *Runtime) CreatePodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	return "", fmt.Errorf("Not implemented")
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (h *Runtime) StopPodSandbox(podSandboxID string) error {
	request := types.PodStopRequest{
		PodID: podSandboxID,
	}

	cxt, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	response, err := h.client.client.PodStop(cxt, &request)

	if err != nil {
		return fmt.Errorf("Stop pod %s failed, code: %d, cause: %s, error: %v", podSandboxID, response.Code, response.Cause, err)
	}

	return nil
}

// DeletePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (h *Runtime) DeletePodSandbox(podSandboxID string) error {
	request := types.PodRemoveRequest{
		PodID: podSandboxID,
	}

	cxt, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	response, err := h.client.client.PodRemove(cxt, &request)
	if response.Code == E_NOT_FOUND {
		return nil
	}

	if err != nil {
		return fmt.Errorf("Delete pod error: %v", err)
	}

	return nil
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (h *Runtime) PodSandboxStatus(podSandboxID string) (*kubeapi.PodSandboxStatus, error) {
	return nil, fmt.Errorf("Not implemented")
}

// ListPodSandbox returns a list of Sandbox.
func (h *Runtime) ListPodSandbox(filter *kubeapi.PodSandboxFilter) ([]*kubeapi.PodSandbox, error) {
	return nil, fmt.Errorf("Not implemented")
}

// CreateContainer creates a new container in specified PodSandbox
func (h *Runtime) CreateContainer(podSandboxID string, config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (string, error) {
	return "", fmt.Errorf("Not implemented")
}

// StartContainer starts the container.
func (h *Runtime) StartContainer(rawContainerID string) error {
	return fmt.Errorf("Not implemented")
}

// StopContainer stops a running container with a grace period (i.e. timeout).
func (h *Runtime) StopContainer(rawContainerID string, timeout int64) error {
	return fmt.Errorf("Not implemented")
}

// RemoveContainer removes the container. If the container is running, the container
// should be force removed.
func (h *Runtime) RemoveContainer(rawContainerID string) error {
	return fmt.Errorf("Not implemented")
}

// ListContainers lists all containers by filters.
func (h *Runtime) ListContainers(filter *kubeapi.ContainerFilter) ([]*kubeapi.Container, error) {
	return nil, fmt.Errorf("Not implemented")
}

// ContainerStatus returns the container status.
func (h *Runtime) ContainerStatus(containerID string) (*kubeapi.ContainerStatus, error) {
	return nil, fmt.Errorf("Not implemented")
}

// Exec execute a command in the container.
func (h *Runtime) Exec(rawContainerID string, cmd []string, tty bool, stdin io.Reader, stdout, stderr io.WriteCloser) error {
	return fmt.Errorf("Not implemented")
}

// ListImages lists existing images.
func (h *Runtime) ListImages(filter *kubeapi.ImageFilter) ([]*kubeapi.Image, error) {
	return nil, fmt.Errorf("Not implemented")
}

// ImageStatus returns the status of the image.
func (h *Runtime) ImageStatus(image *kubeapi.ImageSpec) (*kubeapi.Image, error) {
	return nil, fmt.Errorf("Not implemented")
}

// PullImage pulls a image with authentication config.
func (h *Runtime) PullImage(image *kubeapi.ImageSpec, authConfig *kubeapi.AuthConfig) error {
	return fmt.Errorf("Not implemented")
}

// RemoveImage removes the image.
func (h *Runtime) RemoveImage(image *kubeapi.ImageSpec) error {
	return fmt.Errorf("Not implemented")
}
