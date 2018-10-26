/*
Copyright 2017 The Kubernetes Authors.

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

package docker

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"time"
)

// Version returns the runtime name, runtime version, and runtime API version.
func (p *PrivilegedRuntime) Version(apiVersion string) (*kubeapi.VersionResponse, error) {
	return nil, fmt.Errorf("not implemented yet.")
}

// CreateContainer creates a new container in specified PodSandbox
func (p *PrivilegedRuntime) CreateContainer(podSandboxID string, config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (string, error) {

	request := &kubeapi.CreateContainerRequest{
		PodSandboxId:  podSandboxID,
		Config:        config,
		SandboxConfig: sandboxConfig,
	}

	logrus.Debugf("CreateContainerRequest: %v", request)
	r, err := runtimeClient.CreateContainer(context.Background(), request)
	logrus.Debugf("CreateContainerResponse: %v", r)
	if err != nil {
		return "", err
	}

	return r.ContainerId, nil
}

// StartContainer starts the container.
func (p *PrivilegedRuntime) StartContainer(containerID string) error {
	if containerID == "" {
		return fmt.Errorf("containerID cannot be empty")
	}
	request := &kubeapi.StartContainerRequest{
		ContainerId: containerID,
	}
	logrus.Debugf("StartContainerRequest: %v", request)
	r, err := runtimeClient.StartContainer(context.Background(), request)
	logrus.Debugf("StartContainerResponse: %v", r)
	if err != nil {
		return err
	}

	return nil
}

// StopContainer stops a running container with a grace period
func (p *PrivilegedRuntime) StopContainer(containerID string, timeout int64) error {
	if containerID == "" {
		return fmt.Errorf("containerID cannot be empty")
	}
	request := &kubeapi.StopContainerRequest{
		ContainerId: containerID,
		Timeout:     timeout,
	}
	logrus.Debugf("StopContainerRequest: %v", request)
	r, err := runtimeClient.StopContainer(context.Background(), request)
	if err != nil {
		return err
	}
	logrus.Debugf("StopContainerResponse: %v", r)

	return nil
}

// RemoveContainer removes the container.
func (p *PrivilegedRuntime) RemoveContainer(containerID string) error {
	if containerID == "" {
		return fmt.Errorf("containerID cannot be empty")
	}

	request := &kubeapi.RemoveContainerRequest{
		ContainerId: containerID,
	}

	r, err := runtimeClient.RemoveContainer(context.Background(), request)
	if err != nil {
		return err
	}
	logrus.Debugf("RemoveContainerResponse: %v", r)

	fmt.Println(containerID)
	return nil
}

// ListContainers lists all containers by filters.
func (p *PrivilegedRuntime) ListContainers(filter *kubeapi.ContainerFilter) ([]*kubeapi.Container, error) {
	request := &kubeapi.ListContainersRequest{
		Filter: filter,
	}

	logrus.Debugf("ListContainerRequest: %v", request)
	r, err := runtimeClient.ListContainers(context.Background(), request)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("ListContainerResponse: %v", r)

	return r.Containers, nil
}

// ContainerStatus returns status of the container.
func (p *PrivilegedRuntime) ContainerStatus(containerID string) (*kubeapi.ContainerStatus, error) {
	if containerID == "" {
		return nil, fmt.Errorf("containerID cannot be empty")
	}
	request := &kubeapi.ContainerStatusRequest{
		ContainerId: containerID,
	}
	logrus.Debugf("ContainerStatusRequest: %v", request)
	r, err := runtimeClient.ContainerStatus(context.Background(), request)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("ContainerStatusResponse: %v", r)

	return r.Status, nil
}

// UpdateContainerResources updates ContainerConfig of the container.
func (p *PrivilegedRuntime) UpdateContainerResources(containerID string, resources *kubeapi.LinuxContainerResources) error {
	if containerID == "" {
		return fmt.Errorf("containerID cannot be empty")
	}
	request := &kubeapi.UpdateContainerResourcesRequest{
		ContainerId: containerID,
		Linux: &kubeapi.LinuxContainerResources{
			CpusetCpus:         resources.CpusetCpus,
			CpusetMems:         resources.CpusetMems,
			MemoryLimitInBytes: resources.MemoryLimitInBytes,
			OomScoreAdj:        resources.OomScoreAdj,
		},
	}
	logrus.Debugf("UpdateContainerResourcesRequest: %v", request)
	r, err := runtimeClient.UpdateContainerResources(context.Background(), request)
	if err != nil {
		return err
	}
	logrus.Debugf("UpdateContainerResourcesResponse: %v", r)

	return nil
}

// ExecSync runs a command in a container synchronously.
func (p *PrivilegedRuntime) ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	request := &kubeapi.ExecSyncRequest{
		ContainerId: containerID,
		Cmd:         cmd,
		Timeout:     int64(timeout),
	}
	logrus.Debugf("ExecSyncRequest: %v", request)
	r, err := runtimeClient.ExecSync(context.Background(), request)
	logrus.Debugf("ExecSyncResponse: %v", r)
	if err != nil {
		return nil, nil, err
	}
	if r.ExitCode != 0 {
		fmt.Printf("Exit code: %v\n", r.ExitCode)
	}

	return r.Stdout, r.Stderr, nil
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (p *PrivilegedRuntime) Exec(*kubeapi.ExecRequest) (*kubeapi.ExecResponse, error) {
	return nil, fmt.Errorf("not implemented yet.")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (p *PrivilegedRuntime) Attach(req *kubeapi.AttachRequest) (*kubeapi.AttachResponse, error) {
	return nil, fmt.Errorf("not implemented yet.")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (p *PrivilegedRuntime) PortForward(*kubeapi.PortForwardRequest) (*kubeapi.PortForwardResponse, error) {
	return nil, fmt.Errorf("not implemented yet.")
}

// ReopenContainerLog asks runtime to reopen the stdout/stderr log file for the container.
func (p *PrivilegedRuntime) ReopenContainerLog(ContainerID string) error {
	request := &kubeapi.ReopenContainerLogRequest{ContainerId: ContainerID}

	logrus.Debugf("ReopenContainerLogRequest: %v", request)
	r, err := runtimeClient.ReopenContainerLog(context.Background(), request)
	logrus.Debugf("ReopenContainerLogResponse: %v", r)
	if err != nil {
		return err
	}

	return nil
}

// RunPodSandbox creates and starts a pod-level sandbox.
func (p *PrivilegedRuntime) RunPodSandbox(config *kubeapi.PodSandboxConfig, runtimeHandler string) (string, error) {
	request := &kubeapi.RunPodSandboxRequest{Config: config}
	logrus.Debugf("RunPodSandboxRequest: %v", request)
	r, err := runtimeClient.RunPodSandbox(context.Background(), request)
	logrus.Debugf("RunPodSandboxResponse: %v", r)
	if err != nil {
		return "", err
	}

	return r.PodSandboxId, nil
}

// StopPodSandbox stops any running process
func (p *PrivilegedRuntime) StopPodSandbox(podSandboxID string) error {
	if podSandboxID == "" {
		return fmt.Errorf("podSandboxID cannot be empty")
	}
	request := &kubeapi.StopPodSandboxRequest{PodSandboxId: podSandboxID}
	logrus.Debugf("StopPodSandboxRequest: %v", request)
	r, err := runtimeClient.StopPodSandbox(context.Background(), request)
	logrus.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return err
	}

	fmt.Printf("Stopped sandbox %s\n", podSandboxID)
	return nil

}

// RemovePodSandbox removes the sandbox.
func (p *PrivilegedRuntime) RemovePodSandbox(podSandboxID string) error {
	if podSandboxID == "" {
		return fmt.Errorf("podSandboxID cannot be empty")
	}
	request := &kubeapi.RemovePodSandboxRequest{PodSandboxId: podSandboxID}
	logrus.Debugf("RemovePodSandboxRequest: %v", request)
	r, err := runtimeClient.RemovePodSandbox(context.Background(), request)
	logrus.Debugf("RemovePodSandboxResponse: %v", r)
	if err != nil {
		return err
	}
	fmt.Printf("Removed sandbox %s\n", podSandboxID)
	return nil
}

// PodSandboxStatus returns the status of the PodSandbox.
func (p *PrivilegedRuntime) PodSandboxStatus(podSandboxID string) (*kubeapi.PodSandboxStatus, error) {
	if podSandboxID == "" {
		return nil, fmt.Errorf("podSandboxID cannot be empty")
	}
	request := &kubeapi.PodSandboxStatusRequest{
		PodSandboxId: podSandboxID,
	}

	logrus.Debugf("PodSandboxStatusRequest: %v", request)
	r, err := runtimeClient.PodSandboxStatus(context.Background(), request)
	logrus.Debugf("PodSandboxStatusResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Status, nil
}

// ListPodSandbox returns a list of PodSandboxes.
func (p *PrivilegedRuntime) ListPodSandbox(filter *kubeapi.PodSandboxFilter) ([]*kubeapi.PodSandbox, error) {
	request := &kubeapi.ListPodSandboxRequest{
		Filter: filter,
	}
	logrus.Debugf("ListPodSandboxRequest: %v", request)
	r, err := runtimeClient.ListPodSandbox(context.Background(), request)
	logrus.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Items, nil
}

// ContainerStats returns stats of the container.
func (p *PrivilegedRuntime) ContainerStats(containerID string) (*kubeapi.ContainerStats, error) {
	request := &kubeapi.ContainerStatsRequest{
		ContainerId: containerID,
	}
	logrus.Debugf("ListContainerStatsRequest: %v", request)
	r, err := runtimeClient.ContainerStats(context.Background(), request)
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Stats, nil
}

// ListContainerStats returns stats of all running containers.
func (p *PrivilegedRuntime) ListContainerStats(filter *kubeapi.ContainerStatsFilter) ([]*kubeapi.ContainerStats, error) {
	request := &kubeapi.ListContainerStatsRequest{
		Filter: filter,
	}

	logrus.Debugf("ListContainerStatsRequest: %v", request)
	r, err := runtimeClient.ListContainerStats(context.Background(), request)
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Stats, nil
}

// UpdateRuntimeConfig updates the runtime configuration based on the given request.
func (p *PrivilegedRuntime) UpdateRuntimeConfig(runtimeConfig *kubeapi.RuntimeConfig) error {
	request := &kubeapi.UpdateRuntimeConfigRequest{
		RuntimeConfig: runtimeConfig,
	}

	logrus.Debugf("UpdateRuntimeConfigRequest: %v", request)
	r, err := runtimeClient.UpdateRuntimeConfig(context.Background(), request)
	logrus.Debugf("UpdateRuntimeConfigResponse: %v", r)
	if err != nil {
		return err
	}

	return nil
}

// Status returns the status of the runtime.
func (p *PrivilegedRuntime) Status() (*kubeapi.RuntimeStatus, error) {
	request := &kubeapi.StatusRequest{
		Verbose: false,
	}
	logrus.Debugf("StatusRequest: %v", request)
	r, err := runtimeClient.Status(context.Background(), request)
	logrus.Debugf("StatusResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Status, nil
}
