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
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/hyper/types"
	"k8s.io/frakti/pkg/util/knownflags"
	utilmetadata "k8s.io/frakti/pkg/util/metadata"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	volDriver = "vfs"
)

// CreateContainer creates a new container in specified PodSandbox
func (h *Runtime) CreateContainer(podSandboxID string, config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (string, error) {
	containerSpec, err := buildUserContainer(config, sandboxConfig)
	if err != nil {
		glog.Errorf("Build UserContainer for container %q failed: %v", config.String(), err)
		return "", err
	}

	containerID, err := h.client.CreateContainer(podSandboxID, containerSpec)
	if err != nil {
		glog.Errorf("Create container %s in pod %s failed: %v", config.Metadata.Name, podSandboxID, err)
		return "", err
	}

	return containerID, nil
}

// VolumeOptsData is the struct of json file
type VolumeOptsData struct {
	AccessMode   string   `json:"access_mode"`
	AuthUserName string   `json:"auth_username"`
	AuthEnabled  bool     `json:"auth_enabled"`
	ClusterName  string   `json:"cluster_name"`
	Encrypted    bool     `json:"encrypted"`
	FsType       string   `json:"fsType"`
	Hosts        []string `json:"hosts"`
	Keyring      string   `json:"keyring"`
	Name         string   `json:"name"`
	Ports        []string `json:"ports"`
	SecretUUID   string   `json:"secret_uuid"`
	SecretType   string   `json:"secret_type"`
	VolumeID     string   `json:"volumeID"`
	VolumeType   string   `json:"volume_type"`
}

// buildUserContainer builds hyperd's UserContainer based kubelet ContainerConfig.
func buildUserContainer(config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (*types.UserContainer, error) {
	privilege := false
	readonlyRootfs := false
	if securityContext := config.GetLinux().GetSecurityContext(); securityContext != nil {
		privilege = securityContext.Privileged
		readonlyRootfs = securityContext.ReadonlyRootfs
	}

	if privilege {
		return nil, fmt.Errorf("Privileged containers are not supported in hyper")
	}

	logPath := filepath.Join(sandboxConfig.LogDirectory, config.LogPath)
	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}
	config.Labels[containerLogPathLabelKey] = logPath
	containerSpec := &types.UserContainer{
		Name:       buildContainerName(sandboxConfig, config),
		Image:      config.GetImage().Image,
		Workdir:    config.WorkingDir,
		Tty:        config.Tty,
		Command:    config.Args,
		Entrypoint: config.Command,
		Labels:     buildLabelsWithAnnotations(config.Labels, config.Annotations),
		LogPath:    logPath,
		ReadOnly:   readonlyRootfs,
	}

	// make volumes
	volumes, err := makeContainerVolumes(config)
	if err != nil {
		return nil, err
	}
	containerSpec.Volumes = volumes

	// make environments
	environments := make([]*types.EnvironmentVar, len(config.Envs))
	for idx, env := range config.Envs {
		environments[idx] = &types.EnvironmentVar{
			Env:   env.Key,
			Value: env.Value,
		}
	}
	containerSpec.Envs = environments

	return containerSpec, nil
}

func makeContainerVolumes(config *kubeapi.ContainerConfig) ([]*types.UserVolumeReference, error) {
	volumes := make([]*types.UserVolumeReference, len(config.Mounts))
	for i, m := range config.Mounts {
		hostPath := m.HostPath

		_, volName := filepath.Split(hostPath)

		// In frakti, we can both use normal container volumes (-v host:path), and also cinder-flexvolume
		isCinderFlexvolume := false
		volumeOptsFile := filepath.Join(hostPath, knownflags.FlexvolumeDataFile)

		// no-exist hostPath is allowed, and that case should never be cinder flexvolume
		if hostPathInfo, err := os.Stat(hostPath); !os.IsNotExist(err) {
			// 1. host path is a directory (filter out bind mounted files like /etc/hosts)
			if hostPathInfo.IsDir() {
				// 2. tag file exists in host path
				if _, err := os.Stat(volumeOptsFile); !os.IsNotExist(err) {
					// 3. then this is a cinder-flexvolume
					isCinderFlexvolume = true
				}
			}
		}

		if isCinderFlexvolume {
			// this is a cinder-flexvolume
			optsData := VolumeOptsData{}
			if err := utilmetadata.ReadJson(volumeOptsFile, &optsData); err != nil {
				return nil, fmt.Errorf(
					"buildUserContainer() failed: can't read flexvolume data file in %q: %v",
					hostPath, err,
				)
			}

			if optsData.VolumeType == "rbd" {
				monitors := make([]string, 0, 1)
				for _, host := range optsData.Hosts {
					for _, port := range optsData.Ports {
						monitors = append(monitors, fmt.Sprintf("%s:%s", host, port))
					}
				}
				volDetail := &types.UserVolume{
					Name: volName + fmt.Sprintf("_%08x", rand.Uint32()),
					// kuberuntime will set HostPath to the abs path of volume directory on host
					Source: "rbd:" + optsData.Name,
					Format: optsData.VolumeType,
					Fstype: optsData.FsType,
					Option: &types.UserVolumeOption{
						User:     optsData.AuthUserName,
						Keyring:  optsData.Keyring,
						Monitors: monitors,
					},
				}
				volumes[i] = &types.UserVolumeReference{
					// use the generated volume name above
					Volume:   volDetail.Name,
					Path:     m.ContainerPath,
					ReadOnly: m.Readonly,
					Detail:   volDetail,
				}
			}
		} else {
			// this is a normal volume
			volDetail := &types.UserVolume{
				Name: volName + fmt.Sprintf("_%08x", rand.Uint32()),
				// kuberuntime will set HostPath to the abs path of volume directory on host
				Source: hostPath,
				Format: volDriver,
			}
			volumes[i] = &types.UserVolumeReference{
				// use the generated volume name above
				Volume:   volDetail.Name,
				Path:     m.ContainerPath,
				ReadOnly: m.Readonly,
				Detail:   volDetail,
			}
		}
	}
	return volumes, nil
}

// StartContainer starts the container.
func (h *Runtime) StartContainer(rawContainerID string) error {
	err := h.client.StartContainer(rawContainerID)
	if err != nil {
		glog.Errorf("Start container %q failed: %v", rawContainerID, err)
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
	err := h.client.RemoveContainer(rawContainerID)
	if err != nil {
		glog.Errorf("Remove container %q failed: %v", rawContainerID, err)
		return err
	}

	return nil
}

// ListContainers lists all containers by filters.
func (h *Runtime) ListContainers(filter *kubeapi.ContainerFilter) ([]*kubeapi.Container, error) {
	containerList, err := h.client.GetContainerList()
	if err != nil {
		glog.Errorf("Get container list failed: %v", err)
		return nil, err
	}

	containers := make([]*kubeapi.Container, 0, len(containerList))

	for _, c := range containerList {
		state := toKubeContainerState(c.Status)
		_, _, _, containerName, attempt, err := parseContainerName(strings.Replace(c.ContainerName, "/", "", -1))

		if err != nil {
			glog.V(3).Infof("ParseContainerName for %q failed (%v), assuming it is not managed by frakti", c.ContainerName, err)
			continue
		}

		if filter != nil {
			if filter.Id != "" && c.ContainerID != filter.Id {
				continue
			}

			if filter.PodSandboxId != "" && c.PodID != filter.PodSandboxId {
				continue
			}

			if filter.State != nil && state != filter.GetState().State {
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
			Name:    containerName,
			Attempt: attempt,
		}

		createdAtNano := info.CreatedAt * secondToNano
		containers = append(containers, &kubeapi.Container{
			Id:           c.ContainerID,
			PodSandboxId: c.PodID,
			CreatedAt:    createdAtNano,
			Metadata:     containerMetadata,
			Image:        &kubeapi.ImageSpec{Image: info.Container.Image},
			ImageRef:     info.Container.ImageID,
			State:        state,
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

	logPath := status.Container.Labels[containerLogPathLabelKey]
	state := toKubeContainerState(status.Status.Phase)
	annotations := getAnnotationsFromLabels(status.Container.Labels)
	kubeletLabels := getKubeletLabels(status.Container.Labels)

	_, _, _, containerName, attempt, err := parseContainerName(strings.Replace(status.Container.Name, "/", "", -1))
	if err != nil {
		glog.Errorf("ParseContainerName for %s failed: %v", status.Container.Name, err)
		return nil, err
	}

	containerMetadata := &kubeapi.ContainerMetadata{
		Name:    containerName,
		Attempt: attempt,
	}

	createdAtNano := status.CreatedAt * secondToNano
	kubeStatus := &kubeapi.ContainerStatus{
		Id:          status.Container.ContainerID,
		Image:       &kubeapi.ImageSpec{Image: status.Container.Image},
		ImageRef:    status.Container.ImageID,
		Metadata:    containerMetadata,
		State:       state,
		Labels:      kubeletLabels,
		Annotations: annotations,
		CreatedAt:   createdAtNano,
		LogPath:     logPath,
	}

	mounts := make([]*kubeapi.Mount, len(status.Container.VolumeMounts))
	for idx, mnt := range status.Container.VolumeMounts {
		mounts[idx] = &kubeapi.Mount{
			ContainerPath: mnt.MountPath,
			Readonly:      mnt.ReadOnly,
		}

		for _, v := range podInfo.Spec.Volumes {
			if v.Name == mnt.Name {
				mounts[idx].HostPath = v.Source
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
		kubeStatus.StartedAt = startedAt
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

		kubeStatus.StartedAt = startedAt
		kubeStatus.FinishedAt = finishedAt
		kubeStatus.Reason = status.Status.Terminated.Reason
		kubeStatus.ExitCode = status.Status.Terminated.ExitCode
	default:
		kubeStatus.Reason = status.Status.Waiting.Reason
	}

	return kubeStatus, nil
}

//  UpdateContainerResources updates the resource constraints for the container.
func (h *Runtime) UpdateContainerResources(
	rawContainerID string,
	config *kubeapi.LinuxContainerResources,
) error {
	// TODO(harry): I would suggest to run container with cpuset in docker, but we can not decide which Pod
	// has cpuset configured. It's tricky.
	// Also, we can not throw error here since kubelet will always execute cm.updateContainerCPUSet() by internal
	// container life cycle hook.
	// Will talk with @connor to see if this can be fixed.
	return nil
}

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (h *Runtime) ContainerStats(containerID string) (*kubeapi.ContainerStats, error) {
	return nil, fmt.Errorf("ContainerStats is not implemented for hyper runtime yet.")
}

// ListContainerStats returns stats of all running containers.
func (h *Runtime) ListContainerStats(filter *kubeapi.ContainerStatsFilter) (
	[]*kubeapi.ContainerStats, error) {
	return nil, fmt.Errorf("ContainerStats is not implemented for hyper runtime yet.")
}
