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
	"strings"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// CreateContainer creates a new container in specified PodSandbox
func (h *Runtime) CreateContainer(podSandboxID string, config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (string, error) {
	containerSpec, err := buildUserContainer(config, sandboxConfig)
	if err != nil {
		glog.Errorf("Build UserContainer for container %q failed: %v", config.String(), err)
		return "", err
	}

	// TODO: support container-level log_path in upstream hyperd when creating container.
	containerID, err := h.client.CreateContainer(podSandboxID, containerSpec)
	if err != nil {
		glog.Errorf("Create container %s in pod %s failed: %v", config.Metadata.GetName(), podSandboxID, err)
		return "", err
	}

	return containerID, nil
}

// buildUserContainer builds hyperd's UserContainer based kubelet ContainerConfig.
func buildUserContainer(config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (*types.UserContainer, error) {
	if config.GetLinux().GetSecurityContext().GetPrivileged() {
		return nil, fmt.Errorf("Priviledged containers are not supported in hyper")
	}

	containerSpec := &types.UserContainer{
		Name:       buildContainerName(sandboxConfig, config),
		Image:      config.Image.GetImage(),
		Workdir:    config.GetWorkingDir(),
		Tty:        config.GetTty(),
		Command:    config.GetArgs(),
		Entrypoint: config.GetCommand(),
		Labels:     buildLabelsWithAnnotations(config.Labels, config.Annotations),
	}

	// TODO: support adding device in upstream hyperd when creating container.

	// TODO: support volume mounts in upstream hyperd.
	// volumes := make([]*types.UserVolumeReference, len(config.Mounts))
	// for idx, v := range config.Mounts {
	// 	volumes[idx] = &types.UserVolumeReference{
	// 		Volume:   v.GetHostPath(),
	// 		Path:     v.GetContainerPath(),
	// 		ReadOnly: v.GetReadonly(),
	// 	}
	// }
	containerSpec.Volumes = []*types.UserVolumeReference{}

	// make environments
	environments := make([]*types.EnvironmentVar, len(config.Envs))
	for idx, env := range config.Envs {
		environments[idx] = &types.EnvironmentVar{
			Env:   env.GetKey(),
			Value: env.GetValue(),
		}
	}
	containerSpec.Envs = environments

	return containerSpec, nil
}

// StartContainer starts the container.
func (h *Runtime) StartContainer(rawContainerID string) error {
	// Hyperd doesn't support start a standalone container yet, restart the
	// pod to workaround this.
	// TODO: replace this with real StartContainer after hyperd's refactoring.
	container, err := h.client.GetContainerInfo(rawContainerID)
	if err != nil {
		glog.Errorf("Failed to get container %q: %v", rawContainerID, err)
		return err
	}
	_, reason, err := h.client.StopPod(container.PodID)
	if err != nil {
		glog.Errorf("[StartContainer] Failed to stop pod %q with reason (%q): %v", container.PodID, reason, err)
		return err
	}
	err = h.client.StartPod(container.PodID)
	if err != nil {
		glog.Errorf("[StartContainer] Failed to start pod %q: %v", container.PodID, err)
		return err
	}

	return nil
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
	// Workaround: always suppose container are deleted successfully.
	// TODO: remove container when hyperd is ready for this.
	return nil
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
		_, _, _, containerName, attempt, err := parseContainerName(strings.Replace(c.ContainerName, "/", "", -1))

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

	_, _, _, containerName, attempt, err := parseContainerName(strings.Replace(status.Container.Name, "/", "", -1))
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
