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

package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	"github.com/docker/docker/pkg/truncindex"
	"k8s.io/frakti/pkg/unikernel/image"
	"k8s.io/frakti/pkg/unikernel/libvirt"
	"k8s.io/frakti/pkg/unikernel/metadata"
	"k8s.io/frakti/pkg/unikernel/metadata/store"
	"k8s.io/frakti/pkg/util/alternativeruntime"
	"k8s.io/frakti/pkg/util/indexset"
	"k8s.io/frakti/pkg/util/registrar"
)

const (
	// TODO(Crazykev): make this configurable
	defaultLibvirtdEndpoint string = "qemu:///system"
)

type UnikernelRuntime struct {
	// rootDir is the directory for managing unikernel runtime files
	rootDir string
	// sandboxStore stores all sandbox metadata.
	sandboxStore metadata.SandboxStore
	// containerStore stores all container metadata.
	containerStore metadata.ContainerStore
	// sandboxNameIndex stores all unique sandbox names.
	sandboxNameIndex *registrar.Registrar
	// sandboxIDIndex stores all unique sandbox IDs.
	sandboxIDIndex *indexset.IndexSet
	// containerNameIndex stores all unique container names.
	containerNameIndex *registrar.Registrar
	// containerIDIndex stores all unique container IDs.
	containerIDIndex *truncindex.TruncIndex
	// defaultCPU is the default cpu num of vm.
	defaultCPU int32
	// defaultMem is the default memory num of vm in MB.
	defaultMem int32
	// vmTool is the tools set to manipulate VM related operation.
	vmTool *libvirt.VMTool
	// imageManager manage all images in unikernel runtime.
	imageManager *image.ImageManager
	// enableLog determines whether vm's output print to file or console
	enableLog bool
}

func (u *UnikernelRuntime) ServiceName() string {
	return alternativeruntime.UnikernelRuntimeName
}

func NewUnikernelRuntimeService(cniNetDir, cniPluginDir, fraktiRoot string, defaultCPU, defaultMem int32, enableLog bool) (*UnikernelRuntime, error) {
	glog.Infof("Initialize unikernel runtime\n")

	// Init VMTools
	conn, err := libvirt.NewLibvirtConnect(defaultLibvirtdEndpoint)
	if err != nil {
		return nil, err
	}

	// TODO(Crazykev): Refactor CNI related code to a common lib.
	// TODO(Crazykev): Init CNI plugin.

	// TODO(Crazykev): Init checkpoint handler.

	// TODO(Crazykev): Init and start streaming server.

	runtime := &UnikernelRuntime{
		rootDir:            filepath.Join(fraktiRoot, "unikernel"),
		sandboxStore:       metadata.NewSandboxStore(store.NewMetadataStore()),
		containerStore:     metadata.NewContainerStore(store.NewMetadataStore()),
		sandboxNameIndex:   registrar.NewRegistrar(),
		sandboxIDIndex:     indexset.NewIndexSet(),
		containerNameIndex: registrar.NewRegistrar(),
		containerIDIndex:   truncindex.NewTruncIndex(nil),
		defaultCPU:         defaultCPU,
		defaultMem:         defaultMem,
		vmTool:             libvirt.NewVMTool(conn, enableLog),
		enableLog:          enableLog,
	}

	// Init image manager
	imageManager, err := image.NewImageManager("http", runtime.rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create image manager: %v", err)
	}
	runtime.imageManager = imageManager

	// Init root dir and image dir
	if err = os.MkdirAll(filepath.Join(runtime.rootDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create root dir: %v", err)
	}

	return runtime, nil
}

// Version returns the runtime name, runtime version and runtime API version
func (u *UnikernelRuntime) Version(kubeApiVersion string) (*kubeapi.VersionResponse, error) {
	return &kubeapi.VersionResponse{}, nil
}

// Status returns the status of the runtime.
func (h *UnikernelRuntime) Status() (*kubeapi.RuntimeStatus, error) {

	// TODO(Crazykev): Support CNI plugin and implement this.
	return &kubeapi.RuntimeStatus{}, nil
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (h *UnikernelRuntime) UpdateRuntimeConfig(runtimeConfig *kubeapi.RuntimeConfig) error {
	return nil
}
