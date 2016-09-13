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

// CreatePodSandbox creates a pod-level sandbox.
func (h *Runtime) CreatePodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
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

	podStatus := &kubeapi.PodSandboxStatus{
		Id:          &podSandboxID,
		Metadata:    podSandboxMetadata,
		State:       &state,
		Network:     &kubeapi.PodSandboxNetworkStatus{Ip: &podIP},
		CreatedAt:   &info.CreatedAt,
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
			glog.Errorf("ParseSandboxName for %s failed: %v", pod.PodID, err)
			return nil, err
		}

		if filter != nil {
			if filter.Name != nil && podName != filter.GetName() {
				continue
			}

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

		items = append(items, &kubeapi.PodSandbox{
			Id:        &pod.PodID,
			Metadata:  podSandboxMetadata,
			Labels:    pod.Labels,
			State:     &state,
			CreatedAt: &pod.CreatedAt,
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

	_, _, _, containerName, attempt, err := parseContainerName(containerID)
	if err != nil {
		glog.Errorf("ParseContainerName for %s failed: %v", containerID, err)
		return nil, err
	}

	containerMetadata := &kubeapi.ContainerMetadata{
		Name:    &containerName,
		Attempt: &attempt,
	}

	kubeStatus := &kubeapi.ContainerStatus{
		Id:          &status.Container.ContainerID,
		Image:       &kubeapi.ImageSpec{Image: &status.Container.Image},
		ImageRef:    &status.Container.ImageID,
		Metadata:    containerMetadata,
		State:       &state,
		Labels:      kubeletLabels,
		Annotations: annotations,
		CreatedAt:   &status.CreatedAt,
	}

	mounts := make([]*kubeapi.Mount, len(status.Container.VolumeMounts))
	for idx, mnt := range status.Container.VolumeMounts {
		mounts[idx] = &kubeapi.Mount{
			Name:          &mnt.Name,
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

// Exec execute a command in the container.
func (h *Runtime) Exec(rawContainerID string, cmd []string, tty bool, stdin io.Reader, stdout, stderr io.WriteCloser) error {
	return fmt.Errorf("Not implemented")
}
