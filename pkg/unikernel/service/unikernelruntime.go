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
	"github.com/golang/glog"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	"k8s.io/frakti/pkg/unikernel/metadata"
	"k8s.io/frakti/pkg/util/alternativeruntime"
	"k8s.io/frakti/pkg/util/registrar"
)

type UnikernelRuntime struct {
	// rootDir is the directory for managing unikernel runtime files
	rootDir string
	// sandboxStore stores all sandbox metadata.
	sandboxStore metadata.SandboxStore
	// sandboxNameIndex stores all unique sandbox names.
	sandboxNameIndex *registrar.Registrar
	// sandboxIDIndex stores all unique sandbox names.
	sandboxIDIndex *registrar.Registrar
}

func (u *UnikernelRuntime) ServiceName() string {
	return alternativeruntime.UnikernelRuntimeName
}

func NewUnikernelRuntimeService(cniNetDir, cniPluginDir string) (*UnikernelRuntime, error) {
	glog.Infof("Initialize unikernel runtime\n")

	// TODO(Crazykev): Init CNI plugin.

	// TODO(Crazykev): Init checkpoint handler.

	// TODO(Crazykev): Init and start streaming server.

	return &UnikernelRuntime{}, nil
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
