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
	"time"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ExecSync runs a command in a container synchronously.
func (u *UnikernelRuntime) ExecSync(rawContainerID string, cmd []string, timeout time.Duration) (stdout, stderr []byte, err error) {
	return nil, nil, fmt.Errorf("not implemented")
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (u *UnikernelRuntime) Exec(req *kubeapi.ExecRequest) (*kubeapi.ExecResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (u *UnikernelRuntime) Attach(req *kubeapi.AttachRequest) (*kubeapi.AttachResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (u *UnikernelRuntime) PortForward(req *kubeapi.PortForwardRequest) (*kubeapi.PortForwardResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
