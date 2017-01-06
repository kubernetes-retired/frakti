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

	// SetUpPod is the method called after the infra container of
	// the pod has been created but before the other containers of the
	// pod are launched.
	SetUpPod(netnsPath string, namespace string, name string, containerID string) error

	// TearDownPod is the method called before a pod's infra container will be deleted
	TearDownPod(netnsPath string, namespace string, name string, containerID string) error

	// NetworkStatus returns error if the network plugin is in error state
	Status() error
}
