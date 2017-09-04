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

package metadata

import (
	"encoding/json"

	"k8s.io/frakti/pkg/unikernel/metadata/store"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// SandboxMetadata is the unversioned sandbox metadata.
type SandboxMetadata struct {
	// ID is the sandbox id.
	ID string
	// Name is the sandbox name.
	Name string
	// Config is the CRI sandbox config.
	Config *kubeapi.PodSandboxConfig
	// CreatedAt is the created timestamp.
	CreatedAt int64
	// NetConfig is the cni network config used by the sandbox.
	NetConfig []byte
	// VMConfig is the vm config.
	VMConfig *VMMetadata
	// State is CRI state of sandbox
	State kubeapi.PodSandboxState
	// LogDir is where sandbox's log stores.
	LogDir string
}

// VMMetadata is the vm metadata.
type VMMetadata struct {
	// CPUNum is the vcpu num of VM
	CPUNum int32
	// Memory is the size of memory in MB
	Memory int32
}

// SandboxUpdateFunc is the function used to update SandboxMetadata.
type SandboxUpdateFunc func(SandboxMetadata) (SandboxMetadata, error)

// sandboxToStoreUpdateFunc generates a metadata store UpdateFunc from SandboxUpdateFunc.
func sandboxToStoreUpdateFunc(u SandboxUpdateFunc) store.UpdateFunc {
	return func(data []byte) ([]byte, error) {
		meta := &SandboxMetadata{}
		if err := json.Unmarshal(data, meta); err != nil {
			return nil, err
		}
		newMeta, err := u(*meta)
		if err != nil {
			return nil, err
		}
		return json.Marshal(newMeta)
	}
}

// SandboxStore is the store for metadata of all sandboxes.
type SandboxStore interface {
	// Create creates a sandbox from SandboxMetadata in the store.
	Create(SandboxMetadata) error
	// Get gets the specified sandbox.
	Get(string) (*SandboxMetadata, error)
	// Update updates a specified sandbox.
	Update(string, SandboxUpdateFunc) error
	// List lists all sandboxes.
	List() ([]*SandboxMetadata, error)
	// Delete deletes the sandbox from the store.
	Delete(string) error
}

// sandboxStore is an implmentation of SandboxStore.
type sandboxStore struct {
	store store.MetadataStore
}

// NewSandboxStore creates a SandboxStore from a basic MetadataStore.
func NewSandboxStore(store store.MetadataStore) SandboxStore {
	return &sandboxStore{store: store}
}

// Create creates a sandbox from SandboxMetadata in the store.
func (s *sandboxStore) Create(metadata SandboxMetadata) error {
	data, err := json.Marshal(&metadata)
	if err != nil {
		return err
	}
	return s.store.Create(metadata.ID, data)
}

// Get gets the specified sandbox.
func (s *sandboxStore) Get(sandboxID string) (*SandboxMetadata, error) {
	data, err := s.store.Get(sandboxID)
	if err != nil {
		return nil, err
	}
	sandbox := &SandboxMetadata{}
	if err := json.Unmarshal(data, sandbox); err != nil {
		return nil, err
	}
	return sandbox, nil
}

// Update updates a specified sandbox. The function is running in a
// transaction. Update will not be applied when the update function
// returns error.
func (s *sandboxStore) Update(sandboxID string, u SandboxUpdateFunc) error {
	return s.store.Update(sandboxID, sandboxToStoreUpdateFunc(u))
}

// List lists all sandboxes.
func (s *sandboxStore) List() ([]*SandboxMetadata, error) {
	allData, err := s.store.List()
	if err != nil {
		return nil, err
	}
	var sandboxes []*SandboxMetadata
	for _, data := range allData {
		sandbox := &SandboxMetadata{}
		if err := json.Unmarshal(data, sandbox); err != nil {
			return nil, err
		}
		sandboxes = append(sandboxes, sandbox)
	}
	return sandboxes, nil
}

// Delete deletes the sandbox from the store.
func (s *sandboxStore) Delete(sandboxID string) error {
	return s.store.Delete(sandboxID)
}
