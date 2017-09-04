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

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/unikernel/metadata"
	"k8s.io/frakti/pkg/unikernel/metadata/store"
	"k8s.io/frakti/pkg/util/uuid"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// RunPodSandbox creates and starts a pod-level sandbox.
func (u *UnikernelRuntime) RunPodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	var err error
	// Genrate sandbox ID and name
	podID := uuid.NewUUID()
	podName := makeSandboxName(podID, config.GetMetadata())
	// Reserve sandbox name
	if err = u.sandboxNameIndex.Reserve(podName, podID); err != nil {
		return "", fmt.Errorf("failed to reserve sandbox name %q: %v", podName, err)
	}
	defer func() {
		if err != nil {
			u.sandboxNameIndex.ReleaseByName(podName)
		}
	}()
	// Reserve sandbox ID
	if err = u.sandboxIDIndex.Add(podID); err != nil {
		return "", fmt.Errorf("failed to reserve sandbox ID %q: %v", podID, err)
	}
	defer func() {
		if err != nil {
			u.sandboxIDIndex.Delete(podID)
		}
	}()

	// TODO(Crazykev): Get cpu/mem from cgroup
	vmMeta := &metadata.VMMetadata{
		CPUNum: u.defaultCPU,
		Memory: u.defaultMem,
	}

	// Create sandbox metadata.
	meta := metadata.SandboxMetadata{
		ID:       podID,
		Name:     podName,
		Config:   config,
		VMConfig: vmMeta,
		LogDir:   config.LogDirectory,
	}

	// TODO(Crazykev): Create ns and cni config

	// Add sandbox into sandbox metadata store.
	meta.CreatedAt = time.Now().UnixNano()
	meta.State = kubeapi.PodSandboxState_SANDBOX_READY
	if err = u.sandboxStore.Create(meta); err != nil {
		return "", fmt.Errorf("failed to add sandbox metadata %+v into store: %v", meta, err)
	}

	return podID, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (u *UnikernelRuntime) StopPodSandbox(podSandboxID string) error {
	sandbox, err := u.sandboxStore.Get(podSandboxID)
	if err != nil {
		return fmt.Errorf("failed to find sandbox(%q): %v", podSandboxID, err)
	}
	// Stop relate VM
	if err = u.vmTool.StopVM(sandbox.ID, 0); err != nil {
		return fmt.Errorf("failed to stop sandbox(%q): %v", sandbox.ID, err)
	}
	// Update sandbox metadata
	err = u.sandboxStore.Update(sandbox.ID, func(meta metadata.SandboxMetadata) (metadata.SandboxMetadata, error) {
		meta.State = kubeapi.PodSandboxState_SANDBOX_NOTREADY
		return meta, nil
	})

	return nil
}

// RemovePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (u *UnikernelRuntime) RemovePodSandbox(podSandboxID string) error {
	// Get sandbox and all containers in sandbox
	sandbox, err := u.sandboxStore.Get(podSandboxID)
	if err != nil {
		if err == store.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to find sandbox(%q): %v", podSandboxID, err)
	}
	ctrs, err := u.getAllContainersInPod(sandbox.ID)
	if err != nil {
		return fmt.Errorf("failed to get all containers for sandbox(%q): %v", podSandboxID, err)
	}

	if len(ctrs) > 1 {
		glog.Warningf("Get more than one(%d) containers in sandbox %q, remove them all", len(ctrs), sandbox.ID)
	}
	// Remove all containers found in sandbox, althrough we expected only one exist.
	for _, ctr := range ctrs {
		if err = u.RemoveContainer(ctr.ID); err != nil {
			return err
		}
	}
	if err = u.sandboxStore.Delete(sandbox.ID); err != nil {
		return fmt.Errorf("failed to delete sandbox(%q) metadata: %v", sandbox.ID, err)
	}

	return nil
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (u *UnikernelRuntime) PodSandboxStatus(podSandboxID string) (*kubeapi.PodSandboxStatus, error) {
	sandbox, err := u.sandboxStore.Get(podSandboxID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sandbox(%q): %v", podSandboxID, err)
	}

	// TODO(Crazykev): Fill in network status

	return toCRISandboxStatus(sandbox), nil
}

// ListPodSandbox returns a list of Sandbox.
func (u *UnikernelRuntime) ListPodSandbox(filter *kubeapi.PodSandboxFilter) ([]*kubeapi.PodSandbox, error) {
	glog.V(5).Infof("Unikernel: ListPodSandbox with filter %+v", filter)
	allSandboxes, err := u.sandboxStore.List()
	if err != nil {
		return nil, fmt.Errorf("list sandbox failed: %v", err)
	}
	var sandboxes []*kubeapi.PodSandbox
	for _, sbInStore := range allSandboxes {
		sandboxes = append(sandboxes, toCRISandbox(sbInStore))
	}
	sandboxes = u.filterCRISandboxes(sandboxes, filter)
	return sandboxes, nil
}

func makeSandboxName(podID string, meta *kubeapi.PodSandboxMetadata) string {
	return strings.Join([]string{
		meta.Name,
		meta.Namespace,
		meta.Uid,
		fmt.Sprintf("%d", meta.Attempt),
	}, "_")
}

func toCRISandbox(meta *metadata.SandboxMetadata) *kubeapi.PodSandbox {
	return &kubeapi.PodSandbox{
		Id:          meta.ID,
		Metadata:    meta.Config.GetMetadata(),
		State:       meta.State,
		CreatedAt:   meta.CreatedAt,
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
	}
}

func toCRISandboxStatus(meta *metadata.SandboxMetadata) *kubeapi.PodSandboxStatus {
	return &kubeapi.PodSandboxStatus{
		Id:          meta.ID,
		Metadata:    meta.Config.GetMetadata(),
		State:       meta.State,
		CreatedAt:   meta.CreatedAt,
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
	}
}

func (u *UnikernelRuntime) filterCRISandboxes(sandboxes []*kubeapi.PodSandbox, filter *kubeapi.PodSandboxFilter) []*kubeapi.PodSandbox {
	if filter == nil {
		return sandboxes
	}
	filtered := []*kubeapi.PodSandbox{}
	for _, s := range sandboxes {
		if filter.GetId() != "" && filter.GetId() != s.Id {
			continue
		}
		if filter.GetState() != nil && filter.GetState().GetState() != s.State {
			continue
		}
		if filter.GetLabelSelector() != nil {
			match := true
			for k, v := range filter.GetLabelSelector() {
				value, exist := s.Labels[k]
				if !exist || value != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	return filtered
}
