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
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	hyperRuntimeName    = "hyper"
	minimumHyperVersion = "0.6.0"
	secondToNano        = 1e9

	// timeout in second for interacting with hyperd's gRPC API.
	hyperConnectionTimeout = 300 * time.Second
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
	version, apiVersion, err := h.client.GetVersion()
	if err != nil {
		glog.Errorf("Get hyper version failed: %v", err)
		return "", "", "", err
	}

	return hyperRuntimeName, version, apiVersion, nil
}

// RunPodSandbox creates and starts a pod-level sandbox.
func (h *Runtime) RunPodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	err := updatePodSandboxConfig(config)
	if err != nil {
		glog.Errorf("Update PodSandbox config failed: %v", err)
		return "", err
	}
	userpod, err := buildUserPod(config)
	if err != nil {
		glog.Errorf("Build UserPod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	podID, err := h.client.CreatePod(userpod)
	if err != nil {
		glog.Errorf("Create pod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	err = h.client.StartPod(podID)
	if err != nil {
		glog.Errorf("Start pod %q failed: %v", podID, err)
		if removeError := h.client.RemovePod(podID); removeError != nil {
			glog.Warningf("Remove pod %q failed: %v", removeError)
		}
		return "", err
	}

	return podID, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (h *Runtime) StopPodSandbox(podSandboxID string) error {
	code, cause, err := h.client.StopPod(podSandboxID)
	if err != nil {
		glog.Errorf("Stop pod %s failed, code: %d, cause: %s, error: %v", podSandboxID, code, cause, err)
		return err
	}

	return nil
}

// DeletePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (h *Runtime) DeletePodSandbox(podSandboxID string) error {
	err := h.client.RemovePod(podSandboxID)
	if err != nil {
		glog.Errorf("Remove pod %s failed: %v", podSandboxID, err)
		return err
	}

	return nil
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (h *Runtime) PodSandboxStatus(podSandboxID string) (*kubeapi.PodSandboxStatus, error) {
	info, err := h.client.GetPodInfo(podSandboxID)
	if err != nil {
		glog.Errorf("GetPodInfo for %s failed: %v", podSandboxID, err)
		return nil, err
	}

	state := toPodSandboxState(info.Status.Phase)
	podIP := ""
	if len(info.Status.PodIP) > 0 {
		podIP = info.Status.PodIP[0]
	}

	podName, podNamespace, podUID, attempt, err := parseSandboxName(info.PodName)
	if err != nil {
		glog.Errorf("ParseSandboxName for %s failed: %v", info.PodName, err)
		return nil, err
	}

	podSandboxMetadata := &kubeapi.PodSandboxMetadata{
		Name:      &podName,
		Uid:       &podUID,
		Namespace: &podNamespace,
		Attempt:   &attempt,
	}

	annotations := getAnnotationsFromLabels(info.Spec.Labels)
	kubeletLabels := getKubeletLabels(info.Spec.Labels)
	createdAtNano := info.CreatedAt * secondToNano
	podStatus := &kubeapi.PodSandboxStatus{
		Id:          &podSandboxID,
		Metadata:    podSandboxMetadata,
		State:       &state,
		Network:     &kubeapi.PodSandboxNetworkStatus{Ip: &podIP},
		CreatedAt:   &createdAtNano,
		Labels:      kubeletLabels,
		Annotations: annotations,
	}

	return podStatus, nil
}

// ListPodSandbox returns a list of Sandbox.
func (h *Runtime) ListPodSandbox(filter *kubeapi.PodSandboxFilter) ([]*kubeapi.PodSandbox, error) {
	pods, err := h.client.GetPodList()
	if err != nil {
		glog.Errorf("GetPodList failed: %v", err)
		return nil, err
	}

	items := make([]*kubeapi.PodSandbox, 0, len(pods))
	for _, pod := range pods {
		state := toPodSandboxState(pod.Status)

		podName, podNamespace, podUID, attempt, err := parseSandboxName(pod.PodName)
		if err != nil {
			glog.Errorf("ParseSandboxName for %s failed: %v", pod.PodName, err)
			return nil, err
		}

		if filter != nil {
			if filter.Id != nil && pod.PodID != filter.GetId() {
				continue
			}

			if filter.State != nil && state != filter.GetState() {
				continue
			}

			if filter.LabelSelector != nil && !inMap(filter.LabelSelector, pod.Labels) {
				continue
			}
		}

		podSandboxMetadata := &kubeapi.PodSandboxMetadata{
			Name:      &podName,
			Uid:       &podUID,
			Namespace: &podNamespace,
			Attempt:   &attempt,
		}

		createdAtNano := pod.CreatedAt * secondToNano
		items = append(items, &kubeapi.PodSandbox{
			Id:        &pod.PodID,
			Metadata:  podSandboxMetadata,
			Labels:    pod.Labels,
			State:     &state,
			CreatedAt: &createdAtNano,
		})
	}

	sortByCreatedAt(items)

	return items, nil
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
	err := h.client.StopContainer(rawContainerID, timeout)
	if err != nil {
		glog.Errorf("Stop container %s failed: %v", rawContainerID, err)
		return err
	}

	return nil
}

// RemoveContainer removes the container. If the container is running, the container
// should be force removed.
func (h *Runtime) RemoveContainer(rawContainerID string) error {
	return fmt.Errorf("Not implemented")
}

// ListContainers lists all containers by filters.
func (h *Runtime) ListContainers(filter *kubeapi.ContainerFilter) ([]*kubeapi.Container, error) {
	containerList, err := h.client.GetContainerList(false)
	if err != nil {
		glog.Errorf("Get container list failed: %v", err)
		return nil, err
	}

	containers := make([]*kubeapi.Container, 0, len(containerList))

	for _, c := range containerList {
		state := toKubeContainerState(c.Status)
		_, _, _, containerName, attempt, err := parseContainerName(c.ContainerName)

		if err != nil {
			glog.Errorf("ParseContainerName for %s failed: %v", c.ContainerName, err)
			return nil, err
		}

		if filter != nil {
			if filter.Id != nil && c.ContainerID != filter.GetId() {
				continue
			}

			if filter.PodSandboxId != nil && c.PodID != filter.GetPodSandboxId() {
				continue
			}

			if filter.State != nil && state != filter.GetState() {
				continue
			}
		}

		info, err := h.client.GetContainerInfo(c.ContainerID)
		if err != nil {
			glog.Errorf("Get container info for %s failed: %v", c.ContainerID, err)
			return nil, err
		}

		annotations := getAnnotationsFromLabels(info.Container.Labels)
		kubeletLabels := getKubeletLabels(info.Container.Labels)

		if filter != nil {
			if filter.LabelSelector != nil && !inMap(filter.LabelSelector, kubeletLabels) {
				continue
			}
		}

		containerMetadata := &kubeapi.ContainerMetadata{
			Name:    &containerName,
			Attempt: &attempt,
		}

		createdAtNano := info.CreatedAt * secondToNano
		containers = append(containers, &kubeapi.Container{
			Id:           &c.ContainerID,
			PodSandboxId: &c.PodID,
			CreatedAt:    &createdAtNano,
			Metadata:     containerMetadata,
			Image:        &kubeapi.ImageSpec{Image: &info.Container.Image},
			ImageRef:     &info.Container.ImageID,
			State:        &state,
			Labels:       kubeletLabels,
			Annotations:  annotations,
		})
	}

	return containers, nil
}

// ContainerStatus returns the container status.
func (h *Runtime) ContainerStatus(containerID string) (*kubeapi.ContainerStatus, error) {
	status, err := h.client.GetContainerInfo(containerID)
	if err != nil {
		glog.Errorf("Get container info for %s failed: %v", containerID, err)
		return nil, err
	}

	podInfo, err := h.client.GetPodInfo(status.PodID)
	if err != nil {
		glog.Errorf("Get pod info for %s failed: %v", status.PodID, err)
		return nil, err
	}

	state := toKubeContainerState(status.Status.Phase)
	annotations := getAnnotationsFromLabels(status.Container.Labels)
	kubeletLabels := getKubeletLabels(status.Container.Labels)

	_, _, _, containerName, attempt, err := parseContainerName(status.Container.Name)
	if err != nil {
		glog.Errorf("ParseContainerName for %s failed: %v", status.Container.Name, err)
		return nil, err
	}

	containerMetadata := &kubeapi.ContainerMetadata{
		Name:    &containerName,
		Attempt: &attempt,
	}

	createdAtNano := status.CreatedAt * secondToNano
	kubeStatus := &kubeapi.ContainerStatus{
		Id:          &status.Container.ContainerID,
		Image:       &kubeapi.ImageSpec{Image: &status.Container.Image},
		ImageRef:    &status.Container.ImageID,
		Metadata:    containerMetadata,
		State:       &state,
		Labels:      kubeletLabels,
		Annotations: annotations,
		CreatedAt:   &createdAtNano,
	}

	mounts := make([]*kubeapi.Mount, len(status.Container.VolumeMounts))
	for idx, mnt := range status.Container.VolumeMounts {
		mounts[idx] = &kubeapi.Mount{
			ContainerPath: &mnt.MountPath,
			Readonly:      &mnt.ReadOnly,
		}

		for _, v := range podInfo.Spec.Volumes {
			if v.Name == mnt.Name {
				mounts[idx].HostPath = &v.Source
			}
		}
	}
	kubeStatus.Mounts = mounts

	switch status.Status.Phase {
	case "running":
		startedAt, err := parseTimeString(status.Status.Running.StartedAt)
		if err != nil {
			glog.Errorf("Hyper: can't parse startedAt %s", status.Status.Running.StartedAt)
			return nil, err
		}
		kubeStatus.StartedAt = &startedAt
	case "failed", "succeeded":
		startedAt, err := parseTimeString(status.Status.Terminated.StartedAt)
		if err != nil {
			glog.Errorf("Hyper: can't parse startedAt %s", status.Status.Terminated.StartedAt)
			return nil, err
		}
		finishedAt, err := parseTimeString(status.Status.Terminated.FinishedAt)
		if err != nil {
			glog.Errorf("Hyper: can't parse finishedAt %s", status.Status.Terminated.FinishedAt)
			return nil, err
		}

		kubeStatus.StartedAt = &startedAt
		kubeStatus.FinishedAt = &finishedAt
		kubeStatus.Reason = &status.Status.Terminated.Reason
		kubeStatus.ExitCode = &status.Status.Terminated.ExitCode
	default:
		kubeStatus.Reason = &status.Status.Waiting.Reason
	}

	return kubeStatus, nil
}

// ExecSync runs a command in a container synchronously.
func (h *Runtime) ExecSync() error {
	return fmt.Errorf("Not implemented")
}

// Exec execute a command in the container.
func (h *Runtime) Exec(rawContainerID string, cmd []string, tty bool, stdin io.Reader, stdout, stderr io.WriteCloser) error {
	return fmt.Errorf("Not implemented")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (h *Runtime) Attach() error {
	return fmt.Errorf("Not implemented")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (h *Runtime) PortForward() error {
	return fmt.Errorf("Not implemented")
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (h *Runtime) UpdateRuntimeConfig(runtimeConfig *kubeapi.RuntimeConfig) error {
	return nil
}
