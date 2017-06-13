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

package ocicni

import (
	cnitypes "github.com/containernetworking/cni/pkg/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	DefaultInterfaceName = "eth0"
	CNIPluginName        = "cni"
	DefaultNetDir        = "/etc/cni/net.d"
	DefaultCNIDir        = "/opt/cni/bin"
	VendorCNIDirTemplate = "%s/opt/%s/bin"
)

type CNIPlugin interface {
	// Name returns the plugin's name. This will be used when searching
	// for a plugin by name, e.g.
	Name() string

	// SetUpPod is the method called when the pod is created
	SetUpPod(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, annotations map[string]string, capabilities map[string]interface{}) (cnitypes.Result, error)

	// TearDownPod is the method called before pod stopped
	TearDownPod(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, annotations map[string]string, capabilities map[string]interface{}) error

	// NetworkStatus returns error if the network plugin is in error state
	Status() error
}
