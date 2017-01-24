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
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/golang/glog"
)

const (
	// FraktiRootDir is the root directory for Frakti
	FraktiRootDir = "/var/lib/frakti"
	// default directory to store pod sandbox checkpoint files
	sandboxCheckpointDir = "sandbox"
	ProtocolTCP          = Protocol("tcp")
	ProtocolUDP          = Protocol("udp")
	PortMappingsKey      = "PortMappings"
	schemaVersion        = "v1"
)

type Protocol string

// PortMapping is the port mapping configurations of a sandbox.
type PortMapping struct {
	// Protocol of the port mapping.
	Protocol *Protocol `json:"protocol,omitempty"`
	// Port number within the container.
	ContainerPort *int32 `json:"container_port,omitempty"`
	// Port number on the host.
	HostPort *int32 `json:"host_port,omitempty"`
}

// CheckpointData contains all types of data that can be stored in the checkpoint.
type CheckpointData struct {
	PortMappings []*PortMapping `json:"port_mappings,omitempty"`
}

// PodSandboxCheckpoint is the checkpoint structure for a sandbox
type PodSandboxCheckpoint struct {
	// Version of the pod sandbox checkpoint schema.
	Version string `json:"version"`
	// Pod name of the sandbox. Same as the pod name in the PodSpec.
	Name string `json:"name"`
	// Pod namespace of the sandbox. Same as the pod namespace in the PodSpec.
	Namespace string `json:"namespace"`
	// Pod netnspath of sandbox.
	NetNsPath string `json:"netnspath"`
	// Data to checkpoint for pod sandbox.
	Data *CheckpointData `json:"data,omitempty"`
}

// CheckpointHandler provides the interface to manage PodSandbox checkpoint
type CheckpointHandler interface {
	// CreateCheckpoint persists sandbox checkpoint in CheckpointStore.
	CreateCheckpoint(podSandboxID string, checkpoint *PodSandboxCheckpoint) error
	// GetCheckpoint retrieves sandbox checkpoint from CheckpointStore.
	GetCheckpoint(podSandboxID string) (*PodSandboxCheckpoint, error)
	// RemoveCheckpoint removes sandbox checkpoint form CheckpointStore.
	// WARNING: RemoveCheckpoint will not return error if checkpoint does not exist.
	RemoveCheckpoint(podSandboxID string) error
	// ListCheckpoint returns the list of existing checkpoints.
	ListCheckpoints() []string
}

// PersistentCheckpointHandler is an implementation of CheckpointHandler. It persists checkpoint in CheckpointStore
type PersistentCheckpointHandler struct {
	store CheckpointStore
}

func NewPersistentCheckpointHandler() CheckpointHandler {
	return &PersistentCheckpointHandler{store: &FileStore{path: filepath.Join(FraktiRootDir, sandboxCheckpointDir)}}
}

func (handler *PersistentCheckpointHandler) CreateCheckpoint(podSandboxID string, checkpoint *PodSandboxCheckpoint) error {
	blob, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return handler.store.Add(podSandboxID, blob)
}

func (handler *PersistentCheckpointHandler) GetCheckpoint(podSandboxID string) (*PodSandboxCheckpoint, error) {
	blob, err := handler.store.Get(podSandboxID)
	if err != nil {
		return nil, err
	}
	var checkpoint PodSandboxCheckpoint
	err = json.Unmarshal(blob, &checkpoint)
	return &checkpoint, err
}

func (handler *PersistentCheckpointHandler) RemoveCheckpoint(podSandboxID string) error {
	if err := handler.store.Delete(podSandboxID); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (handler *PersistentCheckpointHandler) ListCheckpoints() []string {
	keys, err := handler.store.List()
	if err != nil {
		glog.Errorf("Failed to list checkpoint store: %v", err)
		return []string{}
	}
	return keys
}

func NewPodSandboxCheckpoint(namespace, name string) *PodSandboxCheckpoint {
	return &PodSandboxCheckpoint{
		Version:   schemaVersion,
		Namespace: namespace,
		Name:      name,
		Data:      &CheckpointData{},
	}
}
