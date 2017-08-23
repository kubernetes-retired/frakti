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

package libvirt

import (
	"fmt"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

type VMTool struct {
	conn *LibvirtConnect
}

func NewVMTool(conn *LibvirtConnect) *VMTool {
	return &VMTool{
		conn: conn,
	}
}

type VMInfo struct {
	ID    string
	State kubeapi.ContainerState
}

// NOTE(Crazykev): This method may be changed when support multiple container per Pod.
// CreateContainer creates VM which contains container defined in container spec
func (vt *VMTool) CreateContainer(domainID string) error {
	return fmt.Errorf("not implemented")
}

// StartVM starts VM by domain UUID
func (vt *VMTool) StartVM(domainID string) error {
	return fmt.Errorf("not implemented")
}

// StopVM stops VM by domain UUID
func (vt *VMTool) StopVM(domainID string) error {
	return fmt.Errorf("not implemented")
}

// RemoveVM stops VM by domain UUID
func (vt *VMTool) RemoveVM(domainID string) error {
	return fmt.Errorf("not implemented")
}

// ListVMs list all known VMs managed by libvirt
func (vt *VMTool) ListVMs() ([]*VMInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetVMInfo get VM instance info by domain UUID
func (vt *VMTool) GetVMInfo(domainID string) (*VMInfo, error) {
	return nil, fmt.Errorf("not implemented")
}
