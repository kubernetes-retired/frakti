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

	"k8s.io/frakti/pkg/unikernel/metadata"
	"k8s.io/frakti/pkg/util/uuid"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// RunPodSandbox creates and starts a pod-level sandbox.
func (u *UnikernelRuntime) RunPodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	var err error
	// Genrate sandbox ID and name
	podID := uuid.NewUUID()
	podName := makeSandboxName(config.GetMetadata())
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

	// Create sandbox metadata.
	meta := metadata.SandboxMetadata{
		ID:     podID,
		Name:   podName,
		Config: config,
	}

	// TODO(Crazykev): Create ns and cni config

	// Add sandbox into sandbox metadata store.
	meta.CreatedAt = time.Now().UnixNano()
	if err = u.sandboxStore.Create(meta); err != nil {
		return "", fmt.Errorf("failed to add sandbox metadata %+v into store: %v", meta, err)
	}

	return podID, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (u *UnikernelRuntime) StopPodSandbox(podSandboxID string) error {
	return fmt.Errorf("not implemented")
}

// RemovePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (u *UnikernelRuntime) RemovePodSandbox(podSandboxID string) error {
	return fmt.Errorf("not implemented")
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (u *UnikernelRuntime) PodSandboxStatus(podSandboxID string) (*kubeapi.PodSandboxStatus, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListPodSandbox returns a list of Sandbox.
func (u *UnikernelRuntime) ListPodSandbox(filter *kubeapi.PodSandboxFilter) ([]*kubeapi.PodSandbox, error) {
	return nil, fmt.Errorf("not implemented")
}

func makeSandboxName(meta *kubeapi.PodSandboxMetadata) string {
	return strings.Join([]string{
		meta.Name,
		meta.Namespace,
		meta.Uid,
		fmt.Sprintf("%d", meta.Attempt),
	}, "_")
}
