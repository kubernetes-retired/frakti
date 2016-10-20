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

package framework

import (
	. "github.com/onsi/ginkgo"
	"io"

	internalApi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/remote"
)

func LoadDefaultClient() (*FraktiClient, error) {
	rService, err := remote.NewRemoteRuntimeService(TestContext.RuntimeServiceAddr, TestContext.RuntimeServiceTimeout)
	if err != nil {
		return nil, err
	}

	iService, err := remote.NewRemoteImageService(TestContext.ImageServiceAddr, TestContext.ImageServiceTimeout)
	if err != nil {
		return nil, err
	}

	return &FraktiClient{
		runtimeService: rService,
		imageService:   iService,
	}, nil
}

type FraktiClient struct {
	runtimeService internalApi.RuntimeService
	imageService   internalApi.ImageManagerService
}

// Get version according to apiVersion
func (c *FraktiClient) Version(apiVersion string) (*runtimeApi.VersionResponse, error) {
	return c.runtimeService.Version(apiVersion)
}

// CreatePodSandbox creates and start a pod-level sandbox
func (c *FraktiClient) RunPodSandbox(config *runtimeApi.PodSandboxConfig) (string, error) {
	return c.runtimeService.RunPodSandbox(config)
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be forced to termination.
func (c *FraktiClient) StopPodSandbox(podSandboxID string) error {
	return c.runtimeService.StopPodSandbox(podSandboxID)
}

// RemovePodSandbox removes the sandbox. If there are running containers in the
// sandbox, they should be forcibly removed.
func (c *FraktiClient) RemovePodSandbox(podSandboxID string) error {
	return c.runtimeService.RemovePodSandbox(podSandboxID)
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (c *FraktiClient) PodSandboxStatus(podSandboxID string) (*runtimeApi.PodSandboxStatus, error) {
	return c.runtimeService.PodSandboxStatus(podSandboxID)
}

// ListPodSandbox returns a list of Sandbox.
func (c *FraktiClient) ListPodSandbox(filter *runtimeApi.PodSandboxFilter) ([]*runtimeApi.PodSandbox, error) {
	return c.runtimeService.ListPodSandbox(filter)
}

// CreateContainer creates a new container in specified PodSandbox.
func (c *FraktiClient) CreateContainer(podSandboxID string, config *runtimeApi.ContainerConfig, sandboxConfig *runtimeApi.PodSandboxConfig) (string, error) {
	return c.runtimeService.CreateContainer(podSandboxID, config, sandboxConfig)
}

// StartContainer starts the container.
func (c *FraktiClient) StartContainer(rawContainerID string) error {
	return c.runtimeService.StartContainer(rawContainerID)
}

// StopContainer stops a running container with a grace period (i.e., timeout).
func (c *FraktiClient) StopContainer(rawContainerID string, timeout int64) error {
	return c.runtimeService.StopContainer(rawContainerID, timeout)
}

// RemoveContainer removes the container.
func (c *FraktiClient) RemoveContainer(rawContainerID string) error {
	return c.runtimeService.RemoveContainer(rawContainerID)
}

// ListContainers lists all containers by filters.
func (c *FraktiClient) ListContainers(filter *runtimeApi.ContainerFilter) ([]*runtimeApi.Container, error) {
	return c.runtimeService.ListContainers(filter)
}

// ContainerStatus returns the status of the container.
func (c *FraktiClient) ContainerStatus(rawContainerID string) (*runtimeApi.ContainerStatus, error) {
	return c.runtimeService.ContainerStatus(rawContainerID)
}

// Exec executes a command in the container.
func (c *FraktiClient) Exec(rawContainerID string, cmd []string, tty bool, stdin io.Reader, stdout, stderr io.WriteCloser) error {
	return c.runtimeService.Exec(rawContainerID, cmd, tty, stdin, stdout, stderr)
}

// ListImages lists the existing images.
func (c *FraktiClient) ListImages(filter *runtimeApi.ImageFilter) ([]*runtimeApi.Image, error) {
	return c.imageService.ListImages(filter)
}

// ImageStatus returns the status of the image.
func (c *FraktiClient) ImageStatus(image *runtimeApi.ImageSpec) (*runtimeApi.Image, error) {
	return c.imageService.ImageStatus(image)
}

// PullImage pulls an image with the authentication config.
func (c *FraktiClient) PullImage(image *runtimeApi.ImageSpec, auth *runtimeApi.AuthConfig) error {
	return c.imageService.PullImage(image, auth)
}

// RemoveImage removes the image.
func (c *FraktiClient) RemoveImage(image *runtimeApi.ImageSpec) error {
	return c.imageService.RemoveImage(image)
}
