/*
Copyright 2018 The Kubernetes Authors.

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

package proc

import (
	mount "github.com/containerd/containerd/mount"
	"github.com/gogo/protobuf/types"
)

const (
	// ErrContainerType represents the specific container type which does not exist
	ErrContainerType = "the containerType does not exist"
)

// InitConfig hold task creation configuration
type InitConfig struct {
	ID            string
	SandboxID     string
	ContainerType string
	Runtime       string
	Rootfs        []mount.Mount
	Terminal      bool
	Stdin         string
	Stdout        string
	Stderr        string
}

// ExecConfig holds exec creation configuration
type ExecConfig struct {
	ID       string
	Terminal bool
	Stdin    string
	Stdout   string
	Stderr   string
	Spec     *types.Any
}

// CheckpointConfig holds task checkpoint configuration
type CheckpointConfig struct {
	Path    string
	Options *types.Any
}
