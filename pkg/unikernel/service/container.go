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
	"strings"
	"time"

	"github.com/docker/docker/pkg/stringid"
	"github.com/golang/glog"
	"k8s.io/frakti/pkg/unikernel/libvirt"
	"k8s.io/frakti/pkg/unikernel/metadata"
	metaimage "k8s.io/frakti/pkg/unikernel/metadata/image"
	"k8s.io/frakti/pkg/unikernel/metadata/store"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// CreateContainer creates a new container in specified PodSandbox
func (u *UnikernelRuntime) CreateContainer(podSandboxID string, config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (string, error) {
	var err error
	// Check if there is any pod already created in pod
	// We now only support one-container-per-pod
	createdContainers, err := u.getAllContainersInPod(podSandboxID)
	if err != nil {
		return "", fmt.Errorf("get containers for pod(%q) failed: %v", podSandboxID, err)
	}
	for _, container := range createdContainers {
		glog.Warningf("Unikernel/CreateContainer: container(%q) already exist in pod(%q), remove it", container.ID, podSandboxID)
		if err := u.RemoveContainer(container.ID); err != nil {
			glog.Errorf("Clean up legacy container(%q) failed: %v", container.ID, err)
		}
	}

	sandbox, err := u.sandboxStore.Get(podSandboxID)
	if err != nil {
		return "", fmt.Errorf("failed to get sandbox(%q) from store: %v", podSandboxID, err)
	}

	cid := generateID()
	cName := makeContainerName(config.GetMetadata(), sandboxConfig.GetMetadata())
	if err = u.containerNameIndex.Reserve(cName, cid); err != nil {
		return "", fmt.Errorf("reserve container name %q failed: %v", cName, err)
	}
	defer func() {
		if err != nil {
			u.containerNameIndex.ReleaseByName(cName)
		}
	}()

	// Create internal container metadata
	meta := metadata.ContainerMetadata{
		ID:        cid,
		Name:      cName,
		SandboxID: podSandboxID,
		Config:    config,
		LogPath:   config.LogPath,
	}

	// Prepare container image
	imageRef := config.GetImage().GetImage()
	if _, err = u.imageManager.GetImageInfo(imageRef); err != nil {
		if metadata.IsNotExistError(err) {
			return "", fmt.Errorf("image %q not found", imageRef)
		}
		return "", fmt.Errorf("failed to get image %q: %v", imageRef, err)
	}
	storage, err := u.imageManager.PrepareImage(imageRef, podSandboxID)
	if err != nil {
		glog.Errorf("Failed to prepare image %q for sandbox %q: %v", imageRef, podSandboxID, err)
		return "", fmt.Errorf("prepare image failed: %v", err)
	}
	defer func() {
		if err != nil {
			err1 := u.imageManager.CleanupImageCopy(imageRef, podSandboxID)
			if err1 != nil {
				glog.Errorf("Failed to cleanup image copy when create container failed: %v", err1)
			}
		}
	}()

	// TODO(Crazykev): Support it!
	if storage.Format != metaimage.QCOW2 {
		err = fmt.Errorf("only support qcow2 image format for now")
		return "", err
	}
	meta.ImageRef = storage.ImageFile

	// Create container in VM, for now we actually create VM
	err = u.vmTool.CreateContainer(&meta, sandbox)
	if err != nil {
		return "", fmt.Errorf("failed to create containerd container: %v", err)
	}
	defer func() {
		// Note: For now, we just remove VM directly if create container(VM) failed.
		if err != nil {
			if err1 := u.vmTool.RemoveVM(podSandboxID); err1 != nil {
				glog.Errorf("Failed to clean up failed VM container: %v", err1)
			}
		}
	}()

	meta.CreatedAt = time.Now().UnixNano()
	if err = u.containerStore.Create(meta); err != nil {
		return "", fmt.Errorf("failed to add container metadata %+v into meta store: %v", meta, err)
	}

	return cid, nil
}

// StartContainer starts the container.
func (u *UnikernelRuntime) StartContainer(rawContainerID string) error {
	container, err := u.containerStore.Get(rawContainerID)
	if err != nil {
		return fmt.Errorf("failed to find container with ID(%q): %v", rawContainerID, err)
	}

	// Start related vm
	err = u.vmTool.StartVM(container.SandboxID)
	if err != nil {
		return fmt.Errorf("failed to start VM(%q): %v", container.SandboxID, err)
	}

	// Update container state
	err = u.containerStore.Update(container.ID, func(meta metadata.ContainerMetadata) (metadata.ContainerMetadata, error) {
		meta.StartedAt = time.Now().UnixNano()
		return meta, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update conatiner(%q) metadata: %v", container.ID, err)
	}
	return nil
}

// StopContainer stops a running container with a grace period (i.e. timeout).
func (u *UnikernelRuntime) StopContainer(rawContainerID string, timeout int64) error {
	container, err := u.containerStore.Get(rawContainerID)
	if err != nil {
		return fmt.Errorf("failed to find container with ID(%q): %v", rawContainerID, err)
	}
	// Stop related VM
	err = u.vmTool.StopVM(container.SandboxID, timeout)
	if err != nil {
		return fmt.Errorf("failed to stop container related vm(%q): %v", container.SandboxID, err)
	}
	// Update container state
	err = u.containerStore.Update(container.ID, func(meta metadata.ContainerMetadata) (metadata.ContainerMetadata, error) {
		meta.FinishedAt = time.Now().UnixNano()
		return meta, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update conatiner(%q) metadata: %v", container.ID, err)
	}
	return nil
}

// RemoveContainer removes the container. If the container is running, the container
// should be force removed.
func (u *UnikernelRuntime) RemoveContainer(rawContainerID string) error {
	container, err := u.containerStore.Get(rawContainerID)
	if err != nil {
		if err == store.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to find container with ID(%q): %v", rawContainerID, err)
	}
	// Remove related VM
	err = u.vmTool.RemoveVM(container.SandboxID)
	if err != nil {
		return fmt.Errorf("failed to remove container related vm(%q): %v", container.SandboxID, err)
	}
	// Cleanup image copy
	if err = u.imageManager.CleanupImageCopy(container.Config.GetImage().GetImage(), container.SandboxID); err != nil {
		glog.Errorf("Failed to cleanup image copy after remove container: %v", err)
		return err
	}
	// Release name and id
	u.containerNameIndex.ReleaseByName(container.Name)
	// Remove container metadata
	if err = u.containerStore.Delete(container.ID); err != nil {
		return fmt.Errorf("failed to delete conatiner(%q) metadata: %v", container.ID, err)
	}

	return nil
}

// ListContainers lists all containers by filters.
func (u *UnikernelRuntime) ListContainers(filter *kubeapi.ContainerFilter) ([]*kubeapi.Container, error) {
	glog.V(5).Infof("Unikernel: ListContainers with filter %+v", filter)
	// Get containers in store
	ctrInStore, err := u.containerStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list container in store: %v", err)
	}
	// Get VMs managed by libvirt
	vmMap, err := u.vmTool.ListVMs()
	if err != nil {
		return nil, err
	}
	glog.V(5).Infof("List all VMs in libvirt: %+v", vmMap)
	// Update container state from libvirt
	var containers []*kubeapi.Container
	for _, container := range ctrInStore {
		vm, exist := vmMap[container.SandboxID]
		if !exist {
			glog.Warningf("Find sandbox(%q) and container(%q) does not have related VM", container.SandboxID, container.ID)
			// FIXME(Crazykev): Should we remove these container and sandbox?
			continue
		}
		curState := getCRIContainerStateFromVMState(vm.State, container.State())
		containers = append(containers, toCRIContainer(container, curState))
	}

	// Filter containers
	containers = u.filterCRIContainers(containers, filter)
	return containers, nil
}

// ContainerStatus returns the container status.
func (u *UnikernelRuntime) ContainerStatus(rawContainerID string) (*kubeapi.ContainerStatus, error) {
	container, err := u.containerStore.Get(rawContainerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find container with ID(%q): %v", rawContainerID, err)
	}
	// Get related VM info
	vmInfo, err := u.vmTool.GetVMInfo(container.SandboxID)
	if err != nil {
		return nil, err
	}
	lastState := container.State()
	curState := getCRIContainerStateFromVMState(vmInfo.State, lastState)
	if err = u.transformContainerState(container, curState, lastState); err != nil {
		return nil, err
	}

	return &kubeapi.ContainerStatus{
		Id:          container.ID,
		Metadata:    container.Config.GetMetadata(),
		State:       curState,
		CreatedAt:   container.CreatedAt,
		StartedAt:   container.StartedAt,
		FinishedAt:  container.FinishedAt,
		Image:       container.Config.GetImage(),
		ImageRef:    container.ImageRef,
		Labels:      container.Config.GetLabels(),
		Annotations: container.Config.GetAnnotations(),
	}, nil
}

// transformContainerState update container state according to container state transformation.
func (u *UnikernelRuntime) transformContainerState(meta *metadata.ContainerMetadata, curState, lastState kubeapi.ContainerState) error {
	if curState == lastState {
		return nil
	}
	if curState != kubeapi.ContainerState_CONTAINER_EXITED {
		glog.Warningf("Unexpected container(%s) state transform from %d to %d", meta.ID, lastState, curState)
	}
	err := u.containerStore.Update(meta.ID, func(meta metadata.ContainerMetadata) (metadata.ContainerMetadata, error) {
		meta.FinishedAt = time.Now().UnixNano()
		return meta, nil
	})
	if err != nil {
		return fmt.Errorf("failed to update container(%s) state: %v", meta.ID, err)
	}
	return nil
}

// getCRIContainerStateFromVMState get CRI container state from last container state and current vm state
func getCRIContainerStateFromVMState(vmState libvirt.DomainState, lastState kubeapi.ContainerState) kubeapi.ContainerState {
	switch vmState {
	case libvirt.DOMAIN_SHUTDOWN:
		// shutdown means between running and shutoff, that is still running
		fallthrough
	case libvirt.DOMAIN_RUNNING:
		return kubeapi.ContainerState_CONTAINER_RUNNING
	case libvirt.DOMAIN_SHUTOFF:
		if lastState == kubeapi.ContainerState_CONTAINER_CREATED {
			return kubeapi.ContainerState_CONTAINER_CREATED
		}
		return kubeapi.ContainerState_CONTAINER_EXITED
	case libvirt.DOMAIN_CRASHED:
		return kubeapi.ContainerState_CONTAINER_EXITED
	case libvirt.DOMAIN_PMSUSPENDED:
		return kubeapi.ContainerState_CONTAINER_EXITED
	case libvirt.DOMAIN_PAUSED:
		if lastState == kubeapi.ContainerState_CONTAINER_CREATED {
			return kubeapi.ContainerState_CONTAINER_CREATED
		}
		return kubeapi.ContainerState_CONTAINER_EXITED
	default:
		return kubeapi.ContainerState_CONTAINER_UNKNOWN
	}
}

// toCRIContainer get CRI defined container from ContainerMetadata
func toCRIContainer(meta *metadata.ContainerMetadata, state kubeapi.ContainerState) *kubeapi.Container {
	return &kubeapi.Container{
		Id:           meta.ID,
		PodSandboxId: meta.SandboxID,
		Metadata:     meta.Config.GetMetadata(),
		Image:        meta.Config.GetImage(),
		ImageRef:     meta.ImageRef,
		State:        state,
		CreatedAt:    meta.CreatedAt,
		Labels:       meta.Config.GetLabels(),
		Annotations:  meta.Config.GetAnnotations(),
	}
}

func (u *UnikernelRuntime) filterCRIContainers(containers []*kubeapi.Container, filter *kubeapi.ContainerFilter) []*kubeapi.Container {
	if filter == nil {
		return containers
	}

	result := []*kubeapi.Container{}
	for _, ctr := range containers {
		if filter.GetId() != "" && filter.GetId() != ctr.Id {
			continue
		}
		if filter.GetPodSandboxId() != "" && filter.GetPodSandboxId() != ctr.PodSandboxId {
			continue
		}
		if filter.GetState() != nil && filter.GetState().GetState() != ctr.State {
			continue
		}
		if filter.GetLabelSelector() != nil {
			match := true
			for k, v := range filter.GetLabelSelector() {
				value, exist := ctr.Labels[k]
				if !exist || value != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, ctr)
	}

	return result
}

func (u *UnikernelRuntime) getAllContainersInPod(podID string) ([]*metadata.ContainerMetadata, error) {
	containersInStore, err := u.containerStore.List()
	if err != nil {
		return nil, err
	}
	var containers []*metadata.ContainerMetadata
	for _, container := range containersInStore {
		if container.SandboxID == podID {
			containers = append(containers, container)
		}
	}
	return containers, nil
}

// generateID generates a random unique id.
func generateID() string {
	return stringid.GenerateNonCryptoID()
}

func makeContainerName(c *kubeapi.ContainerMetadata, s *kubeapi.PodSandboxMetadata) string {
	return strings.Join([]string{
		c.Name,
		s.Name,
		s.Namespace,
		s.Uid,
		fmt.Sprintf("%d", c.Attempt),
	}, "_")
}

//  UpdateContainerResources updates the resource constraints for the container.
func (h *UnikernelRuntime) UpdateContainerResources(
	rawContainerID string,
	config *kubeapi.LinuxContainerResources,
) error {
	return fmt.Errorf("UpdateContainerResources is not implemented yet.")
}

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (h *UnikernelRuntime) ContainerStats(containerID string) (*kubeapi.ContainerStats, error) {
	return nil, fmt.Errorf("ContainerStats is not implemented yet.")
}

// ListContainerStats returns stats of all running containers.
func (h *UnikernelRuntime) ListContainerStats(filter *kubeapi.ContainerStatsFilter) (
	[]*kubeapi.ContainerStats, error) {
	return nil, fmt.Errorf("ContainerStats is not implemented yet.")
}
